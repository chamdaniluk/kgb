package store

import (
	"context"
	"testing"
)

func TestUpsertImportedTeacherDoesNotReplaceExistingPassword(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	var unitID int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type) VALUES ('UNIT-PASS','UNIT PASS','smp') RETURNING id`).Scan(&unitID); err != nil {
		t.Fatal(err)
	}
	var userID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id) VALUES ('199001012020011001','hash-lama','asn','ASN Lama',$1) RETURNING id`, unitID).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO teachers (user_id,nip,name,asn_type,unit_id,pangkat_gol,masa_kerja_tahun,masa_kerja_source) VALUES ($1,'199001012020011001','ASN Lama','pns',$2,'III/b',5,'tmt_cpns')`, userID, unitID); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = UpsertImportedTeacher(ctx, tx, ImportedTeacher{
		NIP: "199001012020011001", Name: "ASN Baru", ASNType: "pns", UnitCode: "UNIT-PASS", UnitName: "UNIT PASS", UnitType: "smp", PangkatGol: "III/c", MasaKerjaTahun: 6, MasaKerjaSource: "tmt_cpns",
	}, func(string) (string, error) { return "hash-baru-yang-tidak-boleh-dipakai", nil })
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var passwordHash, name, gol string
	if err := pool.QueryRow(ctx, `SELECT u.password_hash,u.name,t.pangkat_gol FROM users u JOIN teachers t ON t.user_id=u.id WHERE u.username='199001012020011001'`).Scan(&passwordHash, &name, &gol); err != nil {
		t.Fatal(err)
	}
	if passwordHash != "hash-lama" || name != "ASN Baru" || gol != "III/c" {
		t.Fatalf("re-import menimpa data tidak semestinya: password=%q name=%q gol=%q", passwordHash, name, gol)
	}
}
