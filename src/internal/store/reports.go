package store

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sicendikia/internal/letterdata"
)

// IssuedFilter memfilter riwayat surat terbit.
type IssuedFilter struct {
	Query  string
	Year   int
	UnitID int64
}

// NominationFilter memfilter daftar nominasi KGB.
type NominationFilter struct {
	Query  string
	Bucket string
}

// Nomination adalah ASN yang dijadwalkan KGB berikutnya.
type Nomination struct {
	Teacher
	NextTMT          time.Time `json:"next_tmt"`
	Bucket           string    `json:"bucket"`
	SudahDiusulkan   bool      `json:"sudah_diusulkan"`
	StatusUsulan     string    `json:"status_usulan,omitempty"`
	NomorSuratTerbit string    `json:"nomor_surat_terbit,omitempty"`
}

func staffCanSeeAll(role string) bool {
	switch role {
	case "admin", "admin_dinas", "pimpinan", "verifikator_dinas":
		return true
	default:
		return false
	}
}

func scopeSQL(role string, actorUnitID *int64, alias string, startArg int) (string, []any) {
	if staffCanSeeAll(role) || actorUnitID == nil {
		return "", nil
	}
	sql := ` AND (` + alias + ` = $` + itoa(startArg) + ` OR EXISTS (
		SELECT 1 FROM units scope JOIN units child ON child.parent_id=scope.id
		WHERE scope.id=$` + itoa(startArg) + ` AND scope.type='korwil'
		  AND child.id=` + alias + ` AND child.type IN ('sd','tk')))`
	return sql, []any{*actorUnitID}
}

func itoa(v int) string { return strconv.Itoa(v) }

// ListIssuedSubmissions mengambil pengajuan yang sudah terbit dalam scope aktor.
// Mengembalikan potongan halaman beserta total baris.
func ListIssuedSubmissions(ctx context.Context, pool *pgxpool.Pool, role string, actorUnitID *int64, f IssuedFilter, page Page) ([]Submission, int64, error) {
	where := ` WHERE s.status='terbit'`
	args := []any{}
	n := 1
	if f.Year >= 2000 && f.Year <= 2200 {
		where += ` AND l.issued_at >= make_date($` + itoa(n) + `,1,1) AND l.issued_at < make_date($` + itoa(n) + `+1,1,1)`
		args = append(args, f.Year)
		n++
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		where += ` AND (s.snapshot_name ILIKE $` + itoa(n) + ` OR s.snapshot_nip ILIKE $` + itoa(n) + ` OR l.number ILIKE $` + itoa(n) + ` OR un.name ILIKE $` + itoa(n) + `)`
		args = append(args, "%"+q+"%")
		n++
	}
	if f.UnitID > 0 {
		where += ` AND COALESCE(s.proposed_unit_id, s.snapshot_unit_id, t.unit_id) = $` + itoa(n)
		args = append(args, f.UnitID)
		n++
	}
	scope, scopeArgs := scopeSQL(role, actorUnitID, "COALESCE(s.proposed_unit_id, s.snapshot_unit_id, t.unit_id)", n)
	where += scope
	args = append(args, scopeArgs...)
	// total pakai FROM lengkap agar alias t/un/l tersedia bagi filter.
	countFrom := `FROM submissions s JOIN teachers t ON t.id=s.teacher_id JOIN units un ON un.id=t.unit_id LEFT JOIN units pu ON pu.id=s.proposed_unit_id LEFT JOIN letters l ON l.submission_id=s.id`
	var total int64
	if err := pool.QueryRow(ctx, `SELECT count(*) `+countFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := submissionSelect + where + ` ORDER BY l.issued_at DESC NULLS LAST, s.id DESC`
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

// ListNominations mengambil ASN dalam scope beserta jadwal KGB berikutnya.
// Karena filter bucket dilakukan in-memory, total dihitung setelah filter lalu
// hasil dipotong sesuai halaman. Mengembalikan (halaman, total, err).
func ListNominations(ctx context.Context, pool *pgxpool.Pool, role string, actorUnitID *int64, f NominationFilter, asOf time.Time, page Page) ([]Nomination, int64, error) {
	query := `SELECT ` + teacherCols + ` FROM teachers t JOIN units un ON un.id=t.unit_id WHERE 1=1`
	args := []any{}
	n := 1
	if q := strings.TrimSpace(f.Query); q != "" {
		query += ` AND (t.nip ILIKE $` + itoa(n) + ` OR t.name ILIKE $` + itoa(n) + ` OR un.name ILIKE $` + itoa(n) + `)`
		args = append(args, "%"+q+"%")
		n++
	}
	scope, scopeArgs := scopeSQL(role, actorUnitID, "t.unit_id", n)
	query += scope
	args = append(args, scopeArgs...)
	query += ` ORDER BY t.name ASC`
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	teachers := make([]Teacher, 0)
	for rows.Next() {
		t, err := scanTeacher(rows)
		if err != nil {
			return nil, 0, err
		}
		teachers = append(teachers, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	type usulan struct {
		Status string
		Number string
	}
	pending := map[int64]usulan{}
	prows, err := pool.Query(ctx, `
		SELECT s.teacher_id, s.status, COALESCE(l.number,'')
		FROM submissions s
		LEFT JOIN letters l ON l.submission_id=s.id
		WHERE s.status <> 'terbit' OR l.issued_at >= now() - interval '2 years'`)
	if err != nil {
		return nil, 0, err
	}
	defer prows.Close()
	for prows.Next() {
		var id int64
		var u usulan
		if err := prows.Scan(&id, &u.Status, &u.Number); err != nil {
			return nil, 0, err
		}
		// aktif lebih diprioritaskan daripada terbit lama
		if cur, ok := pending[id]; ok && cur.Status != "terbit" && u.Status == "terbit" {
			continue
		}
		pending[id] = u
	}
	if err := prows.Err(); err != nil {
		return nil, 0, err
	}

	want := strings.TrimSpace(f.Bucket)
	result := make([]Nomination, 0)
	for _, t := range teachers {
		last := t.TMTKGBLast
		if last == nil {
			last = t.LastSKTMTBerlaku
		}
		n := Nomination{Teacher: t}
		if t.TMTAwal == nil {
			// Tanpa TMT awal tidak ada jangkar grid dua tahunan → belum lengkap.
			n.Bucket = letterdata.BucketBelumLengkap
		} else {
			// Rumus tunggal bersama alur usulan (letterdata.NextDueTMT):
			// anniversary setelah SK terakhir; telat tidak menggeser siklus.
			prior := *t.TMTAwal
			if last != nil && last.After(prior) {
				prior = *last
			}
			n.NextTMT = letterdata.NextDueTMT(*t.TMTAwal, prior, asOf)
			n.Bucket = letterdata.NominationBucket(n.NextTMT, asOf)
		}
		if u, ok := pending[t.ID]; ok {
			n.SudahDiusulkan = true
			n.StatusUsulan = u.Status
			n.NomorSuratTerbit = u.Number
		}
		if want != "" && want != "semua" && n.Bucket != want {
			continue
		}
		result = append(result, n)
	}
	total := int64(len(result))
	lo, hi := page.slice(len(result))
	return result[lo:hi], total, nil
}
