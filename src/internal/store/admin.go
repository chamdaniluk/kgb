package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ListUsers mengambil akun untuk admin tanpa password hash.
func ListUsers(ctx context.Context, pool *pgxpool.Pool, q string, page Page) ([]UserSummary, int64, error) {
	pattern := "%" + q + "%"
	var total int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users u WHERE ($1='' OR u.username ILIKE $2 OR u.name ILIKE $2)`, q, pattern).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `
		SELECT u.id, u.username, u.role, u.name, u.unit_id, un.name, u.is_active, u.last_login_at
		FROM users u LEFT JOIN units un ON un.id=u.unit_id
		WHERE ($1='' OR u.username ILIKE $2 OR u.name ILIKE $2)
		ORDER BY u.name, u.username`
	args := []any{q, pattern}
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
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.Name, &u.UnitID, &u.UnitName, &u.IsActive, &u.LastLogin); err != nil {
			return nil, 0, err
		}
		result = append(result, u)
	}
	return result, total, rows.Err()
}

// CreateStaffUser membuat akun petugas dan menolak username yang sama dengan NIP.
func CreateStaffUser(ctx context.Context, pool *pgxpool.Pool, username, passwordHash, role, name string, unitID *int64, nik, signature, employeeNumber, jobTitle string) (UserSummary, error) {
	if role == "asn" {
		return UserSummary{}, errors.New("akun ASN dibuat melalui impor BKN")
	}
	if IsNIPUsername(username) {
		return UserSummary{}, errors.New("username petugas tidak boleh berupa NIP")
	}
	if !IsStaffRole(role) {
		return UserSummary{}, errors.New("role petugas tidak valid")
	}
	if role == StaffRoleVerifikatorUnit && unitID == nil {
		return UserSummary{}, errors.New("verifikator unit wajib memiliki unit kerja")
	}
	if name == "" {
		name = username
	}
	active := true
	if role == StaffRolePimpinan && (nik == "" || signature == "") {
		active = false
	}
	var nipExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM teachers WHERE nip=$1)`, username).Scan(&nipExists); err != nil {
		return UserSummary{}, err
	}
	if nipExists {
		return UserSummary{}, errors.New("username petugas tidak boleh sama dengan NIP")
	}
	var u UserSummary
	err := pool.QueryRow(ctx, `
		INSERT INTO users (username,password_hash,role,name,unit_id,nik,signature_image_base64,employee_number,job_title,is_active)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10)
		RETURNING id, username, role, name, unit_id, (SELECT name FROM units WHERE id=users.unit_id), is_active, last_login_at`, username, passwordHash, role, name, unitID, nik, signature, employeeNumber, jobTitle, active).
		Scan(&u.ID, &u.Username, &u.Role, &u.Name, &u.UnitID, &u.UnitName, &u.IsActive, &u.LastLogin)
	if IsUniqueViolation(err) {
		return UserSummary{}, ErrConflict
	}
	return u, err
}

// IsStaffRole reports whether role is allowed for a non-ASN account.
func IsStaffRole(role string) bool {
	return role == StaffRoleVerifikatorUnit || role == "verifikator_dinas" || role == StaffRoleAdminDinas || role == StaffRolePimpinan || role == "admin"
}

