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
	ID                       int64          `json:"id"`
	TeacherID                int64          `json:"teacher_id"`
	Status                   string         `json:"status"`
	ProposedTMT              time.Time      `json:"proposed_tmt"`
	ProposedMasaKerjaTahun   *int           `json:"proposed_masa_kerja_tahun,omitempty"`
	ProposedTMTKGBLast       *time.Time     `json:"proposed_tmt_kgb_last,omitempty"`
	ProposedPangkatGol       *string        `json:"proposed_pangkat_gol,omitempty"`
	ProposedPangkat          *string        `json:"proposed_pangkat,omitempty"`
	ProposedJabatan          *string        `json:"proposed_jabatan,omitempty"`
	ProposedUnitID           *int64         `json:"proposed_unit_id,omitempty"`
	ProposedUnitName         string         `json:"proposed_unit_name,omitempty"`
	ProposedEffectiveDate    *time.Time     `json:"proposed_effective_date,omitempty"`
	ProposedChangeNote       string         `json:"proposed_change_note,omitempty"`
	CurrentSalary            string         `json:"current_salary"`
	NextSalary               string         `json:"next_salary"`
	FileName                 string         `json:"file_name,omitempty"`
	FilePath                 string         `json:"-"`
	FileSize                 int            `json:"file_size"`
	// Slot berkas pendukung per jenis ASN (017): PNS = SK KP + KGB terakhir
	// (wajib); PPPK = SK terakhir (wajib) + KGB terakhir (opsional) + SKP
	// 2 tahun (wajib). FilePath dirahasiakan; unduh via endpoint per slot.
	FileKPName  string `json:"file_kp_name,omitempty"`
	FileKPPath  string `json:"-"`
	FileKPSize  int    `json:"file_kp_size,omitempty"`
	FileKGBName string `json:"file_kgb_name,omitempty"`
	FileKGBPath string `json:"-"`
	FileKGBSize int    `json:"file_kgb_size,omitempty"`
	FileSKPName string `json:"file_skp_name,omitempty"`
	FileSKPPath string `json:"-"`
	FileSKPSize int    `json:"file_skp_size,omitempty"`
	RejectionNote            string         `json:"rejection_note,omitempty"`
	SubmittedAt              *time.Time     `json:"submitted_at,omitempty"`
	CreatedAt                time.Time      `json:"created_at"`
	UpdatedAt                time.Time      `json:"updated_at"`
	TeacherName              string         `json:"teacher_name"`
	NIP                      string         `json:"nip"`
	ASNType                  string         `json:"asn_type"`
	PangkatGol               string         `json:"pangkat_gol"`
	Pangkat                  string         `json:"pangkat,omitempty"`
	Jabatan                  string         `json:"jabatan,omitempty"`
	MasaKerjaTahun           int            `json:"masa_kerja_tahun"`
	UnitID                   int64          `json:"unit_id"`
	UnitName                 string         `json:"unit_name"`
	UnitType                 string         `json:"unit_type,omitempty"`
	UnitDistrict             string         `json:"unit_district,omitempty"`
	UnitKorwilName           string         `json:"unit_korwil_name,omitempty"`
	Letter                   *LetterSummary `json:"letter,omitempty"`
	SnapshotBirthPlace       string         `json:"snapshot_birth_place,omitempty"`
	SnapshotBirthDate        *time.Time     `json:"snapshot_birth_date,omitempty"`
	SnapshotKarpeg           string         `json:"snapshot_karpeg,omitempty"`
	SnapshotLastSKPejabat    string         `json:"snapshot_last_sk_pejabat,omitempty"`
	SnapshotLastSKTanggal    *time.Time     `json:"snapshot_last_sk_tanggal,omitempty"`
	SnapshotLastSKNomor      string         `json:"snapshot_last_sk_nomor,omitempty"`
	SnapshotLastSKTMTBerlaku *time.Time     `json:"snapshot_last_sk_tmt_berlaku,omitempty"`
	SnapshotLastSKMasaTahun  *int           `json:"snapshot_last_sk_masa_tahun,omitempty"`
	SnapshotLastSKMasaBulan  *int           `json:"snapshot_last_sk_masa_bulan,omitempty"`
	DraftBirthPlace          string         `json:"draft_birth_place,omitempty"`
	DraftBirthDate           *time.Time     `json:"draft_birth_date,omitempty"`
	DraftKarpeg              string         `json:"draft_karpeg,omitempty"`
	DraftPangkat             string         `json:"draft_pangkat,omitempty"`
	DraftJabatan             string         `json:"draft_jabatan,omitempty"`
	DraftLastSKPejabat       string         `json:"draft_last_sk_pejabat,omitempty"`
	DraftLastSKTanggal       *time.Time     `json:"draft_last_sk_tanggal,omitempty"`
	DraftLastSKNomor         string         `json:"draft_last_sk_nomor,omitempty"`
	DraftLastSKTMT           *time.Time     `json:"draft_last_sk_tmt,omitempty"`
	DraftLastSKMasaTahun     *int           `json:"draft_last_sk_masa_tahun,omitempty"`
	DraftLastSKMasaBulan     *int           `json:"draft_last_sk_masa_bulan,omitempty"`
	DraftLastKPGolongan      string         `json:"draft_last_kp_golongan,omitempty"`
	DraftLastKPTMT           *time.Time     `json:"draft_last_kp_tmt,omitempty"`
	DraftLastKPNomor         string         `json:"draft_last_kp_nomor,omitempty"`
	DraftLastKPTanggal       *time.Time     `json:"draft_last_kp_tanggal,omitempty"`
	DraftLastKPPejabat       string         `json:"draft_last_kp_pejabat,omitempty"`
	DraftLastKPMasaTahun     *int           `json:"draft_last_kp_masa_tahun,omitempty"`
	DraftLastKPMasaBulan     *int           `json:"draft_last_kp_masa_bulan,omitempty"`
	DraftMKGLamaTahun        *int           `json:"draft_mkg_lama_tahun,omitempty"`
	DraftMKGLamaBulan        *int           `json:"draft_mkg_lama_bulan,omitempty"`
	DraftMKGBaruTahun        *int           `json:"draft_mkg_baru_tahun,omitempty"`
	DraftMKGBaruBulan        *int           `json:"draft_mkg_baru_bulan,omitempty"`
	DraftMasaPerjanjian      string         `json:"draft_masa_perjanjian,omitempty"`
	DraftPerpanjangan        *time.Time     `json:"draft_perpanjangan_kontrak,omitempty"`
	TMTAwal                  *time.Time     `json:"tmt_awal,omitempty"`
}

