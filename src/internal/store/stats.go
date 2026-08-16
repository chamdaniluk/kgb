package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// GetPublicStats menghasilkan agregat tanpa nama/NIP/PII.
func GetPublicStats(ctx context.Context, pool *pgxpool.Pool) (PublicStats, error) {
	var out PublicStats
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM teachers`).Scan(&out.TeachersTotal); err != nil {
		return out, err
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM submissions WHERE status <> 'terbit'`).Scan(&out.SubmissionsActive); err != nil {
		return out, err
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM letters`).Scan(&out.LettersIssued); err != nil {
		return out, err
	}
	out.PerStatus = make(map[string]int64)
	rows, err := pool.Query(ctx, `SELECT status,count(*) FROM submissions GROUP BY status ORDER BY status`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			return out, err
		}
		out.PerStatus[status] = count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	rows, err = pool.Query(ctx, `SELECT to_char(date_trunc('month',created_at),'YYYY-MM'),count(*) FROM submissions WHERE created_at >= date_trunc('month',now()) - interval '11 months' GROUP BY 1 ORDER BY 1`)
	if err != nil {
		return out, err
	}
	out.SubmissionsPerMonth = make([]MonthCount, 0)
	for rows.Next() {
		var m MonthCount
		if err := rows.Scan(&m.Month, &m.Count); err != nil {
			rows.Close()
			return out, err
		}
		out.SubmissionsPerMonth = append(out.SubmissionsPerMonth, m)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	rows, err = pool.Query(ctx, `SELECT un.name,count(*) FROM submissions s JOIN teachers t ON t.id=s.teacher_id JOIN units un ON un.id=t.unit_id GROUP BY un.id,un.name ORDER BY count(*) DESC,un.name`)
	if err != nil {
		return out, err
	}
	out.PerUnit = make([]UnitCount, 0)
	for rows.Next() {
		var u UnitCount
		if err := rows.Scan(&u.Unit, &u.Count); err != nil {
			rows.Close()
			return out, err
		}
		out.PerUnit = append(out.PerUnit, u)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	return out, nil
}
