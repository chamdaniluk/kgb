package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Kelompok akun di dasbor admin utama (keputusan owner 2026-09-04):
// Admin (admin, admin_dinas), Guru (asn), Unit (verifikator_unit),
// Dinas (verifikator_dinas, pimpinan).
const (
	RoleGroupAdmin = "admin"
	RoleGroupGuru  = "guru"
	RoleGroupUnit  = "unit"
	RoleGroupDinas = "dinas"
)

// RoleGroupRoles memetakan kelompok ke daftar role database.
// Kelompok kosong/"semua" berarti tanpa filter role.
func RoleGroupRoles(group string) ([]string, bool) {
	switch strings.ToLower(strings.TrimSpace(group)) {
	case "", "semua":
		return nil, true
	case RoleGroupAdmin:
		return []string{"admin", StaffRoleAdminDinas}, true
	case RoleGroupGuru:
		return []string{"asn"}, true
	case RoleGroupUnit:
		return []string{StaffRoleVerifikatorUnit}, true
	case RoleGroupDinas:
		return []string{"verifikator_dinas", StaffRolePimpinan}, true
	}
	return nil, false
}

// RoleGroupOf mengelompokkan satu role untuk badge UI.
func RoleGroupOf(role string) string {
	switch role {
	case "admin", StaffRoleAdminDinas:
		return RoleGroupAdmin
	case "asn":
		return RoleGroupGuru
	case StaffRoleVerifikatorUnit:
		return RoleGroupUnit
	case "verifikator_dinas", StaffRolePimpinan:
		return RoleGroupDinas
	}
	return ""
}

// UnitFilter adalah filter wilayah bersama untuk daftar users dan teachers.
type UnitFilter struct {
	UnitID    int64
	UnitType  string
	Kecamatan string
}

// unitWhere membangun potongan " AND ..." untuk filter wilayah.
// unitCol adalah kolom unit pemilik baris (mis. "u.unit_id"); JOIN ke "un"
// (unit) dan "par" (parent korwil) wajib ada di query pemanggil.
func unitWhere(f UnitFilter, unitCol string, start int) (string, []any) {
	var b strings.Builder
	var args []any
	idx := start
	if f.UnitID > 0 {
		fmt.Fprintf(&b, " AND %s = $%d", unitCol, idx)
		args = append(args, f.UnitID)
		idx++
	}
	if t := strings.ToLower(strings.TrimSpace(f.UnitType)); t != "" {
		fmt.Fprintf(&b, " AND un.type = $%d", idx)
		args = append(args, t)
		idx++
	}
	if kec := strings.ToUpper(strings.TrimSpace(f.Kecamatan)); kec != "" {
		if kec == "DINAS" {
			b.WriteString(" AND un.type = 'dinas'")
		} else {
			fmt.Fprintf(&b, " AND (un.name = $%d OR par.name = $%d)", idx, idx+1)
			args = append(args, "KORWILCAM "+kec, "KORWILCAM "+kec)
			idx += 2
		}
	}
	return b.String(), args
}

const unitJoins = `LEFT JOIN units un ON un.id = UNITCOL LEFT JOIN units par ON par.id = un.parent_id`

func joinsFor(unitCol string) string {
	return strings.ReplaceAll(unitJoins, "UNITCOL", unitCol)
}

// ListUsersFiltered mengambil akun admin dengan filter kelompok + wilayah.
func ListUsersFiltered(ctx context.Context, pool *pgxpool.Pool, q string, roles []string, f UnitFilter, page Page) ([]UserSummary, int64, error) {
	pattern := "%" + q + "%"
	args := []any{q, pattern}
	whereRoles := ""
	if len(roles) > 0 {
		placeholders := make([]string, len(roles))
		for i, r := range roles {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
			args = append(args, r)
		}
		whereRoles = " AND u.role IN (" + strings.Join(placeholders, ",") + ")"
	}
	extra, extraArgs := unitWhere(f, "u.unit_id", len(args)+1)
	args = append(args, extraArgs...)
	base := `FROM users u ` + joinsFor("u.unit_id") + `
WHERE ($1='' OR u.username ILIKE $2 OR u.name ILIKE $2)` + whereRoles + extra
	var total int64
	if err := pool.QueryRow(ctx, `SELECT count(*) `+base, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `
		SELECT u.id, u.username, u.role, u.name, u.unit_id, un.name, COALESCE(un.type,''), u.is_active, u.last_login_at ` + base + `
		ORDER BY u.name, u.username`
	lim, limArgs := page.clause(len(args) + 1)
	query += lim
	args = append(args, limArgs...)
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := make([]UserSummary, 0)
	for rows.Next() {
		var u UserSummary
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.Name, &u.UnitID, &u.UnitName, &u.UnitType, &u.IsActive, &u.LastLogin); err != nil {
			return nil, 0, err
		}
		u.RoleGroup = RoleGroupOf(u.Role)
		result = append(result, u)
	}
	return result, total, rows.Err()
}

