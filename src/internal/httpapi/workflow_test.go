package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"sicendikia/internal/auth"
	"sicendikia/internal/esign"
	"sicendikia/internal/files"
	"sicendikia/internal/pdf"
	"sicendikia/internal/store"
)

func TestAlurLengkapPengajuanSampaiTerbit(t *testing.T) {
	// Skip pada mesin tanpa LibreOffice/soffice (mis. Windows dev) — tidak ada binary PDF.
	if bin := pdf.NewRenderer().Binary; bin == "" {
		t.Skip("LibreOffice/soffice tidak tersedia — skip e2e PDF sign (opsional lokal)")
	}
	fx := newFixture(t)
	fx.srv.Close()
	ctx := context.Background()
	var unitID int64
	if err := fx.pool.QueryRow(ctx, `SELECT id FROM units WHERE code='KORWIL-01'`).Scan(&unitID); err != nil {
		t.Fatal(err)
	}
	hash := func(password string) string {
		value, err := auth.HashPassword(password)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	for _, user := range []struct {
		username  string
		role      string
		name      string
		unit      *int64
		nik       string
		signature string
	}{
		{username: "verifikator-unit", role: "verifikator_unit", name: "Verifikator Unit", unit: &unitID},
		{username: "verifikator-dinas", role: "verifikator_dinas", name: "Verifikator Dinas"},
		{username: "pimpinan-utama", role: "pimpinan", name: "Kepala Dinas", nik: "1234567890123456", signature: "c2lnbmF0dXJl"},
	} {
		_, err := fx.pool.Exec(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id,nik,signature_image_base64,employee_number,job_title) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),'196711271995121002','Pembina Utama Muda')`, user.username, hash("sandi"), user.role, user.name, user.unit, user.nik, user.signature)
		if err != nil {
			t.Fatal(err)
		}
	}

	fakeSign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v2/sign/pdf" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("baca request eSign: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var payload struct {
				NIK        string   `json:"nik"`
				Passphrase string   `json:"passphrase"`
				Files      []string `json:"file"`
				Properties []any    `json:"signatureProperties"`
			}
			if err := json.Unmarshal(body, &payload); err != nil || payload.NIK != "1234567890123456" || payload.Passphrase != "passphrase-uji" || len(payload.Files) != 1 || len(payload.Properties) != 1 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			concept, err := base64.StdEncoding.DecodeString(payload.Files[0])
			if err != nil || !bytes.HasPrefix(concept, []byte("%PDF-")) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			// Mock signer mengembalikan kembali konsep SK yang diterima dengan
			// marker TTE dummy agar artifact final dapat diverifikasi.
			signed := append([]byte(nil), concept...)
			signed = append(signed, []byte("\n% DUMMY-SIGNED-BY-MOCK\n")...)
			if err := os.WriteFile("/tmp/si-cendikia-e2e-final-signed.pdf", signed, 0o600); err != nil {
				t.Errorf("simpan artifact dummy: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id_dokumen":"doc-uji-1"}`)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/api/sign/download/doc-uji-1" {
			content, err := os.ReadFile("/tmp/si-cendikia-e2e-final-signed.pdf")
			if err != nil || !bytes.HasPrefix(content, []byte("%PDF-")) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write(content)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer fakeSign.Close()
	fileStore, err := files.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	api := NewWithDependencies(fx.pool, "secret-test", false, Dependencies{
		Files:    fileStore,
		Renderer: pdf.NewRenderer(),
		Signer:   &esign.Client{BaseURL: fakeSign.URL, HTTP: fakeSign.Client()},
	})
	srv := httptest.NewServer(api.Routes())
	defer srv.Close()

	teacherClient, teacherCSRF := loginClient(t, srv.URL, nipPNS, nipPNS)
	createResp, createBody := submitPDF(t, teacherClient, srv.URL, teacherCSRF, "2026-09-01")
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("submit status=%d body=%v", createResp.StatusCode, createBody)
	}
	submissionID := int64(createBody["id"].(float64))

	unitClient, unitCSRF := loginClient(t, srv.URL, "verifikator-unit", "sandi")
	assertQueueHas(t, unitClient, srv.URL+"/api/v1/verifications/unit", submissionID)
	approve(t, unitClient, srv.URL+"/api/v1/verifications/unit/"+itoa(submissionID)+"/approve", unitCSRF)
	assertStatus(t, fx.pool, submissionID, "menunggu_dinas")

	dinasClient, dinasCSRF := loginClient(t, srv.URL, "verifikator-dinas", "sandi")
	assertQueueHas(t, dinasClient, srv.URL+"/api/v1/verifications/dinas", submissionID)
	approve(t, dinasClient, srv.URL+"/api/v1/verifications/dinas/"+itoa(submissionID)+"/approve", dinasCSRF)
	assertStatus(t, fx.pool, submissionID, "menunggu_tte")
	leaderClient, leaderCSRF := loginClient(t, srv.URL, "pimpinan-utama", "sandi")
	assertQueueHas(t, leaderClient, srv.URL+"/api/v1/letters/pending-tte", submissionID)
	signResp := postJSON(t, leaderClient, srv.URL+"/api/v1/letters/"+itoa(submissionID)+"/sign", leaderCSRF, `{"passphrase":"passphrase-uji"}`)
	if signResp.StatusCode != http.StatusCreated {
		t.Fatalf("sign status=%d body=%v", signResp.StatusCode, decodeBody(t, signResp))
	}
	assertStatus(t, fx.pool, submissionID, "terbit")
	letter, _, err := store.GetLetterForSubmission(ctx, fx.pool, submissionID)
	if err != nil {
		t.Fatal(err)
	}
	download, err := teacherClient.Get(srv.URL + "/api/v1/letters/" + itoa(letter.ID) + "/download")
	if err != nil {
		t.Fatal(err)
	}
	defer download.Body.Close()
	if download.StatusCode != http.StatusOK {
		t.Fatalf("download status=%d body=%v", download.StatusCode, decodeBody(t, download))
	}
	content, err := io.ReadAll(download.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		t.Fatalf("hasil unduh bukan PDF: %q", content[:min(len(content), 20)])
	}
	if !bytes.Contains(content, []byte("DUMMY-SIGNED-BY-MOCK")) {
		t.Fatal("PDF final tidak memuat marker tanda tangan dummy")
	}
	if info, err := os.Stat("/tmp/si-cendikia-e2e-final-signed.pdf"); err != nil || info.Size() == 0 {
		t.Fatalf("artifact PDF dummy tidak valid: info=%v err=%v", info, err)
	}
	if letter.TTEReceiptID != "doc-uji-1" {
		t.Fatalf("receipt surat = %q, want doc-uji-1", letter.TTEReceiptID)
	}
	audit, err := store.AuditForSubmission(ctx, fx.pool, submissionID)
	if err != nil {
		t.Fatal(err)
	}
	wantActions := []string{"submit", "setuju_unit", "setuju_dinas", "tte", "terbit"}
	seen := make(map[string]bool, len(audit))
	for _, item := range audit {
		seen[item.Action] = true
	}
	for _, action := range wantActions {
		if !seen[action] {
			t.Errorf("audit action %q tidak ditemukan; audit=%v", action, seen)
		}
	}
	if count, err := store.CountAudit(ctx, fx.pool, "terbit"); err != nil || count != 1 {
		t.Fatalf("audit terbit count=%d err=%v", count, err)
	}
}

