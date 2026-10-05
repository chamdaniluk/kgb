package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"sicendikia/internal/store"
)

// Keputusan owner 2026-10-05: aturan pemenang SK terbaru (2026-09-09) hanya
// untuk NASKAH + jangkar hitungan. Isian draft_last_sk_* tersimpan harus
// SK KGB asli — koreksi/tolak-usulan harus bisa mengubah nilai tersimpan
// (kasus #165: koreksi korwil selalu dicatat "diubah": [] karena isian
// ditimpa pemenang sebelum disimpan).
func TestSubmitIsianSKKGBAsliMeskiKPMenang(t *testing.T) {
	fx := newFixture(t)
	ctx := context.Background()
	if _, err := fx.pool.Exec(ctx, `UPDATE teachers SET tmt_awal='2020-12-01', tmt_kgb_last='2024-01-01', last_sk_tmt_berlaku='2024-01-01', masa_kerja_source='tmt_cpns' WHERE nip=$1`, nipPNS); err != nil {
		t.Fatal(err)
	}
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"tmt_awal":            "2020-12-01",
		"birth_place":         "Grobogan",
		"birth_date":          "1980-01-01",
		"karpeg":              "I 123456",
		"pangkat":             "Penata Muda Tingkat I",
		"jabatan":             "Guru Ahli Pertama",
		// SK KGB terakhir (asli, dokumen 2024) — lebih LAMA dari SK KP.
		"last_sk_pejabat":     "Kepala Dinas Pendidikan",
		"last_sk_tanggal":     "2024-10-08",
		"last_sk_nomor":       "800/011/DISDIK/2024",
		"last_sk_tmt":         "2024-01-01",
		"last_kgb_golongan":   "III/a",
		"last_kgb_masa_tahun": "4",
		"last_kgb_masa_bulan": "0",
		// SK KP terakhir — lebih BARU (pemenang).
		"last_kp_golongan":    "III/b",
		"last_kp_tmt":         "2026-07-01",
		"last_kp_masa_tahun":  "5",
		"last_kp_masa_bulan":  "7",
		"last_kp_nomor":       "800.1.3.2/491/2026",
		"last_kp_tanggal":     "2026-06-03",
		"last_kp_pejabat":     "BUPATI GROBOGAN",
		"pangkat_gol":         "III/b",
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

	var gotNomor, gotKPNomor, gotGol string
	var gotTMT, gotTanggal time.Time
	if err := fx.pool.QueryRow(ctx, `SELECT draft_last_sk_nomor, draft_last_kp_nomor, draft_last_sk_tmt, draft_last_sk_tanggal, draft_last_kgb_golongan FROM submissions WHERE id=$1`, int64(envelope.Data["id"].(float64))).Scan(&gotNomor, &gotKPNomor, &gotTMT, &gotTanggal, &gotGol); err != nil {
		t.Fatal(err)
	}
	if gotNomor != "800/011/DISDIK/2024" {
		t.Fatalf("isian SK KGB tersimpan = %q, ingin nomor SK KGB asli (bukan pemenang KP)", gotNomor)
	}
	if gotGol != "III/a" {
		t.Fatalf("golongan KGB tersimpan = %q, ingin golongan saat SK KGB (III/a, bukan golongan efektif)", gotGol)
	}
	if gotKPNomor != "800.1.3.2/491/2026" {
		t.Fatalf("isian SK KP tersimpan = %q, ingin 800.1.3.2/491/2026", gotKPNomor)
	}
	if gotTMT.Format("2006-01-02") != "2024-01-01" || gotTanggal.Format("2006-01-02") != "2024-10-08" {
		t.Fatalf("TMT/tanggal SK KGB tersimpan = %q/%q, ingin 2024-01-01/2024-10-08", gotTMT.Format("2006-01-02"), gotTanggal.Format("2006-01-02"))
	}
	if !strings.HasPrefix(fmt.Sprint(envelope.Data["proposed_tmt"]), "2026-12-01") {
		t.Fatalf("proposed_tmt harus tetap mengikuti jangkar pemenang (2026-12-01), dapat %v", envelope.Data["proposed_tmt"])
	}

	// Naskah (pratinjau/terbit) tetap memakai identitas SK pemenang.
	sub, err := store.GetSubmission(ctx, fx.pool, int64(envelope.Data["id"].(float64)))
	if err != nil {
		t.Fatal(err)
	}
	nd := sub.NaskahDraft()
	if nd.LastSKNomor != "800.1.3.2/491/2026" || nd.LastSKPejabat != "BUPATI GROBOGAN" {
		t.Fatalf("naskah harus memakai identitas SK pemenang (KP), dapat nomor=%q pejabat=%q", nd.LastSKNomor, nd.LastSKPejabat)
	}
	if sub.DraftLastSKNomor != "800/011/DISDIK/2024" {
		t.Fatalf("isian tersimpan tidak boleh berubah, dapat %q", sub.DraftLastSKNomor)
	}
}