// ListTeachersFiltered mengambil master ASN dengan filter wilayah.
func ListTeachersFiltered(ctx context.Context, pool *pgxpool.Pool, q string, f UnitFilter, limit, offset int) ([]Teacher, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	pattern := "%" + q + "%"
	args := []any{q, pattern}
	extra, extraArgs := unitWhere(f, "t.unit_id", len(args)+1)
	args = append(args, extraArgs...)
	base := `FROM teachers t JOIN units un ON un.id = t.unit_id LEFT JOIN units par ON par.id = un.parent_id
WHERE ($1='' OR t.nip ILIKE $2 OR t.name ILIKE $2)` + extra
	var total int64
	if err := pool.QueryRow(ctx, `SELECT count(*) `+base, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `
		SELECT ` + teacherCols + ` ` + base + `
		ORDER BY t.name ASC LIMIT $` + itoa(len(args)+1) + ` OFFSET $` + itoa(len(args)+2)
	rows, err := pool.Query(ctx, query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := make([]Teacher, 0)
	for rows.Next() {
		var t Teacher
		var cols = teacherScanDests(&t)
		if err := rows.Scan(cols...); err != nil {
			return nil, 0, err
		}
		result = append(result, t)
	}
	return result, total, rows.Err()
}

// AuditFilter adalah filter tambahan log aktivitas untuk admin utama.
type AuditFilter struct {
	Actor string
	From  string
	To    string
}

// ListAuditLogsFiltered mengambil audit dengan filter aksi/pelaksana/tanggal.
func ListAuditLogsFiltered(ctx context.Context, pool *pgxpool.Pool, submissionID *int64, action string, f AuditFilter, page Page) ([]AuditLog, int64, error) {
	var sid any
	if submissionID != nil {
		sid = *submissionID
	}
	args := []any{sid, action, "%" + f.Actor + "%", f.From, f.To}
	base := `FROM audit_logs a LEFT JOIN users u ON u.id = a.actor_user_id
WHERE ($1::bigint IS NULL OR a.submission_id = $1)
AND ($2 = '' OR a.action = $2)
AND ($3 = '%%' OR u.username ILIKE $3 OR u.name ILIKE $3)
AND ($4 = '' OR a.created_at >= ($4 || ' 00:00:00+07')::timestamptz)
AND ($5 = '' OR a.created_at < (($5 || ' 00:00:00+07')::timestamptz + interval '1 day'))`
	var total int64
	if err := pool.QueryRow(ctx, `SELECT count(*) `+base, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `
		SELECT a.id, a.actor_user_id, COALESCE(u.name,''), a.submission_id, a.action, a.details, COALESCE(a.ip,''), a.created_at ` + base + `
		ORDER BY a.created_at DESC, a.id DESC`
	lim, limArgs := page.clause(len(args) + 1)
	query += lim
	args = append(args, limArgs...)
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := make([]AuditLog, 0)
	for rows.Next() {
		var a AuditLog
		var raw []byte
		if err := rows.Scan(&a.ID, &a.ActorUserID, &a.ActorName, &a.SubmissionID, &a.Action, &raw, &a.IP, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &a.Details)
		}
		result = append(result, a)
	}
	return result, total, rows.Err()
}

// ListAuditActions mengembalikan daftar aksi yang pernah tercatat (untuk dropdown).
func ListAuditActions(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT DISTINCT action FROM audit_logs ORDER BY action`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// teacherScanDests mengembalikan tujuan scan teacherCols sesuai urutan
// scanTeacher, agar kolom tambahan (tipe unit) dapat ditambahkan.
func teacherScanDests(t *Teacher) []any {
	return []any{&t.ID, &t.UserID, &t.NIP, &t.Name, &t.ASNType, &t.Kategori,
		&t.UnitID, &t.UnitName, &t.UnitType, &t.UnitDistrict, &t.PangkatGol, &t.Pangkat, &t.Jabatan, &t.BirthDate, &t.BirthPlace, &t.Karpeg,
		&t.LastSKPejabat, &t.LastSKTanggal, &t.LastSKNomor, &t.LastSKTMTBerlaku, &t.LastSKMasaKerjaTahun, &t.LastSKMasaKerjaBulan,
		&t.LastKPGolongan, &t.LastKPTMT, &t.LastKPNomor,
		&t.LastKPMasaTahun, &t.LastKPMasaBulan, &t.LastSKMasaTahun, &t.LastSKMasaBulan,
		&t.MasaKerjaTahun, &t.MasaKerjaSource, &t.TMTKGBLast, &t.TMTAwal}
}

// KecamatanOptions mengembalikan 19 kecamatan Grobogan + DINAS untuk dropdown.
func KecamatanOptions() []string {
	return []string{"BRATI", "GABUS", "GEYER", "GODONG", "GROBOGAN", "GUBUG",
		"KARANGRAYUNG", "KEDUNGJATI", "KLAMBU", "KRADENAN", "NGARINGAN",
		"PENAWANGAN", "PULOKULON", "PURWODADI", "TANGGUNGHARJO", "TAWANGHARJO",
		"TEGOWANU", "TOROH", "WIROSARI", "DINAS"}
}
