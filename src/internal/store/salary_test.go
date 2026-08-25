package store

import (
	"context"
	"testing"
)

func TestSalaryCurrentNextUsesLowerBracketForOddMasaKerja(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO salary_scales (asn_type,golongan,masa_kerja_tahun,gaji)
VALUES ('pns','III/b',0,3000000),('pns','III/b',2,3100000),('pns','III/b',4,3200000),('pns','III/b',6,3300000),('pns','III/b',8,3400000)`); err != nil {
		t.Fatal(err)
	}
	current, next, err := SalaryCurrentNext(ctx, pool, "pns", "III/b", 5)
	if err != nil {
		t.Fatal(err)
	}
	if current != "3200000" || next != "3300000" {
		t.Fatalf("salary 5 tahun = %s/%s, want 3200000/3300000", current, next)
	}
}

func TestSalaryCurrentNextRejectsBelowFirstBracket(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO salary_scales (asn_type,golongan,masa_kerja_tahun,gaji) VALUES ('pns','III/b',2,3100000),('pns','III/b',4,3200000)`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SalaryCurrentNext(ctx, pool, "pns", "III/b", 1); err != ErrScaleNotFound {
		t.Fatalf("salary below first bracket err=%v, want ErrScaleNotFound", err)
	}
}

func TestSalaryCurrentNextCapsAtTopBracket(t *testing.T) {
	// Masa kerja 20 th pada skala yang berhenti di bracket 8: gaji mengikuti
	// nilai tertinggi tabel — current dan next sama (puncak skala).
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO salary_scales (asn_type,golongan,masa_kerja_tahun,gaji)
VALUES ('pns','III/b',0,3000000),('pns','III/b',2,3100000),('pns','III/b',4,3200000),('pns','III/b',6,3300000),('pns','III/b',8,3400000)`); err != nil {
		t.Fatal(err)
	}
	current, next, err := SalaryCurrentNext(ctx, pool, "pns", "III/b", 20)
	if err != nil {
		t.Fatal(err)
	}
	if current != "3400000" || next != "3400000" {
		t.Fatalf("salary 20 tahun = %s/%s, want 3400000/3400000 (cap)", current, next)
	}
}

var _ = context.Background
