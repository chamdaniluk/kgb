package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"sicendikia/internal/auth"
	"sicendikia/internal/files"
	"sicendikia/internal/store"
)

// siapkanPengajuan mengembalikan id pengajuan PNS untuk uji kirim ulang.
// status menentukan status awal; hasKGB mengisi slot KGB (skema 017) atau tidak
// (skema lama: hanya berkas utama).
func siapkanPengajuan(t *testing.T, fx fixture, status string, hasKGB bool) int64 {
	t.Helper()
	ctx := context.Background()
	var teacherID int64
	if err := fx.pool.QueryRow(ctx, `SELECT id FROM teachers WHERE nip=$1`, nipPNS).Scan(&teacherID); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(os.TempDir(), "si-cendikia-uploads", "legacy.pdf")
	kgbName, kgbPath := "", ""
	if hasKGB {
		kgbName = "kgb.pdf"
		kgbPath = filepath.Join(os.TempDir(), "si-cendikia-uploads", "kgb.pdf")
	}
	var subID int64
	if err := fx.pool.QueryRow(ctx, `
		INSERT INTO submissions (teacher_id, status, proposed_tmt, file_name, file_path, file_size,
			file_kgb_name, file_kgb_path, file_kgb_size, submitted_at,
			draft_birth_place, draft_birth_date, draft_karpeg, draft_pangkat, draft_jabatan,
			draft_last_sk_pejabat, draft_last_sk_tanggal, draft_last_sk_nomor, draft_last_sk_tmt,
			draft_last_sk_masa_tahun, draft_last_sk_masa_bulan,
			draft_last_kp_golongan, draft_last_kp_tmt, draft_last_kp_nomor, draft_last_kp_tanggal, draft_last_kp_pejabat,
			draft_last_kp_masa_tahun, draft_last_kp_masa_bulan,
			draft_mkg_lama_tahun, draft_mkg_lama_bulan, draft_mkg_baru_tahun, draft_mkg_baru_bulan,
			current_salary, next_salary)
		VALUES ($1,$2,'2026-04-01','legacy.pdf',$3,100,
			NULLIF($4,''),NULLIF($5,''),0, now(),
			'Grobogan','1980-01-01','K 1','Penata Muda','Guru Ahli Pertama',
			'Bupati','2024-04-01','800/1/2024','2024-04-01',
			4,0, 'III/b','2024-04-01','KP/1','2024-04-01','Bupati', 5,7, 5,7,6,0,
			3089300,3186600)
		RETURNING id`, teacherID, status, mainPath, kgbName, kgbPath).Scan(&subID); err != nil {
		t.Fatal(err)
	}
	return subID
}

