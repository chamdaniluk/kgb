package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"
)

// submitKGBPNSJabatan mengirim form pengajuan PNS dengan jabatan bebas yang
// ditentukan test, supaya jalur "isian manual" dapat diuji sampai handler.
func submitKGBPNSJabatan(t *testing.T, jabatan string) (*http.Response, map[string]any) {
	t.Helper()
	fx := newFixture(t)
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"tmt_awal":            "2016-04-01",
		"birth_place":         "Grobogan",
		"birth_date":          "1980-01-01",
		"karpeg":              "I 123456",
		"pangkat":             "Penata Muda Tingkat I",
		"jabatan":             jabatan,
		"last_sk_pejabat":     "Kepala Dinas Pendidikan",
		"last_sk_tanggal":     "2024-04-01",
		"last_sk_nomor":       "800/001/4.2/2024",
		"last_sk_tmt":         "2024-04-01",
		"last_kgb_golongan":   "III/b",
		"last_kgb_masa_tahun": "4",
		"last_kgb_masa_bulan": "0",
		"last_kp_golongan":    "III/b",
		"last_kp_tmt":         "2024-04-01",
		"last_kp_masa_tahun":  "4",
		"last_kp_masa_bulan":  "0",
		"last_kp_nomor":       "800.1.3.2/795/2024",
		"last_kp_tanggal":     "2024-04-01",
		"last_kp_pejabat":     "BUPATI GROBOGAN",
	}
	for k, v := range fields {
		_ = writer.WriteField(k, v)
	}
	for _, slot := range []struct{ field, name string }{{"file_kp", "sk-kp.pdf"}, {"file_kgb", "sk-kgb.pdf"}} {
		p, err := writer.CreateFormFile(slot.field, slot.name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = p.Write([]byte("%PDF-1.4\n% test\n"))
	}
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
	var envelope struct {
		Data  map[string]any `json:"data"`
		Error map[string]any `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return resp, envelope.Data
}

// TestSubmitJabatanIsianManualDiterima mengunci inti permintaan owner: jabatan
// di luar daftar guru (mis. pengawas/penilik) dapat diisi manual dan tersimpan
// apa adanya di naskah, karena Dinas tidak hanya berisi guru.
func TestSubmitJabatanIsianManualDiterima(t *testing.T) {
	for _, jabatan := range []string{
		"Pengawas Sekolah Ahli Utama",
		"Penilik Terampil",
		"Pamong Belajar Ahli Madya",
		"Pengelola Sistem Informasi Kepegawaian",
	} {
		t.Run(jabatan, func(t *testing.T) {
			resp, data := submitKGBPNSJabatan(t, jabatan)
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("status=%d data=%v, ingin 201", resp.StatusCode, data)
			}
			if got := fmt.Sprint(data["draft_jabatan"]); got != jabatan {
				t.Fatalf("draft_jabatan = %q, ingin %q", got, jabatan)
			}
		})
	}
}

// TestSubmitJabatanNyasarDitolak memastikan kebebasan isian tidak berarti
// menerima apa saja: teks dengan markup/tanda tak wajar tetap 422.
func TestSubmitJabatanNyasarDitolak(t *testing.T) {
	for _, jabatan := range []string{
		"Guru <script>",
		"Guru;DROP TABLE",
		"ab",
	} {
		t.Run(jabatan, func(t *testing.T) {
			resp, data := submitKGBPNSJabatan(t, jabatan)
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d data=%v, ingin 422", resp.StatusCode, data)
			}
		})
	}
}