// SubmissionFile adalah satu slot berkas pendukung (nama asli + path privat).
type SubmissionFile struct {
	Name string
	Path string
	Size int64
}

// SubmissionFiles menampung berkas utama + slot pendukung (017).
// PNS: KP (SK KP) + KGB. PPPK: KP (SK terakhir) + KGB (opsional) + SKP.
type SubmissionFiles struct {
	Main SubmissionFile
	KP   SubmissionFile
	KGB  SubmissionFile
	SKP  SubmissionFile
}

// Slot mengembalikan (path, nama) untuk slot unduhan: "" | kp | kgb | skp.
// Slot "" memakai berkas utama bila ada, lalu fallback ke slot KP.
func (s Submission) Slot(slot string) (path, name string) {
	switch slot {
	case "kp":
		return s.FileKPPath, s.FileKPName
	case "kgb":
		return s.FileKGBPath, s.FileKGBName
	case "skp":
		return s.FileSKPPath, s.FileSKPName
	default:
		if s.FilePath != "" {
			return s.FilePath, s.FileName
		}
		return s.FileKPPath, s.FileKPName
	}
}

// FileNames merangkum nama berkas untuk audit (tanpa path privat).
func FileNames(f SubmissionFiles) map[string]any {
	out := map[string]any{}
	if f.Main.Name != "" {
		out["utama"] = f.Main.Name
	}
	if f.KP.Name != "" {
		out["kp"] = f.KP.Name
	}
	if f.KGB.Name != "" {
		out["kgb"] = f.KGB.Name
	}
	if f.SKP.Name != "" {
		out["skp"] = f.SKP.Name
	}
	return out
}

// KPLast adalah SK Kenaikan Pangkat terakhir: golongan dikunci dari SIPPASN,
// masa kerja mengikuti SK yang diterbitkan. Bila KP lebih baru dari KGB
// terakhir, gaji KGB mengacu golongan KP; jangka waktu KGB tetap 2 tahun
// dari KGB terakhir.
type KPLast struct {
	Golongan  string
	TMT       *time.Time
	MasaTahun *int
	MasaBulan *int
	Nomor     string
	Tanggal   *time.Time
	Pejabat   string
}

// HasData melaporkan apakah ada isian KP (golongan + TMT wajib).
func (k KPLast) HasData() bool {
	return k.Golongan != "" && k.TMT != nil
}

// EffectiveGolongan menentukan golongan acuan gaji: golongan KP bila SK KP
// diterbitkan lebih baru (tanggal SK) dari SK KGB terakhir, selain itu
// golongan KGB/guru. Keputusan owner 2026-09-09: acuan "SK terbaru" adalah
// tanggal SK, bukan TMT berlaku.
func EffectiveGolongan(kp KPLast, kgbTanggalSK *time.Time, golKGB string) string {
	if kp.HasData() && SKNewer(kp.Tanggal, kgbTanggalSK) {
		return kp.Golongan
	}
	return golKGB
}

// SKNewer melaporkan apakah tanggal SK a lebih baru dari b (nil = tidak ada).
// Tanpa tanggal di kedua sisi, a dianggap tidak lebih baru.
func SKNewer(a, b *time.Time) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	return !a.Before(*b)
}

// KGBLast adalah SK KGB terakhir: golongan ruang saat KGB terakhir dapat
// diubah (bisa berbeda dari KP bila ada perubahan di tengah masa KGB),
// masa kerja mengikuti SK yang diterbitkan.
type KGBLast struct {
	Golongan  string
	TMT       *time.Time
	MasaTahun *int
	MasaBulan *int
	Nomor     string
	Tanggal   *time.Time
	Pejabat   string
}