// resubmitReq mengirim form kirim ulang; files memetakan field slot -> nama berkas.
func resubmitReq(t *testing.T, fx fixture, subID int64, uploads map[string]string) (*http.Response, map[string]any) {
	t.Helper()
	client, csrf := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for k, v := range map[string]string{
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
	} {
		_ = writer.WriteField(k, v)
	}
	for field, name := range uploads {
		p, err := writer.CreateFormFile(field, name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = p.Write([]byte("%PDF-1.4\n% test\n"))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, fx.srv.URL+fmt.Sprintf("/api/v1/submissions/%d/resubmit", subID), &body)
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

// TestResubmitWajibSemuaSK: pengajuan skema lama (slot KGB kosong) ditolak saat
// kirim ulang tanpa mengunggah KGB — aturannya semua SK wajib lengkap.
func TestResubmitWajibSemuaSK(t *testing.T) {
	fx := newFixture(t)
	subID := siapkanPengajuan(t, fx, "dikembalikan_unit", false)

	resp, errBody := resubmitReq(t, fx, subID, nil)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d err=%v, ingin 422 (semua SK wajib lengkap)", resp.StatusCode, errBody)
	}
	if !bytes.Contains([]byte(fmt.Sprint(errBody)), []byte("KGB terakhir")) {
		t.Fatalf("pesan = %v, ingin menyebut 'KGB terakhir'", errBody)
	}

	// Unggah KGB terakhir saja (KP terpenuhi lewat berkas utama) → lolos.
	resp, errBody = resubmitReq(t, fx, subID, map[string]string{"file_kgb": "kgb.pdf"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d err=%v, ingin 200 setelah KGB diunggah", resp.StatusCode, errBody)
	}
}

// TestResubmitSlotLamaBolehKosong: slot yang sudah ada boleh dibiarkan kosong.
func TestResubmitSlotLamaBolehKosong(t *testing.T) {
	fx := newFixture(t)
	subID := siapkanPengajuan(t, fx, "dikembalikan_unit", true)
	resp, errBody := resubmitReq(t, fx, subID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d err=%v, ingin 200 (berkas lama dipakai ulang)", resp.StatusCode, errBody)
	}
}

// TestResubmitGagalTidakHapusBerkasLama mengunci bug: bila penyimpanan kirim
// ulang gagal setelah slot lama disalin, berkas lama TIDAK boleh dihapus karena
// pengajuan berjalan masih memakainya. Kegagalan dipicu lewat konflik status
// (pengajuan tidak dalam status dikembalikan).
func TestResubmitGagalTidakHapusBerkasLama(t *testing.T) {
	ctx := context.Background()
	pool := testPoolDB(t)
	resetSchema(t, pool)
	if _, err := store.Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("migrasi: %v", err)
	}
	seed, err := os.ReadFile(filepath.Join("..", "..", "data", "seeds", "001_salary_scales.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(seed)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	root := t.TempDir()
	filesStore, err := files.New(root)
	if err != nil {
		t.Fatal(err)
	}
	api := NewWithDependencies(pool, "secret-test", false, Dependencies{Files: filesStore})
	srv := httptest.NewServer(api.Routes())
	t.Cleanup(srv.Close)

	hash, err := auth.HashPassword(nipPNS)
	if err != nil {
		t.Fatal(err)
	}
	var unitID, userID, teacherID int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type) VALUES ('KORWIL-01','Korwil','korwil') RETURNING id`).Scan(&unitID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id) VALUES ($1,$2,'asn','Guru PNS Contoh',$3) RETURNING id`, nipPNS, hash, unitID).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO teachers (user_id,nip,name,asn_type,unit_id,pangkat_gol,masa_kerja_tahun,masa_kerja_source,tmt_kgb_last,tmt_awal)
		VALUES ($1,$2,'Guru PNS Contoh','pns',$3,'III/b',12,'tmt_cpns','2024-04-01','2016-04-01') RETURNING id`, userID, nipPNS, unitID).Scan(&teacherID); err != nil {
		t.Fatal(err)
	}

	// Dua berkas lama nyata: KP (slot lama via berkas utama) dan KGB.
	oldKP, _, err := filesStore.SavePDF(ctx, bytes.NewReader([]byte("%PDF-1.4\n% lama-kp\n")), "lama-kp.pdf", 18)
	if err != nil {
		t.Fatal(err)
	}
	oldKGB, _, err := filesStore.SavePDF(ctx, bytes.NewReader([]byte("%PDF-1.4\n% lama-kgb\n")), "lama-kgb.pdf", 19)
	if err != nil {
		t.Fatal(err)
	}
	// Status 'menunggu_unit' (bukan dikembalikan) → store.Resubmit menolak.
	var subID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO submissions (teacher_id,status,proposed_tmt,file_name,file_path,file_size,
			file_kgb_name,file_kgb_path,file_kgb_size,submitted_at,
			draft_birth_place,draft_birth_date,draft_karpeg,draft_pangkat,draft_jabatan,
			draft_last_sk_pejabat,draft_last_sk_tanggal,draft_last_sk_nomor,draft_last_sk_tmt,
			draft_last_sk_masa_tahun,draft_last_sk_masa_bulan,
			draft_last_kp_golongan,draft_last_kp_tmt,draft_last_kp_nomor,draft_last_kp_tanggal,draft_last_kp_pejabat,
			draft_last_kp_masa_tahun,draft_last_kp_masa_bulan,
			draft_mkg_lama_tahun,draft_mkg_lama_bulan,draft_mkg_baru_tahun,draft_mkg_baru_bulan,
			current_salary,next_salary)
		VALUES ($1,'menunggu_unit','2026-04-01','lama-kp.pdf',$2,18,
			'lama-kgb.pdf',$3,19, now(),
			'Grobogan','1980-01-01','K 1','Penata Muda','Guru Ahli Pertama',
			'Bupati','2024-04-01','800/1/2024','2024-04-01', 4,0,
			'III/b','2024-04-01','KP/1','2024-04-01','Bupati', 5,7, 5,7,6,0,
			3089300,3186600)
		RETURNING id`, teacherID, oldKP, oldKGB).Scan(&subID); err != nil {
		t.Fatal(err)
	}

	// Kirim ulang tanpa unggahan: slot lama disalin, lalu store.Resubmit gagal
	// karena status bukan dikembalikan_*.
	fx := fixture{pool: pool, srv: srv}
	resp, errBody := resubmitReq(t, fx, subID, nil)
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("kirim ulang pada status menunggu_unit seharusnya gagal, dapat 200")
	}
	t.Logf("respons gagal sesuai harapan: status=%d err=%v", resp.StatusCode, errBody)

	// Berkas lama harus MASIH ADA.
	for _, p := range []string{oldKP, oldKGB} {
		f, err := filesStore.Open(p)
		if err != nil {
			t.Fatalf("berkas lama %s terhapus padahal masih dipakai pengajuan: %v", filepath.Base(p), err)
		}
		_ = f.Close()
	}
}