func loginClient(t *testing.T, base, username, password string) (*http.Client, string) {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp, err := client.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var envelope struct {
		Data struct {
			CSRF string `json:"csrf"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || envelope.Data.CSRF == "" {
		t.Fatalf("login %s status=%d csrf=%q", username, resp.StatusCode, envelope.Data.CSRF)
	}
	return client, envelope.Data.CSRF
}

func submitPDF(t *testing.T, client *http.Client, base, csrf, date string) (*http.Response, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("proposed_tmt", date)
	_ = writer.WriteField("birth_place", "Grobogan")
	_ = writer.WriteField("birth_date", "1980-01-01")
	_ = writer.WriteField("karpeg", "I 123456")
	_ = writer.WriteField("pangkat", "Penata Muda Tingkat I")
	_ = writer.WriteField("jabatan", "Guru Ahli Pertama")
	_ = writer.WriteField("last_sk_pejabat", "Kepala Dinas Pendidikan")
	_ = writer.WriteField("last_sk_tanggal", "2024-04-01")
	_ = writer.WriteField("last_sk_nomor", "800/010/4.2/2024")
	_ = writer.WriteField("last_sk_tmt", "2024-04-01")
	_ = writer.WriteField("last_kgb_golongan", "III/b")
	_ = writer.WriteField("last_kgb_masa_tahun", "10")
	_ = writer.WriteField("last_kgb_masa_bulan", "0")
	_ = writer.WriteField("last_kp_golongan", "III/b")
	_ = writer.WriteField("last_kp_tmt", "2024-04-01")
	_ = writer.WriteField("last_kp_masa_tahun", "10")
	_ = writer.WriteField("last_kp_masa_bulan", "0")
	_ = writer.WriteField("last_kp_nomor", "800.1.3.2/795/2024")
	_ = writer.WriteField("last_kp_tanggal", "2024-03-20")
	_ = writer.WriteField("last_kp_pejabat", "BUPATI GROBOGAN")
	_ = writer.WriteField("mkg_lama_tahun", "10")
	_ = writer.WriteField("mkg_lama_bulan", "0")
	_ = writer.WriteField("mkg_baru_tahun", "12")
	_ = writer.WriteField("mkg_baru_bulan", "0")
	part, err := writer.CreateFormFile("file", "dukungan.pdf")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("%PDF-1.4\n% test\n"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/submissions", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Data  map[string]any `json:"data"`
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode >= 400 {
		return resp, envelope.Error
	}
	return resp, envelope.Data
}

func approve(t *testing.T, client *http.Client, url, csrf string) {
	t.Helper()
	resp := postJSON(t, client, url, csrf, `{}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve status=%d body=%v", resp.StatusCode, decodeBody(t, resp))
	}
}

func assertQueueHas(t *testing.T, client *http.Client, url string, id int64) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	for _, item := range envelope.Data {
		if int64(item["id"].(float64)) == id {
			return
		}
	}
	t.Fatalf("antrean %s tidak memuat pengajuan %d", url, id)
}

