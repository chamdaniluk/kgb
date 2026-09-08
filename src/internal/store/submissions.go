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

	"sicendikia/internal/letterdata"
)

const submissionSelect = `
SELECT s.id, s.teacher_id, s.status, s.proposed_tmt,
       s.proposed_masa_kerja_tahun, s.proposed_tmt_kgb_last,
       s.proposed_pangkat_gol, s.proposed_pangkat, s.proposed_jabatan,
       s.proposed_unit_id, COALESCE(pu.name, ''), s.proposed_effective_date,
       COALESCE(s.proposed_change_note, ''),
       COALESCE(s.current_salary::text, ''), COALESCE(s.next_salary::text, ''),
       COALESCE(s.file_name, ''), COALESCE(s.file_path, ''), COALESCE(s.file_size, 0),
       COALESCE(s.file_kp_name, ''), COALESCE(s.file_kp_path, ''), COALESCE(s.file_kp_size, 0),
       COALESCE(s.file_kgb_name, ''), COALESCE(s.file_kgb_path, ''), COALESCE(s.file_kgb_size, 0),
       COALESCE(s.file_skp_name, ''), COALESCE(s.file_skp_path, ''), COALESCE(s.file_skp_size, 0),
       COALESCE(s.rejection_note, ''), s.submitted_at, s.created_at, s.updated_at,
       COALESCE(s.snapshot_name, t.name), COALESCE(s.snapshot_nip, t.nip),
       COALESCE(s.snapshot_asn_type, t.asn_type), COALESCE(s.snapshot_pangkat_gol, t.pangkat_gol),
       COALESCE(s.snapshot_pangkat, COALESCE(t.pangkat, '')), COALESCE(s.snapshot_jabatan, COALESCE(t.jabatan, '')),
       COALESCE(s.snapshot_masa_kerja_tahun, t.masa_kerja_tahun), COALESCE(s.snapshot_unit_id, t.unit_id),
       COALESCE(s.snapshot_unit_name, un.name),
       COALESCE(vu.type, un.type, ''), COALESCE(vu.district, un.district, ''),
       COALESCE(korwil.name, ''),
       COALESCE(s.snapshot_birth_place, COALESCE(t.birth_place,'')), s.snapshot_birth_date, COALESCE(s.snapshot_karpeg, COALESCE(t.karpeg,'')),
       COALESCE(s.snapshot_last_sk_pejabat, COALESCE(t.last_sk_pejabat,'')), s.snapshot_last_sk_tanggal, COALESCE(s.snapshot_last_sk_nomor, COALESCE(t.last_sk_nomor,'')),
       s.snapshot_last_sk_tmt_berlaku, s.snapshot_last_sk_masa_kerja_tahun, s.snapshot_last_sk_masa_kerja_bulan,
       COALESCE(s.draft_birth_place, ''), s.draft_birth_date, COALESCE(s.draft_karpeg, ''),
       COALESCE(s.draft_pangkat, ''), COALESCE(s.draft_jabatan, ''),
       COALESCE(s.draft_last_sk_pejabat, ''), s.draft_last_sk_tanggal, COALESCE(s.draft_last_sk_nomor, ''),
       s.draft_last_sk_tmt, s.draft_last_sk_masa_tahun, s.draft_last_sk_masa_bulan,
       COALESCE(s.draft_last_kp_golongan, ''), s.draft_last_kp_tmt, COALESCE(s.draft_last_kp_nomor, ''),
       s.draft_last_kp_tanggal, COALESCE(s.draft_last_kp_pejabat, ''),
       s.draft_last_kp_masa_tahun, s.draft_last_kp_masa_bulan,
       s.draft_mkg_lama_tahun, s.draft_mkg_lama_bulan,
       s.draft_mkg_baru_tahun, s.draft_mkg_baru_bulan,
       COALESCE(s.draft_masa_perjanjian, ''), s.draft_perpanjangan_kontrak,
       s.tmt_awal,
       l.id, l.number, l.issued_at, l.tte_receipt_id
FROM submissions s
JOIN teachers t ON t.id = s.teacher_id
JOIN units un ON un.id = t.unit_id
LEFT JOIN units pu ON pu.id = s.proposed_unit_id
LEFT JOIN units vu ON vu.id = COALESCE(s.proposed_unit_id, s.snapshot_unit_id, t.unit_id)
LEFT JOIN units korwil ON korwil.id = vu.parent_id AND korwil.type = 'korwil'
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
		&s.ProposedPangkatGol, &s.ProposedPangkat, &s.ProposedJabatan,
		&s.ProposedUnitID, &s.ProposedUnitName, &s.ProposedEffectiveDate, &s.ProposedChangeNote,
		&s.CurrentSalary, &s.NextSalary, &s.FileName, &s.FilePath, &s.FileSize,
		&s.FileKPName, &s.FileKPPath, &s.FileKPSize,
		&s.FileKGBName, &s.FileKGBPath, &s.FileKGBSize,
		&s.FileSKPName, &s.FileSKPPath, &s.FileSKPSize,
		&s.RejectionNote, &s.SubmittedAt, &s.CreatedAt, &s.UpdatedAt,
		&s.TeacherName, &s.NIP, &s.ASNType, &s.PangkatGol, &s.Pangkat, &s.Jabatan, &s.MasaKerjaTahun,
		&s.UnitID, &s.UnitName, &s.UnitType, &s.UnitDistrict, &s.UnitKorwilName, &s.SnapshotBirthPlace, &s.SnapshotBirthDate, &s.SnapshotKarpeg, &s.SnapshotLastSKPejabat, &s.SnapshotLastSKTanggal, &s.SnapshotLastSKNomor, &s.SnapshotLastSKTMTBerlaku, &s.SnapshotLastSKMasaTahun, &s.SnapshotLastSKMasaBulan,
		&s.DraftBirthPlace, &s.DraftBirthDate, &s.DraftKarpeg, &s.DraftPangkat, &s.DraftJabatan,
		&s.DraftLastSKPejabat, &s.DraftLastSKTanggal, &s.DraftLastSKNomor, &s.DraftLastSKTMT,
		&s.DraftLastSKMasaTahun, &s.DraftLastSKMasaBulan,
		&s.DraftLastKPGolongan, &s.DraftLastKPTMT, &s.DraftLastKPNomor, &s.DraftLastKPTanggal, &s.DraftLastKPPejabat,
		&s.DraftLastKPMasaTahun, &s.DraftLastKPMasaBulan,
		&s.DraftMKGLamaTahun, &s.DraftMKGLamaBulan, &s.DraftMKGBaruTahun, &s.DraftMKGBaruBulan,
		&s.DraftMasaPerjanjian, &s.DraftPerpanjangan,
		&s.TMTAwal,
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

// ListQueue mengambil antrean status tertentu dengan cakupan jenjang:
// akun Korwil melihat usulannya sendiri (pegawai Korwil diverifikasi sub
// di Korwil-nya) + TK/SD sekecamatan (relasi parent atau kecamatan sama);
// akun SMP/SKB melihat unitnya sendiri. Unit Dinas tidak punya antrean unit
// karena usulannya langsung berstatus menunggu_dinas saat dibuat.
// Mengembalikan potongan halaman beserta total baris (untuk pagination).
func ListQueue(ctx context.Context, pool *pgxpool.Pool, role string, unitID *int64, status string, page Page) ([]Submission, int64, error) {
	where := ` WHERE s.status = $1`
	args := []any{status}
	verificationUnit := "COALESCE(s.proposed_unit_id, s.snapshot_unit_id, t.unit_id)"
	if role == "verifikator_unit" {
		if unitID == nil {
			return nil, 0, ErrForbidden
		}
		where += ` AND EXISTS (
			SELECT 1 FROM units actor
			LEFT JOIN units cand ON cand.id = ` + verificationUnit + `
			LEFT JOIN units candpar ON candpar.id = cand.parent_id
			WHERE actor.id = $2 AND (
				-- SMP/SKB: unitnya sendiri
				(actor.type IN ('smp','skb') AND cand.id = actor.id)
				-- Korwil: unitnya sendiri + TK/SD sekecamatan via parent/kecamatan
				OR (actor.type = 'korwil' AND (
					cand.id = actor.id
					OR (cand.type IN ('sd','tk') AND (
						cand.parent_id = actor.id
						OR (candpar.type = 'korwil' AND candpar.district IS NOT NULL AND candpar.district = actor.district)
						OR (cand.district IS NOT NULL AND cand.district = actor.district)
					))
				))
			))`
		args = append(args, *unitID)
	}
	var total int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM submissions s JOIN teachers t ON t.id=s.teacher_id`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := submissionSelect + where + ` ORDER BY s.submitted_at ASC NULLS LAST, s.id ASC`
	lim, limArgs := page.clause(len(args) + 1)
	query += lim
	args = append(args, limArgs...)
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := make([]Submission, 0)
	for rows.Next() {
		s, err := scanSubmission(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, s)
	}
	return result, total, rows.Err()
}

