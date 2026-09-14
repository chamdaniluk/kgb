package store

import (
	"context"
	"testing"
)

// TestRepairUnitIdentityMemulihkanSekolahBernamaSama mereproduksi temuan
// produksi 2026-09-14: dua sekolah bernama sama di kecamatan berbeda —
// karena units.code lama berbasis nama, hanya satu yang tersimpan.
// Perbaikan harus menyisipkan yang hilang, mengoreksi kecamatan, dan idempoten.
func TestRepairUnitIdentityMemulihkanSekolahBernamaSama(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}

	// Keadaan lama: Korwil 2 kecamatan, dan hanya SATU "SDN-1-KARANGANYAR"
	// yang bertahan (bentrok UNIQUE), ditempatkan di kecamatan yang salah.
	var korwilGeyer, korwilKrayung int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district) VALUES ('KORWILCAM GEYER','KORWILCAM GEYER','korwil','GEYER') RETURNING id`).Scan(&korwilGeyer); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district) VALUES ('KORWILCAM KARANGRAYUNG','KORWILCAM KARANGRAYUNG','korwil','KARANGRAYUNG') RETURNING id`).Scan(&korwilKrayung); err != nil {
		t.Fatal(err)
	}
	// SDN 1 Tanggungharjo (desa di Kec. Grobogan) salah masuk Kec. Tanggungharjo.
	_, _ = korwilGeyer, korwilKrayung
	var sdTanggeng int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district) VALUES ('SDN-1-TANGGUNGHARJO','SDN 1 Tanggungharjo','sd','TANGGUNGHARJO') RETURNING id`).Scan(&sdTanggeng); err != nil {
		t.Fatal(err)
	}
	var sdKaranganyarGeyer int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type,district) VALUES ('SDN-1-KARANGANYAR','SDN 1 Karanganyar','sd','GEYER') RETURNING id`).Scan(&sdKaranganyarGeyer); err != nil {
		t.Fatal(err)
	}
	// Guru tertaut ke unit lama: harus tetap tertaut setelah perbaikan.
	var teacherID, userID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id,is_active) VALUES ('199103312005011076','x','asn','GURU CONTOH',$1,true) RETURNING id`, sdKaranganyarGeyer).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO teachers (user_id,nip,name,asn_type,kategori,unit_id,pangkat_gol,masa_kerja_tahun,masa_kerja_source) VALUES ($1,'199103312005011076','GURU CONTOH','pns','guru',$2,'III/c',10,'sippasn') RETURNING id`, userID, sdKaranganyarGeyer).Scan(&teacherID); err != nil {
		t.Fatal(err)
	}

	officers := []SIPPASNOfficer{
		{NIP: "199103312005011076", Name: "GURU CONTOH", JobKind: "2", JobTitle: "Guru Ahli Muda", Golongan: "III/c", UnitCode: "03.09.27", UnitName: "SDN 1 Karanganyar - Dinas Pendidikan", Status: "1"},
		{NIP: "199103312005011077", Name: "GURU DUA", JobKind: "2", JobTitle: "Guru Ahli Muda", Golongan: "III/c", UnitCode: "03.21.02", UnitName: "SDN 1 Karanganyar - Dinas Pendidikan", Status: "1"},
		{NIP: "199103312005011078", Name: "GURU TIGA", JobKind: "2", JobTitle: "Guru Ahli Muda", Golongan: "III/c", UnitCode: "03.10.47", UnitName: "SDN 1 Tanggungharjo - Dinas Pendidikan", Status: "1"},
	}

	// Dry-run tidak menulis.
	if plan, err := RepairUnitIdentity(ctx, pool, officers, false); err != nil {
		t.Fatalf("dry-run: %v", err)
	} else if plan.Planned == 0 {
		t.Fatal("dry-run harus melaporkan rencana perubahan")
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM units WHERE kd_unker IS NOT NULL`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("dry-run menulis %d baris, seharusnya 0", n)
	}

	plan, err := RepairUnitIdentity(ctx, pool, officers, true)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if plan.Inserted != 1 {
		t.Fatalf("Inserted = %d, want 1 (SDN 1 Karanganyar Karangrayung yang hilang)", plan.Inserted)
	}

	// SDN 1 Karanganyar GEYER harus bertahan sebagai unit yang sama + kd_unker.
	var gCode, gDistrict string
	if err := pool.QueryRow(ctx, `SELECT code, COALESCE(district,'') FROM units WHERE id=$1`, sdKaranganyarGeyer).Scan(&gCode, &gDistrict); err != nil {
		t.Fatal(err)
	}
	if gCode != "SDN-1-KARANGANYAR-03-09-27" || gDistrict != "GEYER" {
		t.Fatalf("unit GEYER = %q/%q, want SDN-1-KARANGANYAR-03-09-27/GEYER", gCode, gDistrict)
	}
	// SDN 1 Karanganyar Karangrayung harus ada sebagai unit baru.
	var kCode string
	if err := pool.QueryRow(ctx, `SELECT code FROM units WHERE kd_unker='03.21.02'`).Scan(&kCode); err != nil {
		t.Fatalf("unit Karangrayung tidak tersisip: %v", err)
	}
	if kCode != "SDN-1-KARANGANYAR-03-21-02" {
		t.Fatalf("code Karangrayung = %q", kCode)
	}
	// Guru tetap tertaut ke unit lama (id tidak berubah) dengan kode baru.
	var teacherUnitCode string
	if err := pool.QueryRow(ctx, `SELECT u.code FROM teachers t JOIN units u ON u.id=t.unit_id WHERE t.id=$1`, teacherID).Scan(&teacherUnitCode); err != nil {
		t.Fatal(err)
	}
	if teacherUnitCode != "SDN-1-KARANGANYAR-03-09-27" {
		t.Fatalf("guru kini di unit %q, want SDN-1-KARANGANYAR-03-09-27 (tautan id harus utuh)", teacherUnitCode)
	}
	// Tanggungharjo harus dikoreksi ke GROBOGAN (desa di Kec. Grobogan).
	var tDistrict string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(district,'') FROM units WHERE id=$1`, sdTanggeng).Scan(&tDistrict); err != nil {
		t.Fatal(err)
	}
	if tDistrict != "GROBOGAN" {
		t.Fatalf("district Tanggungharjo = %q, want GROBOGAN (dari kode 03.10)", tDistrict)
	}

	// Idempoten: jalan ulang tidak mengubah apa pun.
	second, err := RepairUnitIdentity(ctx, pool, officers, true)
	if err != nil {
		t.Fatalf("ulang: %v", err)
	}
	if second.Planned != 0 || second.Inserted != 0 || second.Reassigned != 0 || second.Districted != 0 {
		t.Fatalf("jalan ulang tidak idempoten: %+v", second)
	}
}
