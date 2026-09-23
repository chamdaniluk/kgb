package httpapi

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sicendikia/internal/auth"
	"sicendikia/internal/files"
	"sicendikia/internal/store"
)

// TestAlurTTEManualTerbit memastikan pimpinan dapat menerbitkan surat dari
// unggahan PDF TTE manual: nomor dicadangkan, status terbit, master guru
// terbarui lewat jalur CommitIssue yang sama, dan audit mencatat tte_manual.
func TestAlurTTEManualTerbit(t *testing.T) {
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
	if _, err := fx.pool.Exec(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id) VALUES ($1,$2,'pimpinan','Kepala Dinas',$3)`,
		"pimpinan-manual", hash("sandi"), unitID); err != nil {
		t.Fatal(err)
	}

	fileStore, err := files.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	api := NewWithDependencies(fx.pool, "secret-test", false, Dependencies{Files: fileStore})
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
	dinasClient, dinasCSRF := loginClient(t, srv.URL, "verifikator-dinas", "sandi")
	assertQueueHas(t, dinasClient, srv.URL+"/api/v1/verifications/dinas", submissionID)
	approve(t, dinasClient, srv.URL+"/api/v1/verifications/dinas/"+itoa(submissionID)+"/approve", dinasCSRF)
	assertStatus(t, fx.pool, submissionID, "menunggu_tte")

	leaderClient, leaderCSRF := loginClient(t, srv.URL, "pimpinan-manual", "sandi")

	// Tanpa catatan → ditolak.
	var bodyNoNote bytes.Buffer
	wNoNote := multipart.NewWriter(&bodyNoNote)
	pNoNote, _ := wNoNote.CreateFormFile("file", "surat-tte-manual.pdf")
	_, _ = pNoNote.Write([]byte("%PDF-1.4\n% tte manual\n"))
	_ = wNoNote.Close()
	respNoNote := postMultipart(t, leaderClient, srv.URL+"/api/v1/letters/"+itoa(submissionID)+"/manual", leaderCSRF, &bodyNoNote, wNoNote.FormDataContentType())
	defer respNoNote.Body.Close()
	if respNoNote.StatusCode != http.StatusBadRequest {
		t.Fatalf("tanpa catatan harus 400, dapat %d", respNoNote.StatusCode)
	}

	// Berkas bukan PDF → ditolak.
	var bodyNotPDF bytes.Buffer
	wNotPDF := multipart.NewWriter(&bodyNotPDF)
	_ = wNotPDF.WriteField("note", "TTE elektronik gangguan, ditandatangani basah")
	pNotPDF, _ := wNotPDF.CreateFormFile("file", "surat.txt")
	_, _ = pNotPDF.Write([]byte("bukan pdf"))
	_ = wNotPDF.Close()
	respNotPDF := postMultipart(t, leaderClient, srv.URL+"/api/v1/letters/"+itoa(submissionID)+"/manual", leaderCSRF, &bodyNotPDF, wNotPDF.FormDataContentType())
	defer respNotPDF.Body.Close()
	if respNotPDF.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("berkas bukan PDF harus 422, dapat %d", respNotPDF.StatusCode)
	}

	// Unggahan valid → terbit.
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("note", "TTE elektronik gangguan, ditandatangani basah")
	p, _ := w.CreateFormFile("file", "surat-tte-manual.pdf")
	_, _ = p.Write([]byte("%PDF-1.4\n% tte manual\n"))
	_ = w.Close()
	resp := postMultipart(t, leaderClient, srv.URL+"/api/v1/letters/"+itoa(submissionID)+"/manual", leaderCSRF, &body, w.FormDataContentType())
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unggah TTE manual status=%d body=%v", resp.StatusCode, decodeBody(t, resp))
	}
	assertStatus(t, fx.pool, submissionID, "terbit")

	letter, path, err := store.GetLetterForSubmission(ctx, fx.pool, submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(letter.Number, "/2026") {
		t.Fatalf("nomor surat tidak wajar: %q", letter.Number)
	}
	if letter.TTEReceiptID != "" {
		t.Fatalf("TTE manual tidak boleh punya receipt eSign, dapat %q", letter.TTEReceiptID)
	}
	f, err := fileStore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(f); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	_ = f.Close()
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatal("surat tersimpan bukan PDF")
	}

	audit, err := store.AuditForSubmission(ctx, fx.pool, submissionID)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range audit {
		seen[item.Action] = true
	}
	for _, action := range []string{"tte_manual", "terbit"} {
		if !seen[action] {
			t.Errorf("audit action %q tidak ditemukan; audit=%v", action, seen)
		}
	}

	// Guru dapat mengunduh surat hasil TTE manual.
	download, err := teacherClient.Get(srv.URL + "/api/v1/letters/" + itoa(letter.ID) + "/download")
	if err != nil {
		t.Fatal(err)
	}
	defer download.Body.Close()
	if download.StatusCode != http.StatusOK {
		t.Fatalf("unduh surat status=%d", download.StatusCode)
	}
}

// postMultipart mengirim multipart yang sudah tersusun dengan header CSRF.
func postMultipart(t *testing.T, client *http.Client, url, csrf string, body *bytes.Buffer, contentType string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