// CreateSubmission menyimpan pengajuan dan audit submit dalam satu transaksi.
// Status awal mengikuti jenjang unit guru: unit Dinas langsung menunggu_dinas
// (tanpa antrean unit), jenjang lain menunggu_unit untuk verifikasi Korwil/SMP.
func CreateSubmission(ctx context.Context, pool *pgxpool.Pool, teacherID, actorID int64, proposedTMT time.Time, proposedMasaKerja *int, proposedTMTKGBLast, tmtAwal *time.Time, change TeacherChange, draft LetterDraft, currentSalary, nextSalary string, files SubmissionFiles, ip string) (Submission, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Submission{}, err
	}
	defer tx.Rollback(ctx)
	initialStatus := "menunggu_unit"
	var teacherUnitType string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(u.type,'') FROM teachers t JOIN units u ON u.id=t.unit_id WHERE t.id=$1`, teacherID).Scan(&teacherUnitType); err != nil {
		return Submission{}, fmt.Errorf("cek unit guru: %w", err)
	}
	if teacherUnitType == "dinas" {
		initialStatus = "menunggu_dinas"
	}
	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO submissions (teacher_id, status, proposed_tmt, proposed_masa_kerja_tahun, proposed_tmt_kgb_last,
			proposed_pangkat_gol, proposed_pangkat, proposed_jabatan, proposed_unit_id, proposed_effective_date, proposed_change_note,
			current_salary, next_salary, file_name, file_path, file_size,
			file_kp_name, file_kp_path, file_kp_size,
			file_kgb_name, file_kgb_path, file_kgb_size,
			file_skp_name, file_skp_path, file_skp_size, submitted_at,
			draft_birth_place, draft_birth_date, draft_karpeg, draft_pangkat, draft_jabatan,
			draft_last_sk_pejabat, draft_last_sk_tanggal, draft_last_sk_nomor, draft_last_sk_tmt,
			draft_last_sk_masa_tahun, draft_last_sk_masa_bulan,
			draft_last_kp_golongan, draft_last_kp_tmt, draft_last_kp_nomor, draft_last_kp_tanggal, draft_last_kp_pejabat,
			draft_last_kp_masa_tahun, draft_last_kp_masa_bulan,
			draft_mkg_lama_tahun, draft_mkg_lama_bulan, draft_mkg_baru_tahun, draft_mkg_baru_bulan,
			draft_masa_perjanjian, draft_perpanjangan_kontrak, tmt_awal)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23, $24, $25, now(),
			$26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43, $44, $45, $46, $47, $48, $49, $50) RETURNING id`,
		teacherID, initialStatus, proposedTMT, proposedMasaKerja, proposedTMTKGBLast, change.PangkatGol, change.Pangkat, change.Jabatan,
		change.UnitID, change.EffectiveDate, nullIfEmpty(change.Note), currentSalary, nextSalary,
		files.Main.Name, nullIfEmpty(files.Main.Path), files.Main.Size,
		nullIfEmpty(files.KP.Name), nullIfEmpty(files.KP.Path), files.KP.Size,
		nullIfEmpty(files.KGB.Name), nullIfEmpty(files.KGB.Path), files.KGB.Size,
		nullIfEmpty(files.SKP.Name), nullIfEmpty(files.SKP.Path), files.SKP.Size,
		nullIfEmpty(draft.BirthPlace), draft.BirthDate, nullIfEmpty(draft.Karpeg), nullIfEmpty(draft.Pangkat), nullIfEmpty(draft.Jabatan),
		nullIfEmpty(draft.LastSKPejabat), draft.LastSKTanggal, nullIfEmpty(draft.LastSKNomor), draft.LastSKTMT,
		draft.LastSKMasaTahun, draft.LastSKMasaBulan,
		nullIfEmpty(draft.LastKPGolongan), draft.LastKPTMT, nullIfEmpty(draft.LastKPNomor), draft.LastKPTanggal, nullIfEmpty(draft.LastKPPejabat),
		draft.LastKPMasaTahun, draft.LastKPMasaBulan,
		draft.MKGLamaTahun, draft.MKGLamaBulan, draft.MKGBaruTahun, draft.MKGBaruBulan,
		nullIfEmpty(draft.MasaPerjanjian), draft.Perpanjangan, tmtAwal).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Submission{}, ErrConflict
		}
		return Submission{}, fmt.Errorf("insert pengajuan: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE submissions s SET
			snapshot_name=t.name, snapshot_nip=t.nip, snapshot_birth_date=t.birth_date,
			snapshot_asn_type=t.asn_type, snapshot_pangkat_gol=t.pangkat_gol,
			snapshot_pangkat=t.pangkat, snapshot_jabatan=t.jabatan,
			snapshot_masa_kerja_tahun=t.masa_kerja_tahun, snapshot_birth_place=t.birth_place, snapshot_karpeg=t.karpeg,
			snapshot_last_sk_pejabat=t.last_sk_pejabat, snapshot_last_sk_tanggal=t.last_sk_tanggal, snapshot_last_sk_nomor=t.last_sk_nomor,
			snapshot_last_sk_tmt_berlaku=t.last_sk_tmt_berlaku, snapshot_last_sk_masa_kerja_tahun=t.last_sk_masa_kerja_tahun, snapshot_last_sk_masa_kerja_bulan=t.last_sk_masa_kerja_bulan,
			snapshot_unit_id=t.unit_id, snapshot_unit_name=u.name
		FROM teachers t JOIN units u ON u.id=t.unit_id
		WHERE s.id=$1 AND t.id=s.teacher_id`, id); err != nil {
		return Submission{}, fmt.Errorf("snapshot data BKN: %w", err)
	}
	detailsMap := map[string]any{"status": initialStatus, "files": FileNames(files)}
	if len(change.AuditDetails) > 0 {
		detailsMap["perubahan_data"] = change.AuditDetails
	}
	details, _ := json.Marshal(detailsMap)
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_user_id, submission_id, action, details, ip) VALUES ($1, $2, 'submit', $3, $4)`, actorID, id, details, ip); err != nil {
		return Submission{}, fmt.Errorf("audit submit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Submission{}, err
	}
	return GetSubmission(ctx, pool, id)
}

