package store

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IssueContext menyimpan data penerbitan yang sudah dicadangkan singkat di DB.
type IssueContext struct {
	Submission Submission
	Template   LetterNumberTemplate
	Number     string
	LockToken  string
}

// BeginIssue mencadangkan nomor dan token penerbitan dalam transaksi singkat.
// Rendering dan panggilan eSign dilakukan setelah transaksi ini selesai.
func BeginIssue(ctx context.Context, pool *pgxpool.Pool, submissionID int64) (IssueContext, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return IssueContext{}, err
	}
	fail := func(err error) (IssueContext, error) {
		_ = tx.Rollback(ctx)
		return IssueContext{}, err
	}
	row := tx.QueryRow(ctx, submissionSelect+` WHERE s.id=$1 FOR UPDATE OF s`, submissionID)
	submission, err := scanSubmission(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrNotFound)
	}
	if err != nil {
		return fail(err)
	}
	if submission.Status != "menunggu_tte" {
		return fail(ErrConflict)
	}
	var activeToken *string
	var activeExpiry *time.Time
	if err := tx.QueryRow(ctx, `SELECT tte_lock_token, tte_lock_expires_at FROM submissions WHERE id=$1`, submissionID).Scan(&activeToken, &activeExpiry); err != nil {
		return fail(err)
	}
	if activeToken != nil && activeExpiry != nil && activeExpiry.After(time.Now()) {
		return fail(ErrConflict)
	}
	var template LetterNumberTemplate
	err = tx.QueryRow(ctx, `SELECT id, pattern, is_active, created_at, updated_at FROM letter_number_templates WHERE is_active=true LIMIT 1`).Scan(&template.ID, &template.Pattern, &template.IsActive, &template.CreatedAt, &template.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(errors.New("template nomor surat aktif belum tersedia"))
	}
	if err != nil {
		return fail(err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('si-cendikia-letter-number'))`); err != nil {
		return fail(err)
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `
		SELECT count(*) +
		       (SELECT count(*) FROM submissions WHERE status='menunggu_tte' AND tte_number IS NOT NULL) + 1
		FROM letters
		WHERE issued_at >= date_trunc('year', now())
		  AND issued_at < date_trunc('year', now()) + interval '1 year'`).Scan(&sequence); err != nil {
		return fail(err)
	}
	number := strings.ReplaceAll(template.Pattern, "{SEQ}", fmt.Sprintf("%03d", sequence))
	number = strings.ReplaceAll(number, "{YEAR}", time.Now().Format("2006"))
	var tokenBytes [16]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		return fail(err)
	}
	lockToken := fmt.Sprintf("%x", tokenBytes[:])
	tag, err := tx.Exec(ctx, `
		UPDATE submissions SET tte_lock_token=$1, tte_lock_expires_at=now()+interval '10 minutes', tte_number=$2, updated_at=now()
		WHERE id=$3 AND status='menunggu_tte' AND (tte_lock_token IS NULL OR tte_lock_expires_at <= now())`, lockToken, number, submissionID)
	if err != nil {
		return fail(err)
	}
	if tag.RowsAffected() != 1 {
		return fail(ErrConflict)
	}
	if err := tx.Commit(ctx); err != nil {
		return IssueContext{}, err
	}
	return IssueContext{Submission: submission, Template: template, Number: number, LockToken: lockToken}, nil
}

// ReleaseIssue membatalkan reservation jika rendering/TTE gagal.
func ReleaseIssue(ctx context.Context, pool *pgxpool.Pool, submissionID int64, lockToken string) error {
	_, err := pool.Exec(ctx, `UPDATE submissions SET tte_lock_token=NULL, tte_lock_expires_at=NULL, updated_at=now() WHERE id=$1 AND status='menunggu_tte' AND tte_lock_token=$2`, submissionID, lockToken)
	return err
}

// CommitIssue menyimpan surat final, mengubah status, master guru, dan audit.
func CommitIssue(ctx context.Context, pool *pgxpool.Pool, issue IssueContext, signerID int64, receiptID, finalPath, ip string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO letters (submission_id, template_id, number, pdf_path, signer_user_id, tte_receipt_id)
		VALUES ($1,$2,$3,$4,$5,$6)`, issue.Submission.ID, issue.Template.ID, issue.Number, finalPath, signerID, receiptID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE submissions SET status='terbit', rejection_note=NULL, tte_lock_token=NULL, tte_lock_expires_at=NULL, updated_at=now()
		WHERE id=$1 AND status='menunggu_tte' AND tte_lock_token=$2 AND tte_lock_expires_at > now()`, issue.Submission.ID, issue.LockToken)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	masaKerja := issue.Submission.MasaKerjaTahun
	if issue.Submission.ProposedMasaKerjaTahun != nil {
		masaKerja = *issue.Submission.ProposedMasaKerjaTahun
	}
	if err := UpdateTeacherAfterIssue(ctx, tx, issue.Submission.TeacherID, issue.Submission.ProposedTMT, masaKerja); err != nil {
		return err
	}
	change := TeacherChange{
		PangkatGol: issue.Submission.ProposedPangkatGol,
		Pangkat:    issue.Submission.ProposedPangkat,
		Jabatan:    issue.Submission.ProposedJabatan,
		UnitID:     issue.Submission.ProposedUnitID,
	}
	if issue.Submission.HasTeacherChange() {
		if err := ApplyTeacherChangeAtIssue(ctx, tx, issue.Submission.TeacherID, change); err != nil {
			return err
		}
	}
	details, err := json.Marshal(map[string]string{"number": issue.Number, "tte_receipt_id": receiptID})
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_user_id, submission_id, action, details, ip) VALUES ($1,$2,'tte',$3,$4)`, signerID, issue.Submission.ID, details, ip); err != nil {
		return err
	}
	if issue.Submission.HasTeacherChange() {
		changeDetails, _ := json.Marshal(map[string]any{
			"pangkat_gol_baru": issue.Submission.ProposedPangkatGol,
			"pangkat_baru":     issue.Submission.ProposedPangkat,
			"jabatan_baru":     issue.Submission.ProposedJabatan,
			"unit_baru_id":     issue.Submission.ProposedUnitID,
			"unit_baru":        issue.Submission.ProposedUnitName,
			"tanggal_berlaku":  issue.Submission.ProposedEffectiveDate,
			"catatan":          issue.Submission.ProposedChangeNote,
		})
		if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_user_id, submission_id, action, details, ip) VALUES ($1,$2,'perubahan_data_terapan',$3,$4)`, signerID, issue.Submission.ID, changeDetails, ip); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_user_id, submission_id, action, details, ip) VALUES ($1,$2,'terbit',$3,$4)`, signerID, issue.Submission.ID, details, ip); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// GetLetter mengambil surat berdasarkan ID.
