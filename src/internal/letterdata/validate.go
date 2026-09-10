// Package letterdata memvalidasi naskah SK sebelum submit, verifikasi, dan TTE.
package letterdata

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrDraftIncomplete menandai naskah SK yang masih bolong.
var ErrDraftIncomplete = errors.New("naskah SK belum lengkap")

// Draft adalah nilai naskah yang akan dicetak ke template DOCX.
// Seluruh kolom bersifat wajib: KP terakhir, KGB terakhir, dan naskah SK.
type Draft struct {
	ASNType          string
	BirthPlace       string
	Karpeg           string
	Pangkat          string
	PangkatGol       string
	Jabatan          string
	BirthDate        *time.Time
	LastSKPejabat    string
	LastSKNomor      string
	LastSKTanggal    *time.Time
	LastSKTMT        *time.Time
	LastSKMasaTahun  *int
	LastSKMasaBulan  *int
	LastKPGolongan   string
	LastKPTMT        *time.Time
	LastKPMasaTahun  *int
	LastKPMasaBulan  *int
	LastKPNomor      string
	LastKPTanggal    *time.Time
	LastKPPejabat    string
	MKGLamaTahun     int
	MKGLamaBulan     int
	MKGBaruTahun     int
	MKGBaruBulan     int
	ProposedTMT      time.Time
	CurrentSalary    string
	NextSalary       string
	UnitName         string
	MasaPerjanjian   string
	Perpanjangan     *time.Time
	PerpanjanganDash bool
}

func Normalize(v string) string {
	return strings.TrimSpace(v)
}

func requiredString(label, v string) error {
	v = Normalize(v)
	if v == "" || v == "—" {
		return fmt.Errorf("%w: %s wajib diisi", ErrDraftIncomplete, label)
	}
	return nil
}

func requiredDate(label string, v *time.Time) error {
	if v == nil || v.IsZero() {
		return fmt.Errorf("%w: %s wajib diisi", ErrDraftIncomplete, label)
	}
	return nil
}

func requiredMonth(label string, v int) error {
	if v < 0 || v > 11 {
		return fmt.Errorf("%w: %s harus 0–11", ErrDraftIncomplete, label)
	}
	return nil
}

func requiredYear(label string, v int) error {
	if v < 0 {
		return fmt.Errorf("%w: %s tidak boleh negatif", ErrDraftIncomplete, label)
	}
	return nil
}

func requiredYearPtr(label string, v *int) error {
	if v == nil || *v < 0 {
		return fmt.Errorf("%w: %s wajib diisi", ErrDraftIncomplete, label)
	}
	return nil
}

func requiredMonthPtr(label string, v *int) error {
	if v == nil || *v < 0 || *v > 11 {
		return fmt.Errorf("%w: %s wajib diisi 0–11", ErrDraftIncomplete, label)
	}
	return nil
}

// ValidateDraft menolak naskah yang masih bolong atau memakai placeholder strip.
func ValidateDraft(d Draft) error {
	asn := strings.ToLower(Normalize(d.ASNType))
	if asn != "pns" && asn != "pppk" {
		return fmt.Errorf("%w: jenis ASN wajib pns atau pppk", ErrDraftIncomplete)
	}
	// Kolom bersama PNS & PPPK: identitas, SK terakhir (KGB), MKG, dan gaji.
	// Kolom KP / SK Pertama-Perpanjangan Kontrak wajib untuk keduanya —
	// penamaannya berbeda per jenis ASN di form, datanya sama (last_kp_*).
	checks := []error{
		requiredString("Tempat lahir", d.BirthPlace),
		requiredDate("Tanggal lahir", d.BirthDate),
		requiredString("Pangkat", d.Pangkat),
		requiredString("Jabatan", d.Jabatan),
		requiredString("Unit kerja", d.UnitName),
		requiredString("Golongan SK terakhir", d.LastKPGolongan),
		requiredDate("TMT SK terakhir (KP/Perpanjangan)", d.LastKPTMT),
		requiredYearPtr("Masa kerja SK terakhir (tahun)", d.LastKPMasaTahun),
		requiredMonthPtr("Masa kerja SK terakhir (bulan)", d.LastKPMasaBulan),
		requiredString("Nomor SK terakhir (KP/Perpanjangan)", d.LastKPNomor),
		requiredDate("Tanggal SK terakhir (KP/Perpanjangan)", d.LastKPTanggal),
		requiredString("Pejabat SK terakhir (KP/Perpanjangan)", d.LastKPPejabat),
		requiredString("Pejabat SK KGB terakhir", d.LastSKPejabat),
		requiredString("Nomor SK KGB terakhir", d.LastSKNomor),
		requiredDate("Tanggal SK KGB terakhir", d.LastSKTanggal),
		requiredDate("TMT SK KGB terakhir", d.LastSKTMT),
		requiredYearPtr("Masa kerja KGB (tahun)", d.LastSKMasaTahun),
		requiredMonthPtr("Masa kerja KGB (bulan)", d.LastSKMasaBulan),
		requiredYear("Masa kerja lama (tahun)", d.MKGLamaTahun),
		requiredMonth("Masa kerja lama (bulan)", d.MKGLamaBulan),
		requiredYear("Masa kerja baru (tahun)", d.MKGBaruTahun),
		requiredMonth("Masa kerja baru (bulan)", d.MKGBaruBulan),
		requiredString("Gaji lama", d.CurrentSalary),
		requiredString("Gaji baru", d.NextSalary),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	if d.ProposedTMT.IsZero() {
		return fmt.Errorf("%w: TMT usulan wajib diisi", ErrDraftIncomplete)
	}
	// Masa kerja naskah mengikuti SK pemenang apa adanya (bisa ganjil /
	// berbulan, mis. SK KP 5 th 7 bl); KGB +2 tahun dgn bulan sama.
	// Grid gaji berkala (genap) dihitung terpisah di prepareKGBFromForm.
	if d.MKGBaruTahun != d.MKGLamaTahun+2 || d.MKGBaruBulan != d.MKGLamaBulan {
		return fmt.Errorf("%w: masa kerja baru harus +2 tahun dgn bulan sama", ErrDraftIncomplete)
	}
	// KGB berkala dihitung dari TMT awal (CPNS/pengangkatan), bukan SK terakhir
	// (SK terakhir bisa naik pangkat). Invarian di-cek di prepareKGBFromForm.
	if err := requiredString("Karpeg", d.Karpeg); err != nil {
		return err
	}
	if asn == "pppk" {
		if err := requiredString("Masa perjanjian kerja", d.MasaPerjanjian); err != nil {
			return err
		}
		if !d.PerpanjanganDash && d.Perpanjangan == nil {
			return fmt.Errorf("%w: Perpanjangan perjanjian kerja wajib diisi atau diisi -", ErrDraftIncomplete)
		}
	}
	return nil
}