func derefOptional(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// UpdateStaffUser memperbarui data akun petugas; nil berarti tidak mengubah field.
func UpdateStaffUser(ctx context.Context, pool *pgxpool.Pool, id int64, name *string, role *string, unitID **int64, active *bool, passwordHash *string, nik *string, signature *string, employeeNumber *string, jobTitle *string) (UserSummary, error) {
	if role != nil && !IsStaffRole(*role) {
		return UserSummary{}, errors.New("role petugas tidak valid")
	}
	var currentRole string
	var currentNIK, currentSignature *string
	var currentActive bool
	if err := pool.QueryRow(ctx, `SELECT role, nik, signature_image_base64, is_active FROM users WHERE id=$1 AND role <> 'asn'`, id).Scan(&currentRole, &currentNIK, &currentSignature, &currentActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserSummary{}, ErrNotFound
		}
		return UserSummary{}, err
	}
	targetRole := currentRole
	if role != nil {
		targetRole = *role
	}
	targetNIK := derefOptional(currentNIK)
	if nik != nil {
		targetNIK = strings.TrimSpace(*nik)
	}
	targetSignature := derefOptional(currentSignature)
	if signature != nil {
		targetSignature = strings.TrimSpace(*signature)
	}
	wantsActive := currentActive
	if active != nil {
		wantsActive = *active
	}
	if targetRole == StaffRolePimpinan && wantsActive && (targetNIK == "" || targetSignature == "") {
		return UserSummary{}, errors.New("akun pimpinan harus memiliki NIK dan spesimen TTD sebelum diaktifkan")
	}
	var u UserSummary
	var passwordChanged any
	if passwordHash != nil {
		passwordChanged = *passwordHash
	}
	var roleValue any
	if role != nil {
		roleValue = *role
	}
	var nameValue any
	if name != nil {
		nameValue = *name
	}
	var activeValue any
	if active != nil {
		activeValue = *active
	}
	var nikValue any
	if nik != nil {
		nikValue = nullIfEmpty(*nik)
	}
	var signatureValue any
	if signature != nil {
		signatureValue = nullIfEmpty(*signature)
	}
	var employeeNumberValue any
	if employeeNumber != nil {
		employeeNumberValue = nullIfEmpty(*employeeNumber)
	}
	var jobTitleValue any
	if jobTitle != nil {
		jobTitleValue = nullIfEmpty(*jobTitle)
	}
	var unitValue any
	if unitID != nil {
		unitValue = *unitID
	}
	err := pool.QueryRow(ctx, `
		UPDATE users SET
			name=COALESCE($1,name), role=COALESCE($2,role), unit_id=CASE WHEN $3::boolean THEN $4::int ELSE unit_id END,
			is_active=COALESCE($5,is_active), password_hash=COALESCE($6,password_hash), nik=CASE WHEN $7::boolean THEN $8::text ELSE nik END,
			signature_image_base64=CASE WHEN $9::boolean THEN $10::text ELSE signature_image_base64 END,
			employee_number=CASE WHEN $11::boolean THEN $12::text ELSE employee_number END,
			job_title=CASE WHEN $13::boolean THEN $14::text ELSE job_title END, updated_at=now()
		WHERE id=$15 AND role <> 'asn'
		RETURNING id, username, role, name, unit_id, (SELECT name FROM units WHERE id=users.unit_id), is_active, last_login_at`,
		nameValue, roleValue, unitID != nil, unitValue, activeValue, passwordChanged, nik != nil, nikValue, signature != nil, signatureValue,
		employeeNumber != nil, employeeNumberValue, jobTitle != nil, jobTitleValue, id).
		Scan(&u.ID, &u.Username, &u.Role, &u.Name, &u.UnitID, &u.UnitName, &u.IsActive, &u.LastLogin)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserSummary{}, ErrNotFound
	}
	return u, err
}

// ListSalaryScales mengambil skala gaji dengan filter sederhana + pagination.
func ListSalaryScales(ctx context.Context, pool *pgxpool.Pool, asnType, golongan string, page Page) ([]SalaryScale, int64, error) {
	var total int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM salary_scales WHERE ($1='' OR asn_type=$1) AND ($2='' OR golongan=$2)`, asnType, golongan).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT id, asn_type, golongan, masa_kerja_tahun, gaji::text FROM salary_scales WHERE ($1='' OR asn_type=$1) AND ($2='' OR golongan=$2) ORDER BY asn_type, golongan, masa_kerja_tahun`
	args := []any{asnType, golongan}
	lim, limArgs := page.clause(len(args) + 1)
	query += lim
	args = append(args, limArgs...)
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := make([]SalaryScale, 0)
	for rows.Next() {
		var s SalaryScale
		if err := rows.Scan(&s.ID, &s.ASNType, &s.Golongan, &s.MasaKerjaTahun, &s.Gaji); err != nil {
			return nil, 0, err
		}
		result = append(result, s)
	}
	return result, total, rows.Err()
}