func assertStatus(t *testing.T, pool *pgxpool.Pool, id int64, want string) {
	t.Helper()
	var got string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM submissions WHERE id=$1`, id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("status pengajuan %d = %s, ingin %s", id, got, want)
	}
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return body
}

func postJSON(t *testing.T, client *http.Client, url, csrf, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func itoa(v int64) string { return fmt.Sprintf("%d", v) }

func TestSubmitTanpaKarpegDitolak(t *testing.T) {
	fx := newFixture(t)
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("proposed_tmt", "2026-09-01")
	_ = writer.WriteField("birth_place", "Grobogan")
	_ = writer.WriteField("birth_date", "1980-01-01")
	_ = writer.WriteField("pangkat", "Penata")
	_ = writer.WriteField("jabatan", "Guru")
	_ = writer.WriteField("last_sk_pejabat", "Kepala Dinas")
	_ = writer.WriteField("last_sk_tanggal", "2024-04-01")
	_ = writer.WriteField("last_sk_nomor", "800/010/4.2/2024")
	_ = writer.WriteField("last_sk_tmt", "2024-04-01")
	part, err := writer.CreateFormFile("file", "dukungan.pdf")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("%PDF-1.4\n% test\n"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, fx.srv.URL+"/api/v1/submissions", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d, ingin 422", resp.StatusCode)
	}
}

func TestSubmitMenghitungTMTGenapDariSKPertama(t *testing.T) {
	fx := newFixture(t)
	ctx := context.Background()
	if _, err := fx.pool.Exec(ctx, `UPDATE teachers SET tmt_kgb_last='2020-12-01', last_sk_tmt_berlaku='2020-12-01', masa_kerja_source='tmt_cpns', tmt_awal='2020-12-01' WHERE nip=$1`, nipPNS); err != nil {
		t.Fatal(err)
	}
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("tmt_awal", "2020-12-01")
	_ = writer.WriteField("birth_place", "Grobogan")
	_ = writer.WriteField("birth_date", "1980-01-01")
	_ = writer.WriteField("karpeg", "I 123456")
	_ = writer.WriteField("pangkat", "Penata Muda Tingkat I")
	_ = writer.WriteField("jabatan", "Guru Ahli Pertama")
	_ = writer.WriteField("last_sk_pejabat", "Kepala Dinas Pendidikan")
	_ = writer.WriteField("last_sk_tanggal", "2020-12-01")
	_ = writer.WriteField("last_sk_nomor", "800/001/4.2/2020")
	_ = writer.WriteField("last_sk_tmt", "2020-12-01")
	_ = writer.WriteField("last_kgb_golongan", "III/b")
	_ = writer.WriteField("last_kgb_masa_tahun", "4")
	_ = writer.WriteField("last_kgb_masa_bulan", "0")
	_ = writer.WriteField("last_kp_golongan", "III/b")
	_ = writer.WriteField("last_kp_tmt", "2020-12-01")
	_ = writer.WriteField("last_kp_masa_tahun", "4")
	_ = writer.WriteField("last_kp_masa_bulan", "0")
	_ = writer.WriteField("last_kp_nomor", "800.1.3.2/795/2020")
	_ = writer.WriteField("last_kp_tanggal", "2020-11-20")
	_ = writer.WriteField("last_kp_pejabat", "BUPATI GROBOGAN")
	part, err := writer.CreateFormFile("file", "dukungan.pdf")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("%PDF-1.4\n% test\n"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, fx.srv.URL+"/api/v1/submissions", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Data  map[string]any `json:"data"`
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit status=%d body=%v", resp.StatusCode, envelope.Error)
	}
	got := envelope.Data["proposed_tmt"]
	if !strings.HasPrefix(fmt.Sprint(got), "2026-12-01") {
		t.Fatalf("proposed_tmt = %v, ingin 2026-12-01", got)
	}
	if fmt.Sprint(envelope.Data["draft_mkg_lama_tahun"]) != "4" || fmt.Sprint(envelope.Data["draft_mkg_baru_tahun"]) != "6" {
		t.Fatalf("mkg draft = lama %v baru %v, ingin 4 → 6", envelope.Data["draft_mkg_lama_tahun"], envelope.Data["draft_mkg_baru_tahun"])
	}
	if fmt.Sprint(envelope.Data["draft_mkg_lama_bulan"]) != "0" || fmt.Sprint(envelope.Data["draft_mkg_baru_bulan"]) != "0" {
		t.Fatalf("bulan mkg harus 0: %v / %v", envelope.Data["draft_mkg_lama_bulan"], envelope.Data["draft_mkg_baru_bulan"])
	}
}

func TestSKNaikPangkatTidakMenggeserTMTKGB(t *testing.T) {
	fx := newFixture(t)
	ctx := context.Background()
	if _, err := fx.pool.Exec(ctx, `UPDATE teachers SET tmt_kgb_last='2020-12-01', last_sk_tmt_berlaku='2026-07-01', masa_kerja_source='tmt_cpns', tmt_awal='2020-12-01' WHERE nip=$1`, nipPNS); err != nil {
		t.Fatal(err)
	}
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("tmt_awal", "2020-12-01")
	_ = writer.WriteField("birth_place", "Grobogan")
	_ = writer.WriteField("birth_date", "1980-01-01")
	_ = writer.WriteField("karpeg", "I 123456")
	_ = writer.WriteField("pangkat", "Penata")
	_ = writer.WriteField("jabatan", "Guru Ahli Pertama")
	_ = writer.WriteField("last_sk_pejabat", "Kepala Dinas Pendidikan")
	_ = writer.WriteField("last_sk_tanggal", "2026-07-01")
	_ = writer.WriteField("last_sk_nomor", "800/099/4.2/2026")
	_ = writer.WriteField("last_sk_tmt", "2026-07-01")
	_ = writer.WriteField("last_kgb_golongan", "III/b")
	_ = writer.WriteField("last_kgb_masa_tahun", "6")
	_ = writer.WriteField("last_kgb_masa_bulan", "0")
	_ = writer.WriteField("last_kp_golongan", "III/b")
	_ = writer.WriteField("last_kp_tmt", "2026-07-01")
	_ = writer.WriteField("last_kp_masa_tahun", "6")
	_ = writer.WriteField("last_kp_masa_bulan", "0")
	_ = writer.WriteField("last_kp_nomor", "800.1.3.2/795/2026")
	_ = writer.WriteField("last_kp_tanggal", "2026-06-20")
	_ = writer.WriteField("last_kp_pejabat", "BUPATI GROBOGAN")
	part, err := writer.CreateFormFile("file", "dukungan.pdf")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("%PDF-1.4\n% test\n"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, fx.srv.URL+"/api/v1/submissions", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Data  map[string]any `json:"data"`
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit status=%d body=%v", resp.StatusCode, envelope.Error)
	}
	if !strings.HasPrefix(fmt.Sprint(envelope.Data["proposed_tmt"]), "2026-12-01") {
		t.Fatalf("proposed_tmt SK naik pangkat Jul 2026 seharusnya tetap 2026-12-01 (jangkar tmt_awal), dapat %v", envelope.Data["proposed_tmt"])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
