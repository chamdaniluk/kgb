package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

// Multi-berkas (017): PNS wajib file_kp + file_kgb; unduh per slot dengan
// header frame SAMEORIGIN agar preview iframe tampil di Firefox.
func TestSubmitMultiBerkasPNSDanUnduhPerSlot(t *testing.T) {
	fx := newFixture(t)
	ctx := context.Background()
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"birth_place": "Grobogan", "birth_date": "1980-01-01", "karpeg": "I 123456",
		"jabatan": "Guru Ahli Pertama", "last_sk_pejabat": "BUPATI GROBOGAN",
		"last_sk_tanggal": "2024-11-07", "last_sk_nomor": "800/010/4.2/2024",
		"last_sk_tmt": "2024-12-01", "tmt_awal": "2020-12-01", "pangkat_gol": "III/b",
		"last_kgb_golongan": "III/b", "last_kgb_masa_tahun": "4", "last_kgb_masa_bulan": "0",
		"last_kp_golongan": "III/b", "last_kp_tmt": "2024-12-01",
		"last_kp_masa_tahun": "4", "last_kp_masa_bulan": "0",
		"last_kp_nomor": "800.1.3.2/795/2024", "last_kp_tanggal": "2024-11-07",
		"last_kp_pejabat": "BUPATI GROBOGAN",
	}
	for k, v := range fields {
		_ = writer.WriteField(k, v)
	}
	for _, slot := range []struct{ field, name, content string }{
		{"file", "utama.pdf", "%PDF-1.4 utama"},
		{"file_kp", "sk-kp.pdf", "%PDF-1.4 kp"},
		{"file_kgb", "sk-kgb.pdf", "%PDF-1.4 kgb"},
	} {
		p, err := writer.CreateFormFile(slot.field, slot.name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = p.Write([]byte(slot.content))
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
		t.Fatalf("submit multi-berkas: status=%d body=%v", resp.StatusCode, env.Error)
	}
	id := int64(env.Data["id"].(float64))
	if env.Data["file_kp_name"] != "sk-kp.pdf" || env.Data["file_kgb_name"] != "sk-kgb.pdf" {
		t.Fatalf("nama slot tidak tersimpan: %v", env.Data)
	}

	// Unduh tiap slot: isi beda + header frame SAMEORIGIN.
	for _, tc := range []struct {
		slot, want string
	}{
		{"", "%PDF-1.4 utama"},
		{"kp", "%PDF-1.4 kp"},
		{"kgb", "%PDF-1.4 kgb"},
	} {
		url := fx.srv.URL + "/api/v1/submissions/" + itoa(id) + "/file"
		if tc.slot != "" {
			url += "?slot=" + tc.slot
		}
		r2, err := client.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		content, _ := io.ReadAll(r2.Body)
		r2.Body.Close()
		if r2.StatusCode != http.StatusOK || string(content) != tc.want {
			t.Fatalf("slot %q: status=%d isi=%q", tc.slot, r2.StatusCode, content)
		}
		if got := r2.Header.Get("X-Frame-Options"); got != "SAMEORIGIN" {
			t.Fatalf("slot %q: X-Frame-Options=%q, ingin SAMEORIGIN", tc.slot, got)
		}
		if got := r2.Header.Get("Content-Type"); got != "application/pdf" {
			t.Fatalf("slot %q: Content-Type=%q", tc.slot, got)
		}
	}
	// Slot tak dikenal ditolak.
	r3, err := client.Get(fx.srv.URL + "/api/v1/submissions/" + itoa(id) + "/file?slot=ngawur")
	if err != nil {
		t.Fatal(err)
	}
	r3.Body.Close()
	if r3.StatusCode != http.StatusBadRequest {
		t.Fatalf("slot ngawur: status=%d, ingin 400", r3.StatusCode)
	}
	_ = ctx
}