// UpsertSalaryScales memasukkan skala baru dengan kunci resmi.
func UpsertSalaryScales(ctx context.Context, pool *pgxpool.Pool, scales []SalaryScale) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, s := range scales {
		if s.ASNType != "pns" && s.ASNType != "pppk" || s.Golongan == "" || s.MasaKerjaTahun < 0 || s.Gaji == "" {
			return fmt.Errorf("baris skala tidak valid: %+v", s)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO salary_scales (asn_type,golongan,masa_kerja_tahun,gaji) VALUES ($1,$2,$3,$4) ON CONFLICT (asn_type,golongan,masa_kerja_tahun) DO UPDATE SET gaji=EXCLUDED.gaji`, s.ASNType, s.Golongan, s.MasaKerjaTahun, s.Gaji); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ListTemplates mengambil pola nomor surat.
func ListTemplates(ctx context.Context, pool *pgxpool.Pool) ([]LetterNumberTemplate, error) {
	rows, err := pool.Query(ctx, `SELECT id, pattern, is_active, created_at, updated_at FROM letter_number_templates ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]LetterNumberTemplate, 0)
	for rows.Next() {
		var t LetterNumberTemplate
		if err := rows.Scan(&t.ID, &t.Pattern, &t.IsActive, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// CreateTemplate menonaktifkan pola lama dan mengaktifkan pola baru.
func CreateTemplate(ctx context.Context, pool *pgxpool.Pool, pattern string) (LetterNumberTemplate, error) {
	if !strings.Contains(pattern, "{SEQ}") || !strings.Contains(pattern, "{YEAR}") {
		return LetterNumberTemplate{}, errors.New("template wajib memuat {SEQ} dan {YEAR}")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return LetterNumberTemplate{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE letter_number_templates SET is_active=false, updated_at=now()`); err != nil {
		return LetterNumberTemplate{}, err
	}
	var t LetterNumberTemplate
	if err := tx.QueryRow(ctx, `INSERT INTO letter_number_templates (pattern,is_active) VALUES ($1,true) RETURNING id,pattern,is_active,created_at,updated_at`, pattern).Scan(&t.ID, &t.Pattern, &t.IsActive, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return LetterNumberTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return LetterNumberTemplate{}, err
	}
	return t, nil
}

// ListAuditLogs mengambil audit dengan filter admin + pagination.
func ListAuditLogs(ctx context.Context, pool *pgxpool.Pool, submissionID *int64, action string, page Page) ([]AuditLog, int64, error) {
	var sid any
	if submissionID != nil {
		sid = *submissionID
	}
	var total int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs a WHERE ($1::bigint IS NULL OR a.submission_id=$1) AND ($2='' OR a.action=$2)`, sid, action).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `
		SELECT a.id,a.actor_user_id,COALESCE(u.name,''),a.submission_id,a.action,a.details,COALESCE(a.ip,''),a.created_at
		FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_user_id
		WHERE ($1::bigint IS NULL OR a.submission_id=$1) AND ($2='' OR a.action=$2)
		ORDER BY a.created_at DESC,a.id DESC`
	args := []any{sid, action}
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

// TouchAudit stores no secrets; helper for admin mutations.
func TouchAudit(ctx context.Context, pool *pgxpool.Pool, actorID int64, action string, details []byte, ip string) error {
	_, err := pool.Exec(ctx, `INSERT INTO audit_logs (actor_user_id,action,details,ip) VALUES ($1,$2,$3,$4)`, actorID, action, details, ip)
	return err
}
