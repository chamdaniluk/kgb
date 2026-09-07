package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"sicendikia/internal/auth"
	"sicendikia/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunNightlySIPPASNSync menjalankan sinkron SIPPASN berulang tiap interval
// (produksi: 24 jam). Setiap jalan: refresh snapshot, upsert seluruh baris
// layak (SIPPASN menang untuk data induk), catat ringkasan ke bkn_imports +
// audit. Gagal satu jalan hanya dicatat, jadwal berikutnya tetap jalan.
// Berhenti saat ctx dibatalkan; mengembalikan ctx.Err().
func RunNightlySIPPASNSync(ctx context.Context, pool *pgxpool.Pool, snapshot *store.SIPPASNSnapshot, interval time.Duration, actorID int64) error {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			runSIPPASNSyncOnce(ctx, pool, snapshot, actorID)
		}
	}
}

func runSIPPASNSyncOnce(ctx context.Context, pool *pgxpool.Pool, snapshot *store.SIPPASNSnapshot, actorID int64) {
	runCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := snapshot.Ensure(runCtx); err != nil {
		slog.Warn("sinkron SIPPASN malam: snapshot gagal", "err", err)
		return
	}
	officers := snapshot.Officers()
	cfg := store.SIPPASNSyncConfig{HanyaUnitPendidikan: true, HanyaStatusAktif: true}
	result, err := store.ImportSIPPASNOfficers(runCtx, pool, actorID, officers, cfg, auth.HashPassword)
	if err != nil {
		slog.Warn("sinkron SIPPASN malam: impor gagal", "err", err)
		return
	}
	details, _ := json.Marshal(map[string]any{
		"sumber": "sippasn", "jadwal": "malam",
		"rows_total": result.RowsTotal, "rows_created": result.RowsCreated,
		"rows_updated": result.RowsUpdated, "rows_skipped": result.RowsSkipped,
	})
	_ = store.TouchAudit(runCtx, pool, actorID, "sinkron_sippasn_malam", details, "scheduler")
	slog.Info("sinkron SIPPASN malam selesai",
		"total", result.RowsTotal, "baru", result.RowsCreated,
		"diperbarui", result.RowsUpdated, "dilewati", result.RowsSkipped)
}
