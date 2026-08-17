package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Teacher: baris tabel teachers (+ nama unit via join).
type Teacher struct {
	ID                   int64      `json:"id"`
	UserID               *int64     `json:"user_id,omitempty"`
	NIP                  string     `json:"nip"`
	Name                 string     `json:"name"`
	ASNType              string     `json:"asn_type"`
	UnitID               int64      `json:"unit_id"`
	UnitName             string     `json:"unit_name"`
	PangkatGol           string     `json:"pangkat_gol"`
	Pangkat              string     `json:"pangkat,omitempty"`
	Jabatan              string     `json:"jabatan,omitempty"`
	BirthPlace           string     `json:"birth_place,omitempty"`
	Karpeg               string     `json:"karpeg,omitempty"`
	LastSKPejabat        string     `json:"last_sk_pejabat,omitempty"`
	LastSKTanggal        *time.Time `json:"last_sk_tanggal,omitempty"`
	LastSKNomor          string     `json:"last_sk_nomor,omitempty"`
	LastSKTMTBerlaku     *time.Time `json:"last_sk_tmt_berlaku,omitempty"`
	LastSKMasaKerjaTahun *int       `json:"last_sk_masa_kerja_tahun,omitempty"`
	LastSKMasaKerjaBulan *int       `json:"last_sk_masa_kerja_bulan,omitempty"`
	BirthDate            *time.Time `json:"birth_date,omitempty"`
	MasaKerjaTahun       int        `json:"masa_kerja_tahun"`
	MasaKerjaSource      string     `json:"masa_kerja_source,omitempty"`
	TMTKGBLast           *time.Time `json:"tmt_kgb_last,omitempty"`
}

const teacherCols = `t.id, t.user_id, t.nip, t.name, t.asn_type,
       t.unit_id, un.name, t.pangkat_gol, COALESCE(t.pangkat,''), COALESCE(t.jabatan,''), t.birth_date,
       COALESCE(t.birth_place,''), COALESCE(t.karpeg,''),
       COALESCE(t.last_sk_pejabat,''), t.last_sk_tanggal, COALESCE(t.last_sk_nomor,''),
       t.last_sk_tmt_berlaku, t.last_sk_masa_kerja_tahun, t.last_sk_masa_kerja_bulan,
       t.masa_kerja_tahun, COALESCE(t.masa_kerja_source,''), t.tmt_kgb_last`

func scanTeacher(row pgx.Row) (Teacher, error) {
	var t Teacher
	err := row.Scan(&t.ID, &t.UserID, &t.NIP, &t.Name, &t.ASNType,
		&t.UnitID, &t.UnitName, &t.PangkatGol, &t.Pangkat, &t.Jabatan, &t.BirthDate, &t.BirthPlace, &t.Karpeg,
		&t.LastSKPejabat, &t.LastSKTanggal, &t.LastSKNomor, &t.LastSKTMTBerlaku, &t.LastSKMasaKerjaTahun, &t.LastSKMasaKerjaBulan,
		&t.MasaKerjaTahun, &t.MasaKerjaSource, &t.TMTKGBLast)
	return t, err
}

