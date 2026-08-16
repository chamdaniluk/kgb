package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditEntry: satu baris audit_logs (append-only; tidak ada API update/delete).
type AuditEntry struct {
	ActorUserID  *int64
	SubmissionID *int64
	Action       string
	Details      []byte // JSON, boleh nil
	IP           string
}

func InsertAudit(ctx context.Context, pool *pgxpool.Pool, e AuditEntry) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_user_id, submission_id, action, details, ip)
		VALUES ($1, $2, $3, $4, $5)`,
		e.ActorUserID, e.SubmissionID, e.Action, e.Details, e.IP)
	return err
}

// CountAudit: berapa baris audit dengan action tertentu (untuk test).
func CountAudit(ctx context.Context, pool *pgxpool.Pool, action string) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = $1`, action).Scan(&n)
	return n, err
}
