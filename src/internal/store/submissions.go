package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const submissionSelect = `
SELECT s.id, s.teacher_id, s.status, s.proposed_tmt,
       s.proposed_masa_kerja_tahun, s.proposed_tmt_kgb_last,
       COALESCE(s.current_salary::text, ''), COALESCE(s.next_salary::text, ''),
       COALESCE(s.file_name, ''), COALESCE(s.file_path, ''), COALESCE(s.file_size, 0),
       COALESCE(s.rejection_note, ''), s.submitted_at, s.created_at, s.updated_at,
       t.name, t.nip, t.asn_type, t.pangkat_gol, t.masa_kerja_tahun,
       t.unit_id, un.name,
       l.id, l.number, l.issued_at, l.tte_receipt_id
FROM submissions s
JOIN teachers t ON t.id = s.teacher_id
JOIN units un ON un.id = t.unit_id
LEFT JOIN letters l ON l.submission_id = s.id`

func scanSubmission(row pgx.Row) (Submission, error) {
	var s Submission
	var letterID *int64
	var number *string
	var issuedAt *time.Time
	var receipt *string
	err := row.Scan(
		&s.ID, &s.TeacherID, &s.Status, &s.ProposedTMT,
		&s.ProposedMasaKerjaTahun, &s.ProposedTMTKGBLast,
		&s.CurrentSalary, &s.NextSalary, &s.FileName, &s.FilePath, &s.FileSize,
		&s.RejectionNote, &s.SubmittedAt, &s.CreatedAt, &s.UpdatedAt,
		&s.TeacherName, &s.NIP, &s.ASNType, &s.PangkatGol, &s.MasaKerjaTahun,
		&s.UnitID, &s.UnitName,
		&letterID, &number, &issuedAt, &receipt,
	)
	if err != nil {
		return Submission{}, err
	}
	if letterID != nil {
		s.Letter = &LetterSummary{ID: *letterID, Number: derefString(number), IssuedAt: derefTime(issuedAt), TTEReceiptID: derefString(receipt)}
	}
	return s, nil
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefTime(v *time.Time) time.Time {
	if v == nil {
		return time.Time{}
	}
	return *v
}

// GetSubmission mengambil detail pengajuan beserta snapshot guru dan surat.
func GetSubmission(ctx context.Context, pool *pgxpool.Pool, id int64) (Submission, error) {
	s, err := scanSubmission(pool.QueryRow(ctx, submissionSelect+` WHERE s.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Submission{}, ErrNotFound
	}
	return s, err
}

// ListSubmissionsForTeacher mengambil riwayat pengajuan satu guru.
func ListSubmissionsForTeacher(ctx context.Context, pool *pgxpool.Pool, teacherID int64) ([]Submission, error) {
	rows, err := pool.Query(ctx, submissionSelect+` WHERE s.teacher_id = $1 ORDER BY s.created_at DESC`, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Submission, 0)
	for rows.Next() {
		s, err := scanSubmission(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ListQueue mengambil antrean status tertentu. Unit hanya boleh melihat unitnya.
func ListQueue(ctx context.Context, pool *pgxpool.Pool, role string, unitID *int64, status string) ([]Submission, error) {
	query := submissionSelect + ` WHERE s.status = $1`
	args := []any{status}
	if role == "verifikator_unit" {
		if unitID == nil {
			return nil, ErrForbidden
		}
		query += ` AND (t.unit_id = $2 OR EXISTS (SELECT 1 FROM units scope WHERE scope.id=$2 AND scope.type='korwil' AND t.unit_id IN (SELECT id FROM units child WHERE child.parent_id=scope.id)))`
		args = append(args, *unitID)
	}
	query += ` ORDER BY s.submitted_at ASC NULLS LAST, s.id ASC`
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Submission, 0)
	for rows.Next() {
		s, err := scanSubmission(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// CreateSubmission menyimpan pengajuan dan audit submit dalam satu transaksi.
func CreateSubmission(ctx context.Context, pool *pgxpool.Pool, teacherID, actorID int64, proposedTMT time.Time, proposedMasaKerja *int, proposedTMTKGBLast *time.Time, currentSalary, nextSalary, fileName, filePath string, fileSize int64, ip string) (Submission, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Submission{}, err
	}
	defer tx.Rollback(ctx)
	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO submissions (teacher_id, status, proposed_tmt, proposed_masa_kerja_tahun, proposed_tmt_kgb_last, current_salary, next_salary, file_name, file_path, file_size, submitted_at)
		VALUES ($1, 'menunggu_unit', $2, $3, $4, $5, $6, $7, $8, $9, now()) RETURNING id`,
		teacherID, proposedTMT, proposedMasaKerja, proposedTMTKGBLast, currentSalary, nextSalary, fileName, filePath, fileSize).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Submission{}, ErrConflict
		}
		return Submission{}, fmt.Errorf("insert pengajuan: %w", err)
	}
	details, _ := json.Marshal(map[string]any{"status": "menunggu_unit", "file_name": fileName, "file_size": fileSize})
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_user_id, submission_id, action, details, ip) VALUES ($1, $2, 'submit', $3, $4)`, actorID, id, details, ip); err != nil {
		return Submission{}, fmt.Errorf("audit submit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Submission{}, err
	}
	return GetSubmission(ctx, pool, id)
}

// Resubmit mengubah pengajuan dikembalikan sesuai jenjang penolakan.
func Resubmit(ctx context.Context, pool *pgxpool.Pool, id, actorID int64, proposedTMT time.Time, proposedMasaKerja *int, proposedTMTKGBLast *time.Time, currentSalary, nextSalary, fileName, filePath string, fileSize int64, ip string) (Submission, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Submission{}, err
	}
	defer tx.Rollback(ctx)
	var oldStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM submissions WHERE id = $1 AND teacher_id = (SELECT id FROM teachers WHERE user_id = $2) FOR UPDATE`, id, actorID).Scan(&oldStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Submission{}, ErrForbidden
		}
		return Submission{}, err
	}
	var newStatus string
	switch oldStatus {
	case "dikembalikan_unit":
		newStatus = "menunggu_unit"
	case "dikembalikan_dinas":
		newStatus = "menunggu_dinas"
	default:
		return Submission{}, ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE submissions SET status=$1, proposed_tmt=$2, proposed_masa_kerja_tahun=$3, proposed_tmt_kgb_last=$4, current_salary=$5, next_salary=$6, file_name=$7, file_path=$8, file_size=$9, rejection_note=NULL, submitted_at=now(), updated_at=now() WHERE id=$10`, newStatus, proposedTMT, proposedMasaKerja, proposedTMTKGBLast, currentSalary, nextSalary, fileName, filePath, fileSize, id); err != nil {
		return Submission{}, err
	}
	details, _ := json.Marshal(map[string]any{"from": oldStatus, "to": newStatus, "file_name": fileName, "file_size": fileSize})
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_user_id, submission_id, action, details, ip) VALUES ($1, $2, 'resubmit', $3, $4)`, actorID, id, details, ip); err != nil {
		return Submission{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Submission{}, err
	}
	return GetSubmission(ctx, pool, id)
}

