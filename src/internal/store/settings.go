package store

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Kunci pengaturan banner uji coba internal di beranda (keputusan 2026-09-04).
const (
	SettingTrialNoticeEnabled = "trial_notice_enabled"
	SettingTrialNoticeUntil   = "trial_notice_until"
)

// GetSetting mengambil nilai pengaturan; ok=false bila kunci belum pernah disimpan.
func GetSetting(ctx context.Context, pool *pgxpool.Pool, key string) (value string, ok bool, err error) {
	err = pool.QueryRow(ctx, `SELECT value FROM app_settings WHERE key = $1`, key).Scan(&value)
	if err == pgx.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// SetSetting menyimpan pengaturan (upsert) beserta pencatat pelaksana.
func SetSetting(ctx context.Context, pool *pgxpool.Pool, key, value string, actorID int64) error {
	var actor any
	if actorID > 0 {
		actor = actorID
	}
	_, err := pool.Exec(ctx, `INSERT INTO app_settings (key, value, updated_by, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()`,
		key, value, actor)
	return err
}

// ParseTrialUntil memvalidasi isian tanggal admin: YYYY-MM-DD atau RFC3339
// penuh; string kosong berarti kembali ke env/default.
func ParseTrialUntil(raw string) (time.Time, bool, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return time.Time{}, false, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, true, nil
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local), true, nil
	}
	return time.Time{}, false, errInvalidTrialUntil
}

var errInvalidTrialUntil = errTrialUntil()

type trialUntilError struct{ msg string }

func (e trialUntilError) Error() string { return e.msg }

func errTrialUntil() error { return trialUntilError{"format tanggal tidak valid (gunakan YYYY-MM-DD)"} }
