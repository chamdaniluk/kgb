package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"

	"sicendikia/internal/auth"
)

// seedAdminDinas membuat akun admin_dinas dan verifikator_dinas untuk uji.
func seedAdminDinas(t *testing.T, fx fixture) (adminUser, verifUser string) {
	t.Helper()
	ctx := context.Background()
	hash := func(pw string) string {
		h, err := auth.HashPassword(pw)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	for _, u := range []struct{ username, role, password string }{
		{"admin.disdik1", "admin_dinas", "sandi-admin"},
		{"verif.dinas", "verifikator_dinas", "sandi-verif"},
	} {
		if _, err := fx.pool.Exec(ctx,
			`INSERT INTO users (username,password_hash,role,name) VALUES ($1,$2,$3,$4)`,
			u.username, hash(u.password), u.role, u.username); err != nil {
			t.Fatal(err)
		}
	}
	return "admin.disdik1", "verif.dinas"
}

// koreksiReq mengirim form koreksi dengan override field tertentu.
func koreksiReq(t *testing.T, fx fixture, subID int64, username, password string, override map[string]string) (*http.Response, map[string]any) {
	t.Helper()
	client, csrf := loginClient(t, fx.srv.URL, username, password)
	fields := map[string]string{
		"tmt_awal":            "2016-04-01",
		"birth_place":         "Grobogan",
		"birth_date":          "1980-01-01",
		"karpeg":              "I 123456",
		"pangkat":             "Penata Muda Tingkat I",
		"jabatan":             "Guru Ahli Pertama",
		"last_kp_golongan":    "III/b",
		"last_kp_tmt":         "2024-04-01",
		"last_kp_masa_tahun":  "5",
		"last_kp_masa_bulan":  "7",
		"last_kp_nomor":       "KP/1",
		"last_kp_tanggal":     "2024-04-01",
		"last_kp_pejabat":     "Bupati",
		"last_kgb_golongan":   "III/b",
		"last_sk_tmt":         "2024-04-01",
		"last_kgb_masa_tahun": "4",
		"last_kgb_masa_bulan": "0",
		"last_sk_nomor":       "800/1/2024",
		"last_sk_tanggal":     "2024-04-01",
		"last_sk_pejabat":     "Bupati",
		"pangkat_gol":         "III/b",
	}
	for k, v := range override {
		fields[k] = v
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for k, v := range fields {
		_ = writer.WriteField(k, v)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost,
		fx.srv.URL+fmt.Sprintf("/api/v1/verifications/dinas/%d/koreksi", subID), &body)
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
	_ = json.NewDecoder(resp.Body).Decode(&envelope)
	return resp, envelope.Error
}

// TestKoreksiDinasUbahIdentitas: perbaikan identitas naskah tanpa catatan khusus
// diterima, status tetap menunggu_dinas (tidak diteruskan ke pimpinan), dan
// jejak koreksi tercatat di audit.
func TestKoreksiDinasUbahIdentitas(t *testing.T) {
	fx := newFixture(t)
	seedAdminDinas(t, fx)
	subID := siapkanPengajuan(t, fx, "menunggu_dinas", true)

	resp, errBody := koreksiReq(t, fx, subID, "admin.disdik1", "sandi-admin", map[string]string{
		"karpeg":      "I 999999",
		"birth_place": "Semarang",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d err=%v, ingin 200", resp.StatusCode, errBody)
	}
	// Status tidak boleh berpindah ke TTE.
	var status, karpeg string
	if err := fx.pool.QueryRow(context.Background(),
		`SELECT status, draft_karpeg FROM submissions WHERE id=$1`, subID).Scan(&status, &karpeg); err != nil {
		t.Fatal(err)
	}
	if status != "menunggu_dinas" {
		t.Fatalf("status = %s, ingin tetap menunggu_dinas", status)
	}
	if karpeg != "I 999999" {
		t.Fatalf("karpeg = %s, ingin I 999999", karpeg)
	}
	// Audit berisi nilai sebelum/sesudah dan daftar kolom yang berubah.
	var n int
	if err := fx.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE submission_id=$1 AND action='koreksi_dinas'`, subID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("audit koreksi_dinas = %d, ingin 1", n)
	}
}

// TestKoreksiDinasGajiWajibCatatan: koreksi yang mengubah gaji/TMT tanpa catatan
// ditolak 400; dengan catatan memadai diterima.
func TestKoreksiDinasGajiWajibCatatan(t *testing.T) {
	fx := newFixture(t)
	seedAdminDinas(t, fx)
	subID := siapkanPengajuan(t, fx, "menunggu_dinas", true)

	// Mengubah masa kerja SK pemenang (KP 5 → 9 th) → grid gaji ikut berubah
	// (4 → 8) sehingga gaji lama/baru berubah → catatan wajib.
	resp, errBody := koreksiReq(t, fx, subID, "admin.disdik1", "sandi-admin", map[string]string{
		"last_kp_masa_tahun": "9",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d err=%v, ingin 400 (catatan wajib)", resp.StatusCode, errBody)
	}
	if !bytes.Contains([]byte(fmt.Sprint(errBody)), []byte("catatan")) {
		t.Fatalf("pesan = %v, ingin menyebut catatan", errBody)
	}

	// Catatan terlalu pendek tetap ditolak.
	resp, _ = koreksiReq(t, fx, subID, "admin.disdik1", "sandi-admin", map[string]string{
		"last_kp_masa_tahun": "9",
		"koreksi_note":       "salah",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, ingin 400 (catatan < 10 karakter)", resp.StatusCode)
	}

	// Catatan memadai → diterima, dan gaji tersimpan ikut berubah.
	resp, errBody = koreksiReq(t, fx, subID, "admin.disdik1", "sandi-admin", map[string]string{
		"last_kp_masa_tahun": "9",
		"koreksi_note":       "Masa kerja SK dikoreksi sesuai SK KP terakhir",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d err=%v, ingin 200 dengan catatan memadai", resp.StatusCode, errBody)
	}
	// Gaji tersimpan harus ikut terkoreksi (grid 4 → 8).
	var cur string
	if err := fx.pool.QueryRow(context.Background(),
		`SELECT current_salary::text FROM submissions WHERE id=$1`, subID).Scan(&cur); err != nil {
		t.Fatal(err)
	}
	if cur == "3089300" {
		t.Fatalf("current_salary masih %s, koreksi gaji tidak diterapkan", cur)
	}
}

// TestKoreksiDinasStatusSalah: koreksi ditolak bila usulan sudah diteruskan ke
// pimpinan (menunggu_tte) — nomor surat di tahap itu bisa sudah tercadangkan.
func TestKoreksiDinasStatusSalah(t *testing.T) {
	fx := newFixture(t)
	seedAdminDinas(t, fx)
	subID := siapkanPengajuan(t, fx, "menunggu_tte", true)
	resp, errBody := koreksiReq(t, fx, subID, "admin.disdik1", "sandi-admin", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d err=%v, ingin 409", resp.StatusCode, errBody)
	}
}

// TestKoreksiDinasHanyaAdminDinas: verifikator_dinas tidak berwenang (403),
// sesuai keputusan owner bahwa koreksi hanya untuk admin Dinas.
func TestKoreksiDinasHanyaAdminDinas(t *testing.T) {
	fx := newFixture(t)
	seedAdminDinas(t, fx)
	subID := siapkanPengajuan(t, fx, "menunggu_dinas", true)
	resp, errBody := koreksiReq(t, fx, subID, "verif.dinas", "sandi-verif", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d err=%v, ingin 403", resp.StatusCode, errBody)
	}
}