// Transition mengubah status secara compare-and-swap dan mencatat audit.
func Transition(ctx context.Context, pool *pgxpool.Pool, id, actorID int64, expected, next, action, note, ip string) (Submission, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Submission{}, err
	}
	defer tx.Rollback(ctx)
	var current string
	if err := tx.QueryRow(ctx, `SELECT status FROM submissions WHERE id=$1 FOR UPDATE`, id).Scan(&current); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Submission{}, ErrNotFound
		}
		return Submission{}, err
	}
	if current != expected {
		return Submission{}, ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE submissions SET status=$1, rejection_note=$2, updated_at=now() WHERE id=$3`, next, nullIfEmpty(note), id); err != nil {
		return Submission{}, err
	}
	details, _ := json.Marshal(map[string]any{"from": expected, "to": next, "note": note})
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_user_id, submission_id, action, details, ip) VALUES ($1, $2, $3, $4, $5)`, actorID, id, action, details, ip); err != nil {
		return Submission{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Submission{}, err
	}
	return GetSubmission(ctx, pool, id)
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// AuditForSubmission mengambil timeline audit untuk detail pengajuan.
func AuditForSubmission(ctx context.Context, pool *pgxpool.Pool, submissionID int64) ([]AuditLog, error) {
	rows, err := pool.Query(ctx, `
		SELECT a.id, a.actor_user_id, COALESCE(u.name,''), a.submission_id, a.action,
		       a.details, COALESCE(a.ip,''), a.created_at
		FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_user_id
		WHERE a.submission_id=$1 ORDER BY a.created_at ASC, a.id ASC`, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]AuditLog, 0)
	for rows.Next() {
		var a AuditLog
		var raw []byte
		if err := rows.Scan(&a.ID, &a.ActorUserID, &a.ActorName, &a.SubmissionID, &a.Action, &raw, &a.IP, &a.CreatedAt); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &a.Details)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