// HasData melaporkan apakah ada isian KGB (golongan + TMT wajib).
func (k KGBLast) HasData() bool {
	return k.Golongan != "" && k.TMT != nil
}

// LetterDraft adalah nilai naskah SK yang dikirim ASN pada usulan.
type LetterDraft struct {
	BirthPlace     string
	BirthDate      *time.Time
	Karpeg         string
	Pangkat        string
	Jabatan        string
	LastSKPejabat  string
	LastSKTanggal  *time.Time
	LastSKNomor    string
	LastSKTMT      *time.Time
	LastSKMasaTahun *int
	LastSKMasaBulan *int
	LastKPGolongan string
	LastKPTMT      *time.Time
	LastKPMasaTahun *int
	LastKPMasaBulan *int
	LastKPNomor    string
	LastKPTanggal  *time.Time
	LastKPPejabat  string
	MKGLamaTahun   *int
	MKGLamaBulan   *int
	MKGBaruTahun   *int
	MKGBaruBulan   *int
	MasaPerjanjian string
	Perpanjangan   *time.Time
}

// VerificationUnitID returns the unit that owns the current workflow stage.
// A validated mutation proposal takes precedence over the BKN master unit.
func (s Submission) VerificationUnitID() int64 {
	if s.ProposedUnitID != nil {
		return *s.ProposedUnitID
	}
	return s.UnitID
}

// HasTeacherChange reports whether the submission carries an approved-data
// proposal that must be reviewed together with the KGB request.
func (s Submission) HasTeacherChange() bool {
	return s.ProposedPangkatGol != nil || s.ProposedPangkat != nil || s.ProposedJabatan != nil || s.ProposedUnitID != nil
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// LetterDraftValues mengembalikan naskah yang akan dicetak dan divalidasi.
func (s Submission) LetterDraftValues() LetterDraft {
	return LetterDraft{
		BirthPlace:     s.DraftBirthPlace,
		BirthDate:      s.DraftBirthDate,
		Karpeg:         s.DraftKarpeg,
		Pangkat:        firstNonEmpty(s.DraftPangkat, s.Pangkat),
		Jabatan:        firstNonEmpty(s.DraftJabatan, s.Jabatan),
		LastSKPejabat:  s.DraftLastSKPejabat,
		LastSKTanggal:  s.DraftLastSKTanggal,
		LastSKNomor:    s.DraftLastSKNomor,
		LastSKTMT:      s.DraftLastSKTMT,
		LastSKMasaTahun: s.DraftLastSKMasaTahun,
		LastSKMasaBulan: s.DraftLastSKMasaBulan,
		LastKPGolongan: s.DraftLastKPGolongan,
		LastKPTMT:      s.DraftLastKPTMT,
		LastKPNomor:    s.DraftLastKPNomor,
		LastKPTanggal:  s.DraftLastKPTanggal,
		LastKPPejabat:  s.DraftLastKPPejabat,
		LastKPMasaTahun: s.DraftLastKPMasaTahun,
		LastKPMasaBulan: s.DraftLastKPMasaBulan,
		MKGLamaTahun:   s.DraftMKGLamaTahun,
		MKGLamaBulan:   s.DraftMKGLamaBulan,
		MKGBaruTahun:   s.DraftMKGBaruTahun,
		MKGBaruBulan:   s.DraftMKGBaruBulan,
		MasaPerjanjian: s.DraftMasaPerjanjian,
		Perpanjangan:   s.DraftPerpanjangan,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
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
	District   string    `json:"district,omitempty"`
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
	RoleGroup string     `json:"role_group,omitempty"`
	Name      string     `json:"name"`
	UnitID    *int64     `json:"unit_id,omitempty"`
	UnitName  *string    `json:"unit_name,omitempty"`
	UnitType  string     `json:"unit_type,omitempty"`
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

// TeacherChange adalah perubahan kepegawaian yang diusulkan melalui SK.
// Identitas BKN (NIP, nama, tanggal lahir) sengaja tidak termasuk di sini.
type TeacherChange struct {
	PangkatGol    *string
	Pangkat       *string
	Jabatan       *string
	UnitID        *int64
	EffectiveDate *time.Time
	Note          string
	AuditDetails  map[string]any
}

// ImportedTeacher adalah baris master ASN setelah dinormalisasi dari file BKN
// atau sinkron SIPPASN.
type ImportedTeacher struct {
	NIP             string
	Name            string
	ASNType         string
	Kategori        string // guru atau non_guru; kosong = guru (kompatibel impor lama)
	UnitCode        string
	UnitName        string
	UnitType        string
	UnitDistrict    string
	ParentUnitCode  string
	ParentUnitName  string
	PangkatGol      string
	Pangkat         string
	Jabatan         string
	BirthDate       *time.Time
	MasaKerjaTahun  int
	MasaKerjaSource string
	TMTKGBLast      *time.Time
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
