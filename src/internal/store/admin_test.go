package store

import (
	"context"
	"testing"
)

// CreateStaffUser dan UpdateStaffUser harus benar-benar menulis employee_number
// & job_title ke tabel users (jalur data blok TTD pimpinan). Tanpa ini akun
// pimpinan mentok di SIGNER_NOT_CONFIGURED saat TTE.
func TestStaffUserPersistsSignerProfile(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}

	created, err := CreateStaffUser(ctx, pool, "pimpinan.disdik", "hash", StaffRolePimpinan,
		"Kepala Dinas", nil, "3300000000000001", "ttd-base64",
		"198001012005011002", "Kepala Dinas Pendidikan")
	if err != nil {
		t.Fatalf("CreateStaffUser() error = %v", err)
	}

	var employeeNumber, jobTitle string
	if err := pool.QueryRow(ctx, `SELECT employee_number, job_title FROM users WHERE id=$1`, created.ID).
		Scan(&employeeNumber, &jobTitle); err != nil {
		t.Fatal(err)
	}
	if employeeNumber != "198001012005011002" {
		t.Errorf("employee_number tersimpan = %q, want 198001012005011002", employeeNumber)
	}
	if jobTitle != "Kepala Dinas Pendidikan" {
		t.Errorf("job_title tersimpan = %q, want Kepala Dinas Pendidikan", jobTitle)
	}

	newTitle := "Sekretaris Dinas Pendidikan"
	if _, err := UpdateStaffUser(ctx, pool, created.ID, nil, nil, nil, nil, nil, nil, nil, nil, &newTitle); err != nil {
		t.Fatalf("UpdateStaffUser() error = %v", err)
	}

	if err := pool.QueryRow(ctx, `SELECT employee_number, job_title FROM users WHERE id=$1`, created.ID).
		Scan(&employeeNumber, &jobTitle); err != nil {
		t.Fatal(err)
	}
	if jobTitle != newTitle {
		t.Errorf("job_title setelah update = %q, want %q", jobTitle, newTitle)
	}
	// employee_number tidak dikirim (nil) pada update, jadi harus tetap.
	if employeeNumber != "198001012005011002" {
		t.Errorf("employee_number berubah tanpa diminta = %q, want 198001012005011002", employeeNumber)
	}
}
