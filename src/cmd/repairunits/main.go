// Command repairunits memperbaiki identitas unit kerja (units) dari kode unit
// SIPP ASN (kd_unker) secara idempoten. Dipakai sekali saat rilis temuan
// 2026-09-14 (10 sekolah SD hilang + kecamatan salah), aman dijalankan ulang.
//
// Pemakaian:
//
//	repairunits            # dry-run: hitung rencana, tanpa menulis
//	repairunits -execute   # terapkan perbaikan
//
// Env: DATABASE_URL, MIGRATIONS_DIR (opsional), SIPPASN_BASE_URL (opsional).
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"sicendikia/internal/store"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	execute := flag.Bool("execute", false, "terapkan perbaikan (default: dry-run)")
	fromFile := flag.String("from", "", "berkas JSON /api/pegawai tersimpan (mode offline; default unduh dari SIPP ASN)")
	flag.Parse()

	logger := slog.Default()
	dbURL := envOr("DATABASE_URL", "dbname=si_cendikia sslmode=disable")
	migrationsDir := envOr("MIGRATIONS_DIR", "migrations")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := store.Open(ctx, dbURL)
	if err != nil {
		logger.Error("koneksi database gagal", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if n, err := store.Migrate(ctx, pool, migrationsDir); err != nil {
		logger.Error("migrasi gagal", "err", err)
		os.Exit(1)
	} else if n > 0 {
		logger.Info("migrasi diterapkan", "count", n)
	}

	officers, err := loadOfficers(ctx, *fromFile)
	if err != nil {
		logger.Error("ambil data SIPP ASN gagal", "err", err)
		os.Exit(1)
	}
	logger.Info("data SIPP ASN diterima", "rows", len(officers), "mode", map[bool]string{true: "execute", false: "dry-run"}[*execute])

	plan, err := store.RepairUnitIdentity(ctx, pool, officers, *execute)
	if err != nil {
		logger.Error("perbaikan identitas unit gagal", "err", err)
		os.Exit(1)
	}
	logger.Info("selesai", "execute", *execute, "summary", store.FormatUnitRepairPlan(plan))
}

func loadOfficers(ctx context.Context, fromFile string) ([]store.SIPPASNOfficer, error) {
	if fromFile == "" {
		client := store.SIPPASNClient{
			BaseURL: envOr("SIPPASN_BASE_URL", "https://sippasn.grobogan.go.id"),
			HTTP:    &http.Client{Timeout: 90 * time.Second},
		}
		return client.FetchOfficers(ctx)
	}
	raw, err := os.ReadFile(fromFile)
	if err != nil {
		return nil, err
	}
	return store.ParseSIPPASNOfficers(raw)
}