// GetTeacherByUserID mengambil data kepegawaian dari akun ASN.
func GetTeacherByUserID(ctx context.Context, pool *pgxpool.Pool, userID int64) (Teacher, error) {
	t, err := scanTeacher(pool.QueryRow(ctx, `
		SELECT `+teacherCols+`
		FROM teachers t JOIN units un ON un.id = t.unit_id
		WHERE t.user_id = $1`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Teacher{}, ErrNotFound
	}
	return t, err
}

// GetTeacherByID mengambil satu guru.
func GetTeacherByID(ctx context.Context, pool *pgxpool.Pool, id int64) (Teacher, error) {
	t, err := scanTeacher(pool.QueryRow(ctx, `
		SELECT `+teacherCols+`
		FROM teachers t JOIN units un ON un.id = t.unit_id
		WHERE t.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Teacher{}, ErrNotFound
	}
	return t, err
}

// ListTeachers mengambil master ASN untuk admin.
func ListTeachers(ctx context.Context, pool *pgxpool.Pool, q string, limit, offset int) ([]Teacher, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	pattern := "%" + q + "%"
	var total int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM teachers WHERE ($1='' OR nip ILIKE $2 OR name ILIKE $2)`, q, pattern).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := pool.Query(ctx, `
		SELECT `+teacherCols+`
		FROM teachers t JOIN units un ON un.id=t.unit_id
		WHERE ($1='' OR t.nip ILIKE $2 OR t.name ILIKE $2)
		ORDER BY t.name ASC LIMIT $3 OFFSET $4`, q, pattern, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := make([]Teacher, 0)
	for rows.Next() {
		t, err := scanTeacher(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, t)
	}
	return result, total, rows.Err()
}

// UpsertImportedTeacher membuat atau memperbarui unit, akun ASN, dan guru.
func UpsertImportedTeacher(ctx context.Context, tx pgx.Tx, t ImportedTeacher, hashPassword func(string) (string, error)) (created bool, err error) {
	var parentID any
	if t.ParentUnitCode != "" {
		if err = tx.QueryRow(ctx, `
			INSERT INTO units (code,name,type,parent_id) VALUES ($1,$2,'korwil',NULL)
			ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name,type='korwil',parent_id=NULL,updated_at=now()
			RETURNING id`, t.ParentUnitCode, t.ParentUnitName).Scan(&parentID); err != nil {
			return false, fmt.Errorf("upsert parent unit: %w", err)
		}
	}
	var unitID int64
	if err = tx.QueryRow(ctx, `
		INSERT INTO units (code, name, type, parent_id) VALUES ($1,$2,$3,$4)
		ON CONFLICT (code) DO UPDATE SET name=EXCLUDED.name, type=EXCLUDED.type, parent_id=EXCLUDED.parent_id, updated_at=now()
		RETURNING id`, t.UnitCode, t.UnitName, t.UnitType, parentID).Scan(&unitID); err != nil {
		return false, fmt.Errorf("upsert unit: %w", err)
	}
	var existingRole string
	roleErr := tx.QueryRow(ctx, `SELECT role FROM users WHERE username=$1`, t.NIP).Scan(&existingRole)
	if roleErr == nil && existingRole != "asn" {
		return false, errors.New("NIP sudah dipakai akun petugas; username akun petugas tidak boleh sama dengan NIP")
	}
	if roleErr != nil && !errors.Is(roleErr, pgx.ErrNoRows) {
		return false, fmt.Errorf("cek username ASN: %w", roleErr)
	}
	var userID int64
	if errors.Is(roleErr, pgx.ErrNoRows) {
		passwordHash, hashErr := hashPassword(t.NIP)
		if hashErr != nil {
			return false, fmt.Errorf("hash password ASN: %w", hashErr)
		}
		err = tx.QueryRow(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id,is_active) VALUES ($1,$2,'asn',$3,$4,true) RETURNING id`, t.NIP, passwordHash, t.Name, unitID).Scan(&userID)
	} else {
		err = tx.QueryRow(ctx, `UPDATE users SET name=$1, unit_id=$2, is_active=true, updated_at=now() WHERE username=$3 AND role='asn' RETURNING id`, t.Name, unitID, t.NIP).Scan(&userID)
	}
	if err != nil {
		return false, fmt.Errorf("upsert user ASN: %w", err)
	}
	var existingID int64
	err = tx.QueryRow(ctx, `SELECT id FROM teachers WHERE nip=$1`, t.NIP).Scan(&existingID)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `INSERT INTO teachers (user_id,nip,name,asn_type,unit_id,pangkat_gol,pangkat,jabatan,birth_date,masa_kerja_tahun,masa_kerja_source,tmt_kgb_last) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, userID, t.NIP, t.Name, t.ASNType, unitID, t.PangkatGol, t.Pangkat, t.Jabatan, t.BirthDate, t.MasaKerjaTahun, t.MasaKerjaSource, t.TMTKGBLast)
		return true, err
	}
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `UPDATE teachers SET user_id=$1,name=$2,asn_type=$3,unit_id=$4,pangkat_gol=$5,pangkat=$6,jabatan=$7,birth_date=$8,masa_kerja_tahun=$9,masa_kerja_source=$10,tmt_kgb_last=$11,updated_at=now() WHERE id=$12`, userID, t.Name, t.ASNType, unitID, t.PangkatGol, t.Pangkat, t.Jabatan, t.BirthDate, t.MasaKerjaTahun, t.MasaKerjaSource, t.TMTKGBLast, existingID)
	return false, err
}

// ValidateImportedTeacher memeriksa invariant master yang dapat dipercaya.
func IsNIPUsername(username string) bool {
	if len(username) != 18 {
		return false
	}
	for _, r := range username {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func ValidateImportedTeacher(t ImportedTeacher) error {
	if len(t.NIP) != 18 {
		return fmt.Errorf("NIP %q harus 18 digit", t.NIP)
	}
	for _, r := range t.NIP {
		if r < '0' || r > '9' {
			return fmt.Errorf("NIP %q harus numerik", t.NIP)
		}
	}
	if t.Name == "" || t.UnitCode == "" || t.UnitName == "" {
		return errors.New("nama dan unit wajib diisi")
	}
	if t.ASNType != "pns" && t.ASNType != "pppk" {
		return errors.New("jenis ASN harus pns atau pppk")
	}
	if t.MasaKerjaTahun < 0 {
		return errors.New("masa kerja tidak boleh negatif")
	}
	if t.ASNType == "pppk" && t.PangkatGol != "IX" {
		return errors.New("golongan PPPK guru harus IX")
	}
	return nil
}

// EnsureImportAudit menyimpan riwayat impor master.
func EnsureImportAudit(ctx context.Context, pool *pgxpool.Pool, actorID int64, fileName string, result ImportResult) error {
	notes := ""
	for _, n := range result.Notes {
		if notes != "" {
			notes += "\n"
		}
		notes += n
	}
	_, err := pool.Exec(ctx, `INSERT INTO bkn_imports (file_name, imported_by, rows_total, rows_created, rows_updated, rows_skipped, notes) VALUES ($1,$2,$3,$4,$5,$6,$7)`, fileName, actorID, result.RowsTotal, result.RowsCreated, result.RowsUpdated, result.RowsSkipped, nullIfEmpty(notes))
	return err
}

// UpdateTeacherAfterIssue menerapkan snapshot KGB yang telah diterbitkan.
func UpdateTeacherAfterIssue(ctx context.Context, tx pgx.Tx, teacherID int64, proposedTMT time.Time, proposedMasaKerja int) error {
	if proposedMasaKerja < 0 {
		return errors.New("masa kerja snapshot tidak boleh negatif")
	}
	_, err := tx.Exec(ctx, `UPDATE teachers SET tmt_kgb_last=$1, masa_kerja_tahun=$2+2, masa_kerja_source='kgb_terbit', updated_at=now() WHERE id=$3`, proposedTMT, proposedMasaKerja, teacherID)
	return err
}

// ApplyTeacherChangeAtIssue menerapkan snapshot perubahan yang sudah melewati
// verifikasi dan TTE. NIP, nama, dan birth_date tidak pernah disentuh.
func ApplyTeacherChangeAtIssue(ctx context.Context, tx pgx.Tx, teacherID int64, change TeacherChange) error {
	_, err := tx.Exec(ctx, `
		UPDATE teachers SET
			pangkat_gol=COALESCE($1,pangkat_gol),
			pangkat=COALESCE($2,pangkat),
			jabatan=COALESCE($3,jabatan),
			unit_id=COALESCE($4,unit_id),
			updated_at=now()
		WHERE id=$5`, change.PangkatGol, change.Pangkat, change.Jabatan, change.UnitID, teacherID)
	return err
}

// IsUniqueViolation membantu handler memetakan konflik username/NIP.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
