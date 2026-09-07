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

// submitPDFWith mengirim form submit dengan field tambahan/override.
func submitPDFWith(t *testing.T, client *http.Client, base, csrf string, extra map[string]string) (*http.Response, map[string]any) {
	t.Helper()
	fields := map[string]string{
		"birth_place": "Grobogan", "birth_date": "1980-01-01", "karpeg": "I 123456",
		"jabatan": "Guru Ahli Pertama", "last_sk_pejabat": "BUPATI GROBOGAN",
		"last_sk_tanggal": "2024-11-07", "last_sk_nomor": "800/010/4.2/2024",
		"last_sk_tmt": "2024-12-01", "tmt_awal": "2020-12-01", "pangkat_gol": "III/a",
		"last_kgb_golongan": "III/a", "last_kgb_masa_tahun": "4", "last_kgb_masa_bulan": "0",
		"last_kp_golongan": "III/a", "last_kp_tmt": "2024-12-01",
		"last_kp_masa_tahun": "4", "last_kp_masa_bulan": "0",
		"last_kp_nomor": "800.1.3.2/795/2024", "last_kp_tanggal": "2024-11-07",
		"last_kp_pejabat": "BUPATI GROBOGAN",
	}
	for k, v := range extra {
		fields[k] = v
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for k, v := range fields {
		_ = writer.WriteField(k, v)
	}
	part, err := writer.CreateFormFile("file", "dukungan.pdf")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("%PDF-1.4 test"))
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

// Contoh owner: TMT CPNS Des 2020, KGB Terakhir Des 2024,
// SIPPASN sudah KP III/b, KP III/b TMT 2025-07. Usul KGB Des 2026:
// gaji pakai III/b MKG 6, jangka waktu tetap dari KGB terakhir.
// Gol KGB (III/a) berbeda dari KP karena perubahan di tengah masa KGB.
func TestSubmitDenganKPBaruMemakaiGolKP(t *testing.T) {
	fx := newFixture(t)
	ctx := context.Background()
	if _, err := fx.pool.Exec(ctx, `UPDATE teachers SET tmt_awal='2020-12-01', tmt_kgb_last='2024-12-01', last_sk_tmt_berlaku='2024-12-01', pangkat_gol='III/b', pangkat='Penata Muda Tingkat I', masa_kerja_tahun=4, masa_kerja_source='tmt_cpns' WHERE nip=$1`, nipPNS); err != nil {
		t.Fatal(err)
	}
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	resp, data := submitPDFWith(t, client, fx.srv.URL, csrf, map[string]string{
		"last_kgb_golongan":  "III/a",
		"last_kp_golongan":   "III/b",
		"last_kp_tmt":        "2025-07-01",
		"last_kp_masa_tahun": "5",
		"last_kp_masa_bulan": "0",
		"last_kp_nomor":      "800.1.3.2/795/2025",
		"last_kp_tanggal":    "2025-06-20",
		"last_kp_pejabat":    "BUPATI GROBOGAN",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit dengan KP status=%d body=%v", resp.StatusCode, data)
	}
	// MKG 2020-12 -> 2026-12 = 6 th; KP tersimpan.
	if data["proposed_masa_kerja_tahun"] != float64(6) {
		t.Errorf("masa kerja = %v, want 6", data["proposed_masa_kerja_tahun"])
	}
	if data["draft_last_kp_golongan"] != "III/b" {
		t.Errorf("KP tersimpan = %v, want III/b", data["draft_last_kp_golongan"])
	}
	var gaji string
	if err := fx.pool.QueryRow(ctx, `SELECT next_salary::text FROM submissions ORDER BY id DESC LIMIT 1`).Scan(&gaji); err != nil {
		t.Fatal(err)
	}
	var expect string
	if err := fx.pool.QueryRow(ctx, `SELECT gaji::text FROM salary_scales WHERE asn_type='pns' AND golongan='III/b' AND masa_kerja_tahun=6`).Scan(&expect); err != nil {
		t.Fatal(err)
	}
	if gaji != expect {
		t.Errorf("gaji KGB = %s, want %s (III/b MKG 6)", gaji, expect)
	}
}

// Tanpa KP: seluruh seksi KP wajib → 422.
func TestSubmitTanpaKPDitolak(t *testing.T) {
	fx := newFixture(t)
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	noKP := map[string]string{
		"last_kp_golongan": "", "last_kp_tmt": "", "last_kp_masa_tahun": "",
		"last_kp_masa_bulan": "", "last_kp_nomor": "", "last_kp_tanggal": "",
		"last_kp_pejabat": "",
	}
	resp, body := submitPDFWith(t, client, fx.srv.URL, csrf, noKP)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("submit tanpa KP status=%d body=%v, want 422", resp.StatusCode, body)
	}
}

// KP sebagian (gol tanpa TMT): 422.
func TestSubmitKPSeparuhDitolak(t *testing.T) {
	fx := newFixture(t)
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	resp, body := submitPDFWith(t, client, fx.srv.URL, csrf, map[string]string{
		"last_kp_golongan": "III/b",
		"last_kp_tmt":      "",
		"last_kp_nomor":    "",
		"last_kp_tanggal":  "",
		"last_kp_pejabat":  "",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("KP tanpa TMT status=%d body=%v, want 422", resp.StatusCode, body)
	}
}

// KP PNS yang berbeda dari SIPPASN: 422 (gol KP dikunci).
func TestSubmitKPBedaSIPPASNDitolak(t *testing.T) {
	fx := newFixture(t)
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	resp, body := submitPDFWith(t, client, fx.srv.URL, csrf, map[string]string{
		"last_kgb_golongan": "III/a",
		"last_kp_golongan":  "III/c",
		"last_kp_tmt":       "2025-07-01",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("KP beda SIPPASN status=%d body=%v, want 422", resp.StatusCode, body)
	}
}

// Peninjauan masa kerja: CPNS Des 2020 + MKG SK KGB 8 (wiyata bakti 4 th)
// → KGB Des 2026 MKG 10 dengan golongan KP III/b.
func TestSubmitPeninjauanMasaKerja(t *testing.T) {
	fx := newFixture(t)
	ctx := context.Background()
	if _, err := fx.pool.Exec(ctx, `UPDATE teachers SET tmt_awal='2020-12-01', tmt_kgb_last='2024-12-01', last_sk_tmt_berlaku='2024-12-01', last_sk_masa_kerja_tahun=8, last_sk_masa_kerja_bulan=0, pangkat_gol='III/b', pangkat='Penata Muda Tingkat I', masa_kerja_tahun=8, masa_kerja_source='sk_peninjauan' WHERE nip=$1`, nipPNS); err != nil {
		t.Fatal(err)
	}
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	resp, data := submitPDFWith(t, client, fx.srv.URL, csrf, map[string]string{
		"last_kgb_golongan":  "III/a",
		"last_kgb_masa_tahun": "8",
		"last_kgb_masa_bulan": "0",
		"last_kp_golongan":   "III/b",
		"last_kp_tmt":        "2026-07-01",
		"last_kp_nomor":      "800.1.3.2/795/2026",
		"last_kp_tanggal":    "2026-06-20",
		"last_kp_pejabat":    "BUPATI GROBOGAN",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit peninjauan status=%d body=%v", resp.StatusCode, data)
	}
	if data["proposed_masa_kerja_tahun"] != float64(10) {
		t.Errorf("masa kerja = %v, want 10 (8+2)", data["proposed_masa_kerja_tahun"])
	}
	var gaji string
	if err := fx.pool.QueryRow(ctx, `SELECT next_salary::text FROM submissions ORDER BY id DESC LIMIT 1`).Scan(&gaji); err != nil {
		t.Fatal(err)
	}
	var expect string
	if err := fx.pool.QueryRow(ctx, `SELECT gaji::text FROM salary_scales WHERE asn_type='pns' AND golongan='III/b' AND masa_kerja_tahun=10`).Scan(&expect); err != nil {
		t.Fatal(err)
	}
	if gaji != expect {
		t.Errorf("gaji = %s, want %s (III/b MKG 10)", gaji, expect)
	}
}
