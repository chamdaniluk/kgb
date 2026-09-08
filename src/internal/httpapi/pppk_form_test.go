package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"sicendikia/internal/auth"
)

// Form PPPK berdiri sendiri (tanpa seksi KP): submit PPPK tanpa satu pun
// kolom last_kp_* harus sukses — SK Pertama/Perpanjangan Kontrak menggantikan
// KP sebagai SK terakhir.
func TestSubmitPPPKTanpaKPLolos(t *testing.T) {
	fx := newFixture(t)
	client, csrf := loginClient(t, fx.srv.URL, nipPPPK, nipPPPK)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"birth_place": "Grobogan", "birth_date": "1985-01-01", "karpeg": "-",
		"jabatan": "Guru Ahli Pertama", "last_sk_pejabat": "KEPALA DINAS PENDIDIKAN",
		"last_sk_tanggal": "2024-11-07", "last_sk_nomor": "800/421/4.2/2024",
		"last_sk_tmt": "2024-12-01", "tmt_awal": "2021-01-01", "pangkat_gol": "IX",
		"last_kgb_golongan": "IX", "last_kgb_masa_tahun": "4", "last_kgb_masa_bulan": "0",
		"masa_perjanjian_kerja": "5 tahun", "perpanjangan_perjanjian_kerja": "2026-12-31",
	}
	for k, v := range fields {
		_ = writer.WriteField(k, v)
	}
	for _, slot := range []struct{ field, name string }{
		{"file_kp", "sk-pertama.pdf"},
		{"file_skp", "skp.pdf"},
	} {
		p, err := writer.CreateFormFile(slot.field, slot.name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = p.Write([]byte("%PDF-1.4 test"))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, fx.srv.URL+"/api/v1/submissions", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env struct {
		Data  map[string]any `json:"data"`
		Error map[string]any `json:"error"`
	}
	_ = json.Unmarshal(raw, &env)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit PPPK tanpa KP: status=%d body=%v", resp.StatusCode, env.Error)
	}
	if v, ok := env.Data["draft_last_kp_golongan"]; ok && v != "" {
		t.Fatalf("golongan KP PPPK harus kosong, dapat %v", v)
	}
}

// Draft SK (Word) dapat diunduh petugas Dinas saat pengajuan menunggu TTE,
// dan tetap terlarang bagi pihak di luar kewenangan (guru lain pun terlarang
// via canAccessSubmission; di sini cukup memastikan jalur Dinas 200 + DOCX).
func TestDraftDOCXDapatDiaksesDinas(t *testing.T) {
	fx := newFixture(t)
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
		username, role, name string
		unit                 *int64
	}{
		{username: "verifikator-unit", role: "verifikator_unit", name: "Verifikator Unit", unit: &unitID},
		{username: "verifikator-dinas", role: "verifikator_dinas", name: "Verifikator Dinas"},
		{username: "pimpinan-utama", role: "pimpinan", name: "Kepala Dinas"},
	} {
		if _, err := fx.pool.Exec(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id,employee_number,job_title) VALUES ($1,$2,$3,$4,$5,'196711271995121002','Pembina Utama Muda')`, user.username, hash("sandi"), user.role, user.name, user.unit); err != nil {
			t.Fatal(err)
		}
	}
	teacherClient, teacherCSRF := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	resp, data := submitPDFWith(t, teacherClient, fx.srv.URL, teacherCSRF, map[string]string{
		// Golongan KP sama dengan fixture SIPPASN (III/b).
		"last_kp_golongan": "III/b",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit status=%d body=%v", resp.StatusCode, data)
	}
	subID := int64(data["id"].(float64))
	unitClient, unitCSRF := loginClient(t, fx.srv.URL, "verifikator-unit", "sandi")
	approve(t, unitClient, fx.srv.URL+"/api/v1/verifications/unit/"+itoa(subID)+"/approve", unitCSRF)
	dinasClient, dinasCSRF := loginClient(t, fx.srv.URL, "verifikator-dinas", "sandi")
	approve(t, dinasClient, fx.srv.URL+"/api/v1/verifications/dinas/"+itoa(subID)+"/approve", dinasCSRF)

	// Petugas Dinas mengunduh draft DOCX — harus 200 dengan berkas DOCX.
	dl, err := dinasClient.Get(fx.srv.URL + "/api/v1/letters/" + itoa(subID) + "/draft-docx")
	if err != nil {
		t.Fatal(err)
	}
	defer dl.Body.Close()
	content, _ := io.ReadAll(dl.Body)
	if dl.StatusCode != http.StatusOK {
		t.Fatalf("draft-docx dinas: status=%d body=%s", dl.StatusCode, content)
	}
	if !bytes.HasPrefix(content, []byte("PK")) {
		t.Fatalf("draft bukan DOCX (zip): %q", content[:min(len(content), 4)])
	}
	if got := dl.Header.Get("Content-Type"); got != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Fatalf("Content-Type=%q", got)
	}
}