func GetLetter(ctx context.Context, pool *pgxpool.Pool, id int64) (LetterSummary, string, int64, error) {
	var l LetterSummary
	var path string
	var submissionID int64
	err := pool.QueryRow(ctx, `SELECT id, number, issued_at, COALESCE(tte_receipt_id,''), pdf_path, submission_id FROM letters WHERE id=$1`, id).Scan(&l.ID, &l.Number, &l.IssuedAt, &l.TTEReceiptID, &path, &submissionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return LetterSummary{}, "", 0, ErrNotFound
	}
	return l, path, submissionID, err
}

// GetLetterForSubmission mengambil surat sebuah pengajuan.
func GetLetterForSubmission(ctx context.Context, pool *pgxpool.Pool, submissionID int64) (LetterSummary, string, error) {
	var l LetterSummary
	var path string
	err := pool.QueryRow(ctx, `SELECT id, number, issued_at, COALESCE(tte_receipt_id,''), pdf_path FROM letters WHERE submission_id=$1`, submissionID).Scan(&l.ID, &l.Number, &l.IssuedAt, &l.TTEReceiptID, &path)
	if errors.Is(err, pgx.ErrNoRows) {
		return LetterSummary{}, "", ErrNotFound
	}
	return l, path, err
}

// ListLetters mengambil rekap surat terbit.
func ListLetters(ctx context.Context, pool *pgxpool.Pool, year int) ([]LetterSummary, error) {
	if year < 2000 || year > 2200 {
		year = time.Now().Year()
	}
	rows, err := pool.Query(ctx, `SELECT id, number, issued_at, COALESCE(tte_receipt_id,'') FROM letters WHERE issued_at >= make_date($1,1,1) AND issued_at < make_date($1+1,1,1) ORDER BY issued_at DESC`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]LetterSummary, 0)
	for rows.Next() {
		var l LetterSummary
		if err := rows.Scan(&l.ID, &l.Number, &l.IssuedAt, &l.TTEReceiptID); err != nil {
			return nil, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}
