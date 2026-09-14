package store

import (
	"context"
	"testing"
)

// TestUpsertImportedTeacherPertahankanParentUnit memastikan sinkron SIPP ASN
// (yang tidak mengirim kode unit induk) tidak menghapus tautan sekolah ke Korwil
// kecamatan. Tanpa ini, sinkron malam mengosongkan parent_id seluruh SD/TK.
func TestUpsertImportedTeacherPertahankanParentUnit(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	var korwil int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district) VALUES ('KORWILCAM-GROBOGAN','Korwil Grobogan','korwil','GROBOGAN') RETURNING id`).Scan(&korwil); err != nil {
		t.Fatal(err)
	}
	// Unit hasil perbaikan identitas: kode sudah ber-kd_unker dan tertaut Korwil.
	var sd int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district,parent_id,kd_unker) VALUES ('SDN-1-CONTOH-03-10-01','SDN 1 Contoh','sd','GROBOGAN',$1,'03.10.01') RETURNING id`, korwil).Scan(&sd); err != nil {
		t.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// Baris SIPP ASN: tanpa ParentUnitCode (sumber tidak mengirim unit induk).
	created, err := UpsertImportedTeacher(ctx, tx, ImportedTeacher{
		NIP: "199001012010011001", Name: "GURU CONTOH", ASNType: "pns", Kategori: KategoriGuru,
		UnitCode: "SDN-1-CONTOH-03-10-01", UnitName: "SDN 1 Contoh", UnitType: "sd",
		UnitDistrict: "GROBOGAN", KdUnker: "03.10.01", PangkatGol: "III/c", MasaKerjaTahun: 10,
		MasaKerjaSource: SyncSourceSIPPASN,
	}, func(string) (string, error) { return "x", nil })
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("guru baru harus dibuat")
	}

	// Kode unit berubah (kd_unker ditambahkan) tetapi baris harus tetap tertaut
	// ke Korwil, bukan kehilangan parent.
	var parentID *int64
	var code string
	if err := pool.QueryRow(ctx, `SELECT parent_id, code FROM units WHERE kd_unker='03.10.01'`).Scan(&parentID, &code); err != nil {
		t.Fatal(err)
	}
	if parentID == nil || *parentID != korwil {
		t.Fatalf("parent_id = %v, want %d (tautan Korwil harus dipertahankan)", parentID, korwil)
	}
	if code != "SDN-1-CONTOH-03-10-01" {
		t.Fatalf("code = %q, want SDN-1-CONTOH-03-10-01", code)
	}
	_ = sd
}
