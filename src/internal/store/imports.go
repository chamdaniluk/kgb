package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ImportTeachers mengimpor master ASN dan mencatat ringkasan dalam satu transaksi.
func ImportTeachers(ctx context.Context, pool *pgxpool.Pool, actorID int64, fileName string, teachers []ImportedTeacher, hashPassword func(string) (string, error)) (ImportResult, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	defer tx.Rollback(ctx)
	result := ImportResult{RowsTotal: len(teachers), Notes: make([]string, 0)}
	for i, teacher := range teachers {
		if err := ValidateImportedTeacher(teacher); err != nil {
			result.RowsSkipped++
			result.Notes = append(result.Notes, fmt.Sprintf("baris %d: %v", i+2, err))
			continue
		}
		hash, err := hashPassword(teacher.NIP)
		if err != nil {
			return ImportResult{}, fmt.Errorf("hash password baris %d: %w", i+2, err)
		}
		created, err := UpsertImportedTeacher(ctx, tx, teacher, hash)
		if err != nil {
			return ImportResult{}, fmt.Errorf("baris %d NIP %s: %w", i+2, teacher.NIP, err)
		}
		if created {
			result.RowsCreated++
		} else {
			result.RowsUpdated++
		}
	}
	notes := ""
	for _, note := range result.Notes {
		if notes != "" {
			notes += "\n"
		}
		notes += note
	}
	if _, err := tx.Exec(ctx, `INSERT INTO bkn_imports (file_name, imported_by, rows_total, rows_created, rows_updated, rows_skipped, notes) VALUES ($1,$2,$3,$4,$5,$6,$7)`, fileName, actorID, result.RowsTotal, result.RowsCreated, result.RowsUpdated, result.RowsSkipped, nullIfEmpty(notes)); err != nil {
		return ImportResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ImportResult{}, err
	}
	return result, nil
}

// ImportStaffUsers mengimpor akun petugas secara atomik tanpa menyimpan password polos.
func ImportStaffUsers(ctx context.Context, pool *pgxpool.Pool, actorID int64, users []ImportedStaffUser, hashPassword func(string) (string, error)) (ImportResult, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	defer tx.Rollback(ctx)
	result := ImportResult{RowsTotal: len(users), Notes: make([]string, 0)}
	for i, item := range users {
		if item.Username == "" || item.Password == "" || item.Name == "" || item.Role == "" {
			result.RowsSkipped++
			result.Notes = append(result.Notes, fmt.Sprintf("baris %d: username, password, nama, dan role wajib", i+2))
			continue
		}
		if item.Role == "asn" || (item.Role != "verifikator_unit" && item.Role != "verifikator_dinas" && item.Role != "pimpinan" && item.Role != "admin") {
			result.RowsSkipped++
			result.Notes = append(result.Notes, fmt.Sprintf("baris %d: role petugas tidak valid", i+2))
			continue
		}
		if IsNIPUsername(item.Username) {
			result.RowsSkipped++
			result.Notes = append(result.Notes, fmt.Sprintf("baris %d: username petugas tidak boleh berupa NIP", i+2))
			continue
		}
		var existingRole string
		roleErr := tx.QueryRow(ctx, `SELECT role FROM users WHERE username=$1`, item.Username).Scan(&existingRole)
		if roleErr == nil && existingRole == "asn" {
			result.RowsSkipped++
			result.Notes = append(result.Notes, fmt.Sprintf("baris %d: username sudah menjadi akun ASN", i+2))
			continue
		}
		if roleErr != nil && !errors.Is(roleErr, pgx.ErrNoRows) {
			return ImportResult{}, roleErr
		}
		var collision bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM teachers WHERE nip=$1)`, item.Username).Scan(&collision); err != nil {
			return ImportResult{}, err
		}
		if collision {
			result.RowsSkipped++
			result.Notes = append(result.Notes, fmt.Sprintf("baris %d: username petugas sama dengan NIP", i+2))
			continue
		}
		var unitID any
		if item.UnitCode != "" {
			unitType := item.UnitType
			if unitType == "" {
				unitType = "smp"
			}
			var id int64
			if err := tx.QueryRow(ctx, `INSERT INTO units (code,name,type) VALUES ($1,$2,$3) ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,type=EXCLUDED.type,updated_at=now() RETURNING id`, item.UnitCode, item.UnitName, unitType).Scan(&id); err != nil {
				return ImportResult{}, err
			}
			unitID = id
		}
		hash, err := hashPassword(item.Password)
		if err != nil {
			return ImportResult{}, err
		}
		var existed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username=$1)`, item.Username).Scan(&existed); err != nil {
			return ImportResult{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO users (username,password_hash,role,name,unit_id,nik,signature_image_base64) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,'')) ON CONFLICT(username) DO UPDATE SET password_hash=EXCLUDED.password_hash,role=EXCLUDED.role,name=EXCLUDED.name,unit_id=EXCLUDED.unit_id,nik=EXCLUDED.nik,signature_image_base64=EXCLUDED.signature_image_base64,is_active=true,updated_at=now()`, item.Username, hash, item.Role, item.Name, unitID, item.NIK, item.SignatureImageBase64); err != nil {
			return ImportResult{}, fmt.Errorf("baris %d: %w", i+2, err)
		}
		if existed {
			result.RowsUpdated++
		} else {
			result.RowsCreated++
		}
	}
	details, _ := json.Marshal(map[string]any{"rows_total": result.RowsTotal, "rows_created": result.RowsCreated, "rows_updated": result.RowsUpdated, "rows_skipped": result.RowsSkipped})
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_user_id,action,details) VALUES ($1,'impor_akun_petugas',$2)`, actorID, details); err != nil {
		return ImportResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ImportResult{}, err
	}
	return result, nil
}
