package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound dialami ketika query tidak mengembalikan baris.
var ErrNotFound = errors.New("tidak ditemukan")

// User: baris tabel users (+ nama unit via join, bisa kosong).
type User struct {
	ID                   int64
	Username             string
	PasswordHash         string
	Role                 string
	Name                 string
	UnitID               *int64
	UnitName             *string
	UnitType             *string
	UnitParentID         *int64
	IsActive             bool
	NIK                  *string
	SignatureImageBase64 *string
}

func scanUser(row pgxRow) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Name,
		&u.UnitID, &u.UnitName, &u.UnitType, &u.UnitParentID, &u.IsActive, &u.NIK, &u.SignatureImageBase64)
	return u, err
}

// pgxRow disatukan agar QueryRow dan Query tx bisa memakai scanUser.
type pgxRow interface {
	Scan(dest ...any) error
}

const userCols = `u.id, u.username, u.password_hash, u.role, u.name, u.unit_id, un.name, un.type, un.parent_id, u.is_active,
       u.nik, u.signature_image_base64
FROM users u LEFT JOIN units un ON un.id = u.unit_id`

func GetUserByUsername(ctx context.Context, pool *pgxpool.Pool, username string) (User, error) {
	u, err := scanUser(pool.QueryRow(ctx, `SELECT `+userCols+` WHERE u.username = $1`, username))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func GetUserByID(ctx context.Context, pool *pgxpool.Pool, id int64) (User, error) {
	u, err := scanUser(pool.QueryRow(ctx, `SELECT `+userCols+` WHERE u.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func TouchLastLogin(ctx context.Context, pool *pgxpool.Pool, userID int64) error {
	_, err := pool.Exec(ctx, `UPDATE users SET last_login_at = now(), updated_at = now() WHERE id = $1`, userID)
	return err
}
