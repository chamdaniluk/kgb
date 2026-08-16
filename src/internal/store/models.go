package store

import (
	"errors"
	"time"
)

// LetterSummary adalah ringkasan surat yang terkait dengan pengajuan.
type LetterSummary struct {
	ID           int64     `json:"id"`
	Number       string    `json:"number"`
	IssuedAt     time.Time `json:"issued_at"`
	TTEReceiptID string    `json:"tte_receipt_id,omitempty"`
}

// Submission adalah pengajuan KGB beserta snapshot data guru.
type Submission struct {
	ID             int64          `json:"id"`
	TeacherID      int64          `json:"teacher_id"`
	Status         string         `json:"status"`
	ProposedTMT    time.Time      `json:"proposed_tmt"`
	CurrentSalary  string         `json:"current_salary"`
	NextSalary     string         `json:"next_salary"`
	FileName       string         `json:"file_name,omitempty"`
	FilePath       string         `json:"-"`
	FileSize       int            `json:"file_size"`
	RejectionNote  string         `json:"rejection_note,omitempty"`
	SubmittedAt    *time.Time     `json:"submitted_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	TeacherName    string         `json:"teacher_name"`
	NIP            string         `json:"nip"`
	ASNType        string         `json:"asn_type"`
	PangkatGol     string         `json:"pangkat_gol"`
	MasaKerjaTahun int            `json:"masa_kerja_tahun"`
	UnitID         int64          `json:"unit_id"`
	UnitName       string         `json:"unit_name"`
	Letter         *LetterSummary `json:"letter,omitempty"`
}

// AuditLog adalah satu entri jejak audit append-only.
type AuditLog struct {
	ID           int64          `json:"id"`
	ActorUserID  *int64         `json:"actor_user_id,omitempty"`
	ActorName    string         `json:"actor_name,omitempty"`
	SubmissionID *int64         `json:"submission_id,omitempty"`
	Action       string         `json:"action"`
	Details      map[string]any `json:"details,omitempty"`
	IP           string         `json:"ip,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// Unit adalah unit kerja verifikasi.
type Unit struct {
	ID         int64     `json:"id"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	ParentID   *int64    `json:"parent_id,omitempty"`
	ParentName string    `json:"parent_name,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UserSummary adalah informasi akun yang aman ditampilkan admin.
type UserSummary struct {
	ID        int64      `json:"id"`
	Username  string     `json:"username"`
	Role      string     `json:"role"`
	Name      string     `json:"name"`
	UnitID    *int64     `json:"unit_id,omitempty"`
	UnitName  *string    `json:"unit_name,omitempty"`
	IsActive  bool       `json:"is_active"`
	LastLogin *time.Time `json:"last_login_at,omitempty"`
}

// SalaryScale adalah satu baris skala gaji resmi.
type SalaryScale struct {
	ID             int64  `json:"id"`
	ASNType        string `json:"asn_type"`
	Golongan       string `json:"golongan"`
	MasaKerjaTahun int    `json:"masa_kerja_tahun"`
	Gaji           string `json:"gaji"`
}

// LetterNumberTemplate adalah pola nomor surat.
type LetterNumberTemplate struct {
	ID        int64     `json:"id"`
	Pattern   string    `json:"pattern"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PublicStats berisi agregat yang aman dipublikasikan.
type PublicStats struct {
	TeachersTotal       int64            `json:"teachers_total"`
	SubmissionsActive   int64            `json:"submissions_active"`
	LettersIssued       int64            `json:"letters_issued"`
	SubmissionsPerMonth []MonthCount     `json:"submissions_per_month"`
	PerStatus           map[string]int64 `json:"per_status"`
	PerUnit             []UnitCount      `json:"per_unit"`
}

// MonthCount adalah agregat pengajuan per bulan.
type MonthCount struct {
	Month string `json:"month"`
	Count int64  `json:"count"`
}

// UnitCount adalah agregat pengajuan per unit.
type UnitCount struct {
	Unit  string `json:"unit"`
	Count int64  `json:"count"`
}

// ImportedTeacher adalah baris master ASN setelah dinormalisasi dari file BKN.
type ImportedTeacher struct {
	NIP            string
	Name           string
	ASNType        string
	UnitCode       string
	UnitName       string
	UnitType       string
	PangkatGol     string
	MasaKerjaTahun int
	TMTKGBLast     *time.Time
}

// Staff role constants are the normalized roles used by the SI CENDIKIA account importer.
const (
	StaffRoleAdminDinas      = "admin_dinas"
	StaffRolePimpinan        = "pimpinan"
	StaffRoleVerifikatorUnit = "verifikator_unit"
)

// ImportResult merangkum hasil impor master.
type ImportResult struct {
	RowsTotal   int      `json:"rows_total"`
	RowsCreated int      `json:"rows_created"`
	RowsUpdated int      `json:"rows_updated"`
	RowsSkipped int      `json:"rows_skipped"`
	Notes       []string `json:"notes,omitempty"`
}

// ErrConflict menandai perubahan status yang kalah oleh request lain.
var ErrConflict = errors.New("konflik perubahan data")

// ErrForbidden dipakai service saat objek ada tetapi bukan milik scope aktor.
var ErrForbidden = errors.New("akses tidak berwenang")
