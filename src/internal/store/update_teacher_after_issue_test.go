package store

import (
	"context"
	"testing"
	"time"
)

// Regression: UpdateTeacherAfterIssue pernah gagal TOTAL ("inconsistent types
// for parameter $2", 42P08) sehingga semua penerbitan (TTE manual & esign)
// gagal. Pakai-ulang placeholder $1/$2 dalam deduksi COALESCE adalah
// pemicunya; kini placeholder terpisah + cast eksplisit.
func TestUpdateTeacherAfterIssueDiterbitkan(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	resetSchema(t, pool)
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("migrasi: %v", err)
	}
	var unitID int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type) VALUES ('SCRATCH-1','Scratch','korwil') RETURNING id`).Scan(&unitID); err != nil {
		t.Fatal(err)
	}
	var teacherID int64
	if err := pool.QueryRow(ctx, `INSERT INTO teachers (nip, name, asn_type, unit_id, pangkat_gol, masa_kerja_tahun, masa_kerja_source, tmt_awal) VALUES ('199001012024011001','Scratch Guru','pns',$1,'III/b',4,'tmt_cpns','2020-12-01') RETURNING id`, unitID).Scan(&teacherID); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	draft := LetterDraft{
		BirthPlace: "Grobogan", Karpeg: "K 1", Pangkat: "Penata Muda", Jabatan: "Guru Ahli Pertama",
		LastSKPejabat: "Kepala Dinas", LastSKTanggal: date("2024-10-08"), LastSKNomor: "800/011/2024",
		LastSKMasaTahun: intPtr(4), LastSKMasaBulan: intPtr(0),
		LastKPGolongan: "III/b", LastKPTMT: date("2026-07-01"), LastKPNomor: "800.1.3.2/491/2026",
		LastKPMasaTahun: intPtr(5), LastKPMasaBulan: intPtr(7),
	}
	tmtAwal := date("2020-12-01")
	if err := UpdateTeacherAfterIssue(ctx, tx, teacherID, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), 6, draft, "III/b", unitID, tmtAwal); err != nil {
		t.Fatalf("UpdateTeacherAfterIssue: %v", err)
	}
}
