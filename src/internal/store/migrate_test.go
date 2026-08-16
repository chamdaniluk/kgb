package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Dev lokal: unix socket + peer auth (tanpa password); bisa dioverride via env.
const testDBURL = "dbname=si_cendikia_test sslmode=disable"

// testPool membuka pool ke database uji; test di-skip bila DB tidak tersedia.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("SI_CENDIKIA_TEST_DB")
	if url == "" {
		url = testDBURL
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := Open(ctx, url)
	if err != nil {
		t.Skipf("database uji tidak tersedia: %v", err)
	}
	lockConn, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		t.Skipf("koneksi lock database uji tidak tersedia: %v", err)
	}
	if _, err := lockConn.Exec(ctx, `SELECT pg_advisory_lock(hashtext('si-cendikia-test-suite'))`); err != nil {
		lockConn.Release()
		pool.Close()
		t.Skipf("lock database uji tidak tersedia: %v", err)
	}
	t.Cleanup(func() {
		_, _ = lockConn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext('si-cendikia-test-suite'))`)
		lockConn.Release()
		pool.Close()
	})
	return pool
}

func resetSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatalf("reset skema gagal: %v", err)
	}
}

var semuaTabel = []string{
	"units", "users", "teachers", "submissions",
	"letter_number_templates", "letters", "audit_logs",
	"bkn_imports", "salary_scales",
}

func TestMigrateMembuat9Tabel(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()

	n, err := Migrate(ctx, pool, "../../migrations")
	if err != nil {
		t.Fatalf("Migrate gagal: %v", err)
	}
	if n != 3 {
		t.Errorf("berkas migrasi terapkan = %d, ingin 3", n)
	}
	for _, tbl := range append([]string{"schema_migrations"}, semuaTabel...) {
		var ada bool
		if err := pool.QueryRow(ctx,
			`SELECT to_regclass('public.' || $1) IS NOT NULL`, tbl,
		).Scan(&ada); err != nil || !ada {
			t.Errorf("tabel %s tidak ada (err=%v)", tbl, err)
		}
	}
}

func TestMigrateIdempoten(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()

	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("Migrate #1 gagal: %v", err)
	}
	n, err := Migrate(ctx, pool, "../../migrations")
	if err != nil {
		t.Fatalf("Migrate #2 gagal: %v", err)
	}
	if n != 0 {
		t.Errorf("Migrate #2 menerapkan %d berkas, ingin 0 (harus idempoten)", n)
	}
}

// Seed skala gaji: file hasil generator dari seeder e-KGB (PP 5/2024 & Perpres 11/2024).
// Spot-check memakai angka yang juga dipakai test e-KGB (III/b MKG 12 = 3.497.300).
func TestSeedSalaryScales(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()

	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("migrasi gagal: %v", err)
	}
	seed, err := os.ReadFile(filepath.Join("..", "..", "data", "seeds", "001_salary_scales.sql"))
	if err != nil {
		t.Fatalf("baca seed: %v", err)
	}
	if _, err := pool.Exec(ctx, string(seed)); err != nil {
		t.Fatalf("jalankan seed gagal: %v", err)
	}

	var total int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM salary_scales`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total < 500 {
		t.Errorf("total baris skala gaji = %d, ingin >= 500", total)
	}

	spot := []struct {
		asn, gol string
		mkg      int
		want     string
	}{
		{"pns", "III/b", 12, "3497300"},
		{"pns", "I/a", 0, "1685700"},
		{"pns", "IV/e", 32, "6373200"},
		{"pppk", "IX", 0, "3203600"},
		{"pppk", "IX", 24, "4647700"},
	}
	for _, s := range spot {
		var got string
		if err := pool.QueryRow(ctx,
			`SELECT gaji FROM salary_scales WHERE asn_type=$1 AND golongan=$2 AND masa_kerja_tahun=$3`,
			s.asn, s.gol, s.mkg,
		).Scan(&got); err != nil {
			t.Errorf("lookup %s %s MKG %d gagal: %v", s.asn, s.gol, s.mkg, err)
			continue
		}
		if got != s.want {
			t.Errorf("gaji %s %s MKG %d = %s, ingin %s", s.asn, s.gol, s.mkg, got, s.want)
		}
	}

	// Jalankan seed dua kali: harus tetap sama (idempoten).
	if _, err := pool.Exec(ctx, string(seed)); err != nil {
		t.Fatalf("seed ulang gagal: %v", err)
	}
	var total2 int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM salary_scales`).Scan(&total2); err != nil {
		t.Fatal(err)
	}
	if total2 != total {
		t.Errorf("seed ulang mengubah jumlah baris: %d -> %d", total, total2)
	}
}