// Resubmit mengubah pengajuan dikembalikan sesuai jenjang penolakan.
func Resubmit(ctx context.Context, pool *pgxpool.Pool, id, actorID int64, proposedTMT time.Time, proposedMasaKerja *int, proposedTMTKGBLast, tmtAwal *time.Time, change TeacherChange, draft LetterDraft, currentSalary, nextSalary string, files SubmissionFiles, ip string) (Submission, error) {
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
	if _, err := tx.Exec(ctx, `UPDATE submissions SET status=$1, proposed_tmt=$2, proposed_masa_kerja_tahun=$3, proposed_tmt_kgb_last=$4,
		proposed_pangkat_gol=$5, proposed_pangkat=$6, proposed_jabatan=$7, proposed_unit_id=$8, proposed_effective_date=$9, proposed_change_note=$10,
		current_salary=$11, next_salary=$12, file_name=$13, file_path=$14, file_size=$15,
		file_kp_name=$16, file_kp_path=$17, file_kp_size=$18,
		file_kgb_name=$19, file_kgb_path=$20, file_kgb_size=$21,
		file_skp_name=$22, file_skp_path=$23, file_skp_size=$24,
		rejection_note=NULL, submitted_at=now(), updated_at=now(),
		draft_birth_place=$25, draft_birth_date=$26, draft_karpeg=$27, draft_pangkat=$28, draft_jabatan=$29,
		draft_last_sk_pejabat=$30, draft_last_sk_tanggal=$31, draft_last_sk_nomor=$32, draft_last_sk_tmt=$33,
		draft_last_sk_masa_tahun=$34, draft_last_sk_masa_bulan=$35,
		draft_last_kp_golongan=$36, draft_last_kp_tmt=$37, draft_last_kp_nomor=$38, draft_last_kp_tanggal=$39, draft_last_kp_pejabat=$40,
		draft_last_kp_masa_tahun=$41, draft_last_kp_masa_bulan=$42,
		draft_mkg_lama_tahun=$43, draft_mkg_lama_bulan=$44, draft_mkg_baru_tahun=$45, draft_mkg_baru_bulan=$46,
		draft_masa_perjanjian=$47, draft_perpanjangan_kontrak=$48, tmt_awal=$50 WHERE id=$49`,
		newStatus, proposedTMT, proposedMasaKerja, proposedTMTKGBLast, change.PangkatGol, change.Pangkat, change.Jabatan, change.UnitID, change.EffectiveDate,
		nullIfEmpty(change.Note), currentSalary, nextSalary,
		files.Main.Name, nullIfEmpty(files.Main.Path), files.Main.Size,
		nullIfEmpty(files.KP.Name), nullIfEmpty(files.KP.Path), files.KP.Size,
		nullIfEmpty(files.KGB.Name), nullIfEmpty(files.KGB.Path), files.KGB.Size,
		nullIfEmpty(files.SKP.Name), nullIfEmpty(files.SKP.Path), files.SKP.Size,
		nullIfEmpty(draft.BirthPlace), draft.BirthDate, nullIfEmpty(draft.Karpeg), nullIfEmpty(draft.Pangkat), nullIfEmpty(draft.Jabatan),
		nullIfEmpty(draft.LastSKPejabat), draft.LastSKTanggal, nullIfEmpty(draft.LastSKNomor), draft.LastSKTMT,
		draft.LastSKMasaTahun, draft.LastSKMasaBulan,
		nullIfEmpty(draft.LastKPGolongan), draft.LastKPTMT, nullIfEmpty(draft.LastKPNomor), draft.LastKPTanggal, nullIfEmpty(draft.LastKPPejabat),
		draft.LastKPMasaTahun, draft.LastKPMasaBulan,
		draft.MKGLamaTahun, draft.MKGLamaBulan, draft.MKGBaruTahun, draft.MKGBaruBulan,
		nullIfEmpty(draft.MasaPerjanjian), draft.Perpanjangan, id, tmtAwal); err != nil {
		return Submission{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE submissions s SET
			snapshot_name=t.name, snapshot_nip=t.nip, snapshot_birth_date=t.birth_date,
			snapshot_asn_type=t.asn_type, snapshot_pangkat_gol=t.pangkat_gol,
			snapshot_pangkat=t.pangkat, snapshot_jabatan=t.jabatan,
			snapshot_masa_kerja_tahun=t.masa_kerja_tahun, snapshot_birth_place=t.birth_place, snapshot_karpeg=t.karpeg,
			snapshot_last_sk_pejabat=t.last_sk_pejabat, snapshot_last_sk_tanggal=t.last_sk_tanggal, snapshot_last_sk_nomor=t.last_sk_nomor,
			snapshot_last_sk_tmt_berlaku=t.last_sk_tmt_berlaku, snapshot_last_sk_masa_kerja_tahun=t.last_sk_masa_kerja_tahun, snapshot_last_sk_masa_kerja_bulan=t.last_sk_masa_kerja_bulan,
			snapshot_unit_id=t.unit_id, snapshot_unit_name=u.name
		FROM teachers t JOIN units u ON u.id=t.unit_id
		WHERE s.id=$1 AND t.id=s.teacher_id`, id); err != nil {
		return Submission{}, fmt.Errorf("snapshot data BKN saat submit ulang: %w", err)
	}
	detailsMap := map[string]any{"from": oldStatus, "to": newStatus, "files": FileNames(files)}
	if len(change.AuditDetails) > 0 {
		detailsMap["perubahan_data"] = change.AuditDetails
	}
	details, _ := json.Marshal(detailsMap)
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_user_id, submission_id, action, details, ip) VALUES ($1, $2, 'resubmit', $3, $4)`, actorID, id, details, ip); err != nil {
		return Submission{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Submission{}, err
	}
	return GetSubmission(ctx, pool, id)
}

// ValidateSubmissionDraft menolak naskah bolong sebelum verifikasi atau TTE.
func ValidateSubmissionDraft(s Submission) error {
	d := s.LetterDraftValues()
	input := letterdata.Draft{
		ASNType:        s.ASNType,
		BirthPlace:     d.BirthPlace,
		Karpeg:         d.Karpeg,
		Pangkat:        d.Pangkat,
		PangkatGol:     s.PangkatGol,
		Jabatan:        d.Jabatan,
		BirthDate:      d.BirthDate,
		LastSKPejabat:  d.LastSKPejabat,
		LastSKNomor:    d.LastSKNomor,
		LastSKTanggal:  d.LastSKTanggal,
		LastSKTMT:      d.LastSKTMT,
		LastSKMasaTahun: d.LastSKMasaTahun,
		LastSKMasaBulan: d.LastSKMasaBulan,
		MKGLamaTahun:   derefInt(d.MKGLamaTahun),
		MKGLamaBulan:   derefInt(d.MKGLamaBulan),
		MKGBaruTahun:   derefInt(d.MKGBaruTahun),
		MKGBaruBulan:   derefInt(d.MKGBaruBulan),
		LastKPGolongan: d.LastKPGolongan,
		LastKPTMT:      d.LastKPTMT,
		LastKPMasaTahun: d.LastKPMasaTahun,
		LastKPMasaBulan: d.LastKPMasaBulan,
		LastKPNomor:    d.LastKPNomor,
		LastKPTanggal:  d.LastKPTanggal,
		LastKPPejabat:  d.LastKPPejabat,
		ProposedTMT:    s.ProposedTMT,
		CurrentSalary:  s.CurrentSalary,
		NextSalary:     s.NextSalary,
		UnitName:       s.UnitName,
		MasaPerjanjian: d.MasaPerjanjian,
		Perpanjangan:   d.Perpanjangan,
	}
	if s.ASNType == "pppk" && letterdata.Normalize(d.MasaPerjanjian) != "" && d.Perpanjangan == nil {
		input.PerpanjanganDash = true
	}
	if err := letterdata.ValidateDraft(input); err != nil {
		return err
	}
	return ValidateSubmissionFiles(s)
}

// ValidateSubmissionFiles memeriksa kelengkapan slot berkas pendukung:
// PNS wajib SK KP + KGB terakhir; PPPK wajib SK terakhir + SKP 2 tahun,
// KGB terakhir opsional. Berkas utama lama dihitung sebagai pelengkap.
func ValidateSubmissionFiles(s Submission) error {
	has := func(path string) bool { return path != "" }
	missing := func(label string) error {
		return fmt.Errorf("%w: berkas %s wajib diunggah", letterdata.ErrDraftIncomplete, label)
	}
	if s.ASNType == "pppk" {
		if !has(s.FileKPPath) && !has(s.FilePath) {
			return missing("SK terakhir")
		}
		if !has(s.FileSKPPath) {
			return missing("SKP 2 tahun")
		}
		return nil
	}
	if !has(s.FileKPPath) && !has(s.FilePath) {
		return missing("SK KP terakhir")
	}
	if !has(s.FileKGBPath) {
		return missing("KGB terakhir")
	}
	return nil
}

// Transition mengubah status secara compare-and-swap dan mencatat audit.
func Transition(ctx context.Context, pool *pgxpool.Pool, id, actorID int64, expected, next, action, note, ip string) (Submission, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Submission{}, err
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, submissionSelect+` WHERE s.id=$1 FOR UPDATE OF s`, id)
	full, err := scanSubmission(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Submission{}, ErrNotFound
	}
	if err != nil {
		return Submission{}, err
	}
	if full.Status != expected {
		return Submission{}, ErrConflict
	}
	if action == "setuju_unit" || action == "setuju_dinas" {
		if err := ValidateSubmissionDraft(full); err != nil {
			return Submission{}, err
		}
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
