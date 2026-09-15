package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"sicendikia/internal/auth"
)

// seedUnitKoreksi menyiapkan Korwil (kecamatan BRATI), SD sekecamatan, SD luar
// kecamatan, dan akun verifikator_unit untuk Korwil tersebut.
func seedUnitKoreksi(t *testing.T, fx fixture) (sdDalam, sdLuar int64) {
	t.Helper()
	ctx := context.Background()
	hash := func(pw string) string {
		h, err := auth.HashPassword(pw)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	insertUnit := func(code, name, typ, district string) int64 {
		t.Helper()
		var id int64
		if err := fx.pool.QueryRow(ctx,
			`INSERT INTO units (code,name,type,district) VALUES ($1,$2,$3,$4) RETURNING id`,
			code, name, typ, district).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	korwil := insertUnit("KW-BRT", "KORWILCAM BRATI", "korwil", "BRATI")
	sdDalam = insertUnit("SD-BRT-1", "SDN 1 BRATI", "sd", "BRATI")
	sdLuar = insertUnit("SD-GBG-1", "SDN 1 GROBOGAN", "sd", "GROBOGAN")
	if _, err := fx.pool.Exec(ctx,
		`INSERT INTO users (username,password_hash,role,name,unit_id) VALUES ('kw-brati',$1,'verifikator_unit','Korwil Brati',$2)`,
		hash("sandi-korwil"), korwil); err != nil {
		t.Fatal(err)
	}
	return sdDalam, sdLuar
}

// seedPengajuanUnit membuat guru pada unit tertentu beserta usulan berstatus
// yang diminta, lalu mengembalikan id usulan.
func seedPengajuanUnit(t *testing.T, fx fixture, unitID int64, nip, status string) int64 {
	t.Helper()
	ctx := context.Background()
	var userID int64
	hash, err := auth.HashPassword(nip)
	if err != nil {
		t.Fatal(err)
	}
	if err := fx.pool.QueryRow(ctx,
		`INSERT INTO users (username,password_hash,role,name,unit_id) VALUES ($1,$2,'asn',$3,$4) RETURNING id`,
		nip, hash, "Guru "+nip, unitID).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var teacherID int64
	if err := fx.pool.QueryRow(ctx,
		`INSERT INTO teachers (user_id,nip,name,asn_type,unit_id,pangkat_gol,masa_kerja_tahun,masa_kerja_source,tmt_kgb_last,tmt_awal)
		 VALUES ($1,$2,$3,'pns',$4,'III/b',12,'tmt_cpns','2024-04-01','2016-04-01') RETURNING id`,
		userID, nip, "Guru "+nip, unitID).Scan(&teacherID); err != nil {
		t.Fatal(err)
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
		VALUES ($1,$2,'2026-04-01','legacy.pdf','/tmp/legacy.pdf',100,
			'kgb.pdf','/tmp/kgb.pdf',100, now(),
			'Grobogan','1980-01-01','K 1','Penata Muda','Guru Ahli Pertama',
			'Bupati','2024-04-01','800/1/2024','2024-04-01',
			4,0, 'III/b','2024-04-01','KP/1','2024-04-01','Bupati', 5,7, 5,7,6,0,
			3089300,3186600)
		RETURNING id`, teacherID, status).Scan(&subID); err != nil {
		t.Fatal(err)
	}
	return subID
}

func koreksiUnitReq(t *testing.T, fx fixture, subID int64, username, password string, override map[string]string) (*http.Response, map[string]any) {
	t.Helper()
	return koreksiFormReq(t, fx, fmt.Sprintf("/api/v1/verifications/unit/%d/koreksi", subID), username, password, override)
}

// TestKoreksiUnitDalamScope: Korwil dapat mengoreksi usulan SD sekecamatannya
// pada tahap menunggu_unit; status tetap dan jejaknya tercatat action koreksi_unit.
func TestKoreksiUnitDalamScope(t *testing.T) {
	fx := newFixture(t)
	sdDalam, _ := seedUnitKoreksi(t, fx)
	subID := seedPengajuanUnit(t, fx, sdDalam, "198001012005011021", "menunggu_unit")

	resp, errBody := koreksiUnitReq(t, fx, subID, "kw-brati", "sandi-korwil", map[string]string{
		"karpeg":      "I 777777",
		"birth_place": "Semarang",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d err=%v, ingin 200", resp.StatusCode, errBody)
	}
	var status, karpeg string
	if err := fx.pool.QueryRow(context.Background(),
		`SELECT status, draft_karpeg FROM submissions WHERE id=$1`, subID).Scan(&status, &karpeg); err != nil {
		t.Fatal(err)
	}
	if status != "menunggu_unit" {
		t.Fatalf("status = %s, ingin tetap menunggu_unit", status)
	}
	if karpeg != "I 777777" {
		t.Fatalf("karpeg = %s, ingin I 777777", karpeg)
	}
	var n int
	if err := fx.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE submission_id=$1 AND action='koreksi_unit'`, subID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("audit koreksi_unit = %d, ingin 1", n)
	}
}

// TestKoreksiUnitLuarScope: Korwil tidak berwenang atas usulan SD di kecamatan
// lain (403) — scope mengikuti relasi parent/kecamatan yang sama.
func TestKoreksiUnitLuarScope(t *testing.T) {
	fx := newFixture(t)
	_, sdLuar := seedUnitKoreksi(t, fx)
	subID := seedPengajuanUnit(t, fx, sdLuar, "198001012005011022", "menunggu_unit")

	resp, errBody := koreksiUnitReq(t, fx, subID, "kw-brati", "sandi-korwil", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d err=%v, ingin 403", resp.StatusCode, errBody)
	}
}

// TestKoreksiUnitTahap: unit masih boleh mengoreksi setelah usulannya
// diteruskan (menunggu_dinas), tetapi sudah tertutup saat menunggu TTE karena
// nomor surat bisa sudah tercadangkan.
func TestKoreksiUnitTahap(t *testing.T) {
	fx := newFixture(t)
	sdDalam, _ := seedUnitKoreksi(t, fx)

	lanjut := seedPengajuanUnit(t, fx, sdDalam, "198001012005011023", "menunggu_dinas")
	resp, errBody := koreksiUnitReq(t, fx, lanjut, "kw-brati", "sandi-korwil", map[string]string{
		"karpeg": "I 888888",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("koreksi menunggu_dinas: status=%d err=%v, ingin 200", resp.StatusCode, errBody)
	}

	tte := seedPengajuanUnit(t, fx, sdDalam, "198001012005011024", "menunggu_tte")
	resp, errBody = koreksiUnitReq(t, fx, tte, "kw-brati", "sandi-korwil", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("koreksi menunggu_tte: status=%d err=%v, ingin 409", resp.StatusCode, errBody)
	}
}

// TestKoreksiUnitSmpUnitSendiri: SMP/SKB hanya berwenang atas unitnya sendiri,
// bukan unit SMP lain.
func TestKoreksiUnitSmpUnitSendiri(t *testing.T) {
	fx := newFixture(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("sandi-smp")
	if err != nil {
		t.Fatal(err)
	}
	insertUnit := func(code, name string) int64 {
		t.Helper()
		var id int64
		if err := fx.pool.QueryRow(ctx,
			`INSERT INTO units (code,name,type,district) VALUES ($1,$2,'smp','GROBOGAN') RETURNING id`,
			code, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	smpSendiri := insertUnit("SMP-1-GBG", "SMPN 1 GROBOGAN")
	smpLain := insertUnit("SMP-2-GBG", "SMPN 2 GROBOGAN")
	if _, err := fx.pool.Exec(ctx,
		`INSERT INTO users (username,password_hash,role,name,unit_id) VALUES ('smp-1',$1,'verifikator_unit','Admin SMPN 1',$2)`,
		hash, smpSendiri); err != nil {
		t.Fatal(err)
	}

	milik := seedPengajuanUnit(t, fx, smpSendiri, "198001012005011025", "menunggu_unit")
	resp, errBody := koreksiUnitReq(t, fx, milik, "smp-1", "sandi-smp", map[string]string{
		"karpeg": "I 666666",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("koreksi unit sendiri: status=%d err=%v, ingin 200", resp.StatusCode, errBody)
	}

	lain := seedPengajuanUnit(t, fx, smpLain, "198001012005011026", "menunggu_unit")
	resp, errBody = koreksiUnitReq(t, fx, lain, "smp-1", "sandi-smp", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("koreksi unit lain: status=%d err=%v, ingin 403", resp.StatusCode, errBody)
	}
}

// TestKoreksiUnitPeranLain: hanya verifikator_unit yang boleh memakai jalur
// koreksi unit — admin/verifikator Dinas ditolak di gerbang peran (403).
func TestKoreksiUnitPeranLain(t *testing.T) {
	fx := newFixture(t)
	seedAdminDinas(t, fx)
	sdDalam, _ := seedUnitKoreksi(t, fx)
	subID := seedPengajuanUnit(t, fx, sdDalam, "198001012005011027", "menunggu_unit")

	resp, errBody := koreksiUnitReq(t, fx, subID, "admin.disdik1", "sandi-admin", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("koreksi unit oleh admin dinas: status=%d err=%v, ingin 403", resp.StatusCode, errBody)
	}
}
