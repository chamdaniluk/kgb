package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"
)

// TestSalaryPreviewPoin8DariGridPoin6 mengunci perbaikan insiden #125 pada jalur
// pratinjau form: ketika SK KP lebih baru dan berbulan (5 th 7 bl), poin 6
// menampilkan mentah 05/07 sementara poin 8 memakai grid genap poin 6 + 2 th
// 0 bl (06/00). Sebelumnya pratinjau memakai masa SK pemenang + 2 (07/00 atau
// 07/07).
func TestSalaryPreviewPoin8DariGridPoin6(t *testing.T) {
	fx := newFixture(t)
	client, _ := loginClient(t, fx.srv.URL, nipPNS, nipPNS)

	url := fx.srv.URL + "/api/v1/salary-preview?tmt_awal=2016-04-01&last_sk_tmt=2024-04-01&golongan=III/b" +
		"&last_kgb_masa_tahun=4&last_kgb_masa_bulan=0" +
		"&last_kp_masa_tahun=5&last_kp_masa_bulan=7" +
		"&last_kp_tanggal=2026-06-20&last_sk_tanggal=2024-04-01"
	resp, err := client.Get(url)
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
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d err=%v", resp.StatusCode, envelope.Error)
	}
	get := func(k string) string { return fmt.Sprint(envelope.Data[k]) }
	if got := get("mkg_lama_tahun"); got != "5" {
		t.Fatalf("mkg_lama_tahun = %s, ingin 5 (mentah SK KP)", got)
	}
	if got := get("mkg_lama_bulan"); got != "7" {
		t.Fatalf("mkg_lama_bulan = %s, ingin 7 (mentah SK KP)", got)
	}
	if got := get("mkg_baru_tahun"); got != "6" {
		t.Fatalf("mkg_baru_tahun = %s, ingin 6 (KGB + 2)", got)
	}
	if got := get("mkg_baru_bulan"); got != "0" {
		t.Fatalf("mkg_baru_bulan = %s, ingin 0 (KGB selalu 0 bulan)", got)
	}
}

// submitKGBPNS mengirim form pengajuan PNS lengkap dan mengembalikan respons.
// unitID opsional: bila diisi, form menyertakan unit tujuan tersebut.
func submitKGBPNS(t *testing.T, fx fixture, unitID int64, kgbMasaTahun, kpMasaTahun, kpMasaBulan, kpTanggal string) (*http.Response, map[string]any) {
	t.Helper()
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"tmt_awal":            "2016-04-01",
		"birth_place":         "Grobogan",
		"birth_date":          "1980-01-01",
		"karpeg":              "I 123456",
		"pangkat":             "Penata Muda Tingkat I",
		"jabatan":             "Guru Ahli Pertama",
		"last_sk_pejabat":     "Kepala Dinas Pendidikan",
		"last_sk_tanggal":     "2024-04-01",
		"last_sk_nomor":       "800/001/4.2/2024",
		"last_sk_tmt":         "2024-04-01",
		"last_kgb_golongan":   "III/b",
		"last_kgb_masa_tahun": kgbMasaTahun,
		"last_kgb_masa_bulan": "0",
		"last_kp_golongan":    "III/b",
		"last_kp_tmt":         "2024-04-01",
		"last_kp_masa_tahun":  kpMasaTahun,
		"last_kp_masa_bulan":  kpMasaBulan,
		"last_kp_nomor":       "800.1.3.2/795/2024",
		"last_kp_tanggal":     kpTanggal,
		"last_kp_pejabat":     "BUPATI GROBOGAN",
	}
	if unitID > 0 {
		fields["unit_id"] = fmt.Sprint(unitID)
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

// TestSubmitPoin8TerpisahDariSKPemenang mengunci aturan di jalur simpan:
// saat KP 5/7 lebih baru, naskah tersimpan tetap poin 6 = 5/7 dan poin 8 = 6/0.
func TestSubmitPoin8TerpisahDariSKPemenang(t *testing.T) {
	fx := newFixture(t)
	resp, data := submitKGBPNS(t, fx, 0, "4", "5", "7", "2026-06-20")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d data=%v", resp.StatusCode, data)
	}
	get := func(k string) string { return fmt.Sprint(data[k]) }
	if got := get("draft_mkg_lama_tahun"); got != "5" {
		t.Fatalf("poin 6 tahun = %s, ingin 5", got)
	}
	if got := get("draft_mkg_lama_bulan"); got != "7" {
		t.Fatalf("poin 6 bulan = %s, ingin 7", got)
	}
	if got := get("draft_mkg_baru_tahun"); got != "6" {
		t.Fatalf("poin 8 tahun = %s, ingin 6 (KGB + 2)", got)
	}
	if got := get("draft_mkg_baru_bulan"); got != "0" {
		t.Fatalf("poin 8 bulan = %s, ingin 0", got)
	}
}

// TestSubmitTolakUnitDinasUntukPegawaiNonDinas memastikan unit tujuan bertipe
// Dinas tidak bisa dipilih pegawai unit lain: usulan akan berstatus
// menunggu_unit padahal unit Dinas tidak punya verifikator unit, sehingga
// pengajuan tersangkut tanpa jalur keluar.
func TestSubmitTolakUnitDinasUntukPegawaiNonDinas(t *testing.T) {
	fx := newFixture(t)
	ctx := context.Background()
	var dinasID int64
	if err := fx.pool.QueryRow(ctx,
		`INSERT INTO units (code, name, type) VALUES ('DINAS', 'Dinas Pendidikan', 'dinas') RETURNING id`).Scan(&dinasID); err != nil {
		t.Fatal(err)
	}
	resp, data := submitKGBPNS(t, fx, dinasID, "4", "4", "0", "2024-04-01")
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d data=%v, ingin 422", resp.StatusCode, data)
	}
}

// TestSalaryPreviewPoin8MasaKPLebihBesar mengunci bentuk #119: masa SK KP
// (26 th) lebih besar dari masa SK KGB (24 th). Poin 8 harus 28/0 — jika
// dipatok ke masa SK KGB, hasilnya 26 dan gaji lama justru turun dari bracket
// 26 ke bracket 24.
func TestSalaryPreviewPoin8MasaKPLebihBesar(t *testing.T) {
	fx := newFixture(t)
	client, _ := loginClient(t, fx.srv.URL, nipPNS, nipPNS)

	url := fx.srv.URL + "/api/v1/salary-preview?tmt_awal=2016-04-01&last_sk_tmt=2024-04-01&golongan=III/b" +
		"&last_kgb_masa_tahun=24&last_kgb_masa_bulan=0" +
		"&last_kp_masa_tahun=26&last_kp_masa_bulan=0" +
		"&last_kp_tanggal=2025-04-14&last_sk_tanggal=2024-12-26"
	resp, err := client.Get(url)
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
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d err=%v", resp.StatusCode, envelope.Error)
	}
	get := func(k string) string { return fmt.Sprint(envelope.Data[k]) }
	if got := get("mkg_lama_tahun"); got != "26" {
		t.Fatalf("poin 6 tahun = %s, ingin 26 (mentah SK KP)", got)
	}
	if got := get("mkg_baru_tahun"); got != "28" {
		t.Fatalf("poin 8 tahun = %s, ingin 28 (grid 26 + 2)", got)
	}
	if got := get("mkg_baru_bulan"); got != "0" {
		t.Fatalf("poin 8 bulan = %s, ingin 0", got)
	}
}
