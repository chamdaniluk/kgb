package store

import (
	"context"
	"testing"
	"time"
)

// Alur jenjang: TK/SD sekecamatan terlihat Korwil-nya; pegawai Korwil
// diverifikasi sub di Korwil-nya sendiri; usulan Dinas langsung
// menunggu_dinas; admin dinas hanya memproses unit dinas (dicek di handler).
func TestAlurJenjangKorwilDanDinasLangsung(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}

	var korwilBrati, korwilGabus int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district) VALUES ('KORWIL-BRATI','KORWILCAM BRATI','korwil','BRATI') RETURNING id`).Scan(&korwilBrati); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district) VALUES ('KORWIL-GABUS','KORWILCAM GABUS','korwil','GABUS') RETURNING id`).Scan(&korwilGabus); err != nil {
		t.Fatal(err)
	}
	// SD tanpa parent_id (kondisi live) tetapi berkecamatan BRATI.
	var sdBrati int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district) VALUES ('SDN-1-BRATI','SDN 1 BRATI','sd','BRATI') RETURNING id`).Scan(&sdBrati); err != nil {
		t.Fatal(err)
	}
	var seksiDinas int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district) VALUES ('SEKSI-PSDM','Seksi PSDM','dinas','DINAS') RETURNING id`).Scan(&seksiDinas); err != nil {
		t.Fatal(err)
	}

	mkTeacher := func(nip, name string, unitID int64) int64 {
		var uid int64
		if err := pool.QueryRow(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id) VALUES ($1,'x','asn',$2,$3) RETURNING id`, nip, name, unitID).Scan(&uid); err != nil {
			t.Fatal(err)
		}
		var tid int64
		if err := pool.QueryRow(ctx, `INSERT INTO teachers (user_id,nip,name,asn_type,unit_id,pangkat_gol,masa_kerja_tahun,masa_kerja_source) VALUES ($1,$2,$3,'pns',$4,'III/b',10,'tmt_cpns') RETURNING id`, uid, nip, name, unitID).Scan(&tid); err != nil {
			t.Fatal(err)
		}
		return tid
	}
	tmt := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	draft := LetterDraft{BirthPlace: "Grobogan", Karpeg: "K 1", Pangkat: "Penata Muda", Jabatan: "Guru Ahli Pertama", LastSKPejabat: "Bupati", LastSKNomor: "1", MKGLamaTahun: intPtr(8), MKGLamaBulan: intPtr(0), MKGBaruTahun: intPtr(10), MKGBaruBulan: intPtr(0)}

	tidSD := mkTeacher("198001012005011001", "Guru SD Brati", sdBrati)
	tidDinas := mkTeacher("198001012005011002", "Pegawai Seksi PSDM", seksiDinas)
	tidKorwil := mkTeacher("198001012005011003", "Pegawai Korwil Brati", korwilBrati)

	subSD, err := CreateSubmission(ctx, pool, tidSD, 1, tmt, intPtr(10), nil, &tmt, TeacherChange{}, draft, "100", "200", SubmissionFiles{Main: SubmissionFile{Name: "a.pdf", Path: "p/a.pdf", Size: 10}}, "127.0.0.1")
	if err != nil {
		t.Fatalf("buat usulan SD: %v", err)
	}
	if subSD.Status != "menunggu_unit" {
		t.Errorf("status usulan SD = %q, ingin menunggu_unit", subSD.Status)
	}
	if subSD.UnitType != "sd" || subSD.UnitDistrict != "BRATI" {
		t.Errorf("jenjang usulan SD = %q/%q, ingin sd/BRATI", subSD.UnitType, subSD.UnitDistrict)
	}
	subDinas, err := CreateSubmission(ctx, pool, tidDinas, 1, tmt, intPtr(10), nil, &tmt, TeacherChange{}, draft, "100", "200", SubmissionFiles{Main: SubmissionFile{Name: "b.pdf", Path: "p/b.pdf", Size: 10}}, "127.0.0.1")
	if err != nil {
		t.Fatalf("buat usulan dinas: %v", err)
	}
	if subDinas.Status != "menunggu_dinas" {
		t.Errorf("status usulan dinas = %q, ingin menunggu_dinas", subDinas.Status)
	}
	// Pegawai Korwil: usulannya menunggu_unit dan terlihat Korwil-nya sendiri.
	subKorwil, err := CreateSubmission(ctx, pool, tidKorwil, 1, tmt, intPtr(10), nil, &tmt, TeacherChange{}, draft, "100", "200", SubmissionFiles{Main: SubmissionFile{Name: "c.pdf", Path: "p/c.pdf", Size: 10}}, "127.0.0.1")
	if err != nil {
		t.Fatalf("buat usulan korwil: %v", err)
	}
	if subKorwil.Status != "menunggu_unit" {
		t.Errorf("status usulan korwil = %q, ingin menunggu_unit", subKorwil.Status)
	}
	if subKorwil.UnitType != "korwil" || subKorwil.UnitDistrict != "BRATI" {
		t.Errorf("jenjang usulan korwil = %q/%q, ingin korwil/BRATI", subKorwil.UnitType, subKorwil.UnitDistrict)
	}

	// Korwil Brati melihat SD sekecamatan walau tanpa parent_id.
	items, total, err := ListQueue(ctx, pool, "verifikator_unit", &korwilBrati, "menunggu_unit", NewPage(20, 0))
	if err != nil {
		t.Fatalf("antrean korwil brati: %v", err)
	}
	if total != 2 {
		t.Errorf("antrean korwil brati = total %d, ingin 2 (SD + pegawai korwil)", total)
	} else {
		seen := map[int64]bool{}
		for _, it := range items {
			seen[it.ID] = true
		}
		if !seen[subSD.ID] || !seen[subKorwil.ID] {
			t.Errorf("antrean korwil brati tidak memuat kedua usulan: %v", seen)
		}
	}
	// Korwil Gabus tidak melihat SD Brati.
	_, total, err = ListQueue(ctx, pool, "verifikator_unit", &korwilGabus, "menunggu_unit", NewPage(20, 0))
	if err != nil {
		t.Fatalf("antrean korwil gabus: %v", err)
	}
	if total != 0 {
		t.Errorf("antrean korwil gabus = %d, ingin 0", total)
	}

	// UnitInScope: kecamatan sama cukup; beda kecamatan ditolak.
	if ok, _ := UnitInScope(ctx, pool, korwilBrati, sdBrati); !ok {
		t.Error("UnitInScope korwil brati -> SD brati = false, ingin true")
	}
	if ok, _ := UnitInScope(ctx, pool, korwilGabus, sdBrati); ok {
		t.Error("UnitInScope korwil gabus -> SD brati = true, ingin false")
	}
	// Pegawai Korwil dalam scope Korwil-nya sendiri, bukan Korwil lain.
	if ok, _ := UnitInScope(ctx, pool, korwilBrati, korwilBrati); !ok {
		t.Error("UnitInScope korwil brati -> korwil brati = false, ingin true")
	}
	if ok, _ := UnitInScope(ctx, pool, korwilGabus, korwilBrati); ok {
		t.Error("UnitInScope korwil gabus -> korwil brati = true, ingin false")
	}
}

func intPtr(v int) *int { return &v }
