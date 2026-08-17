package letterdata

import (
	"strings"
	"testing"
	"time"
)

func date(s string) *time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return &t
}

func validPNS() Draft {
	return Draft{
		ASNType:       "pns",
		BirthPlace:    "Grobogan",
		BirthDate:     date("1990-02-13"),
		Karpeg:        "I 123456",
		Pangkat:       "Penata Muda Tingkat I",
		Jabatan:       "Guru Ahli Pertama",
		LastSKPejabat: "Kepala Dinas Pendidikan",
		LastSKNomor:   "800/001/4.2/2024",
		LastSKTanggal: date("2020-12-01"),
		LastSKTMT:     date("2020-12-01"),
		MKGLamaTahun:  4,
		MKGLamaBulan:  0,
		MKGBaruTahun:  6,
		MKGBaruBulan:  0,
		ProposedTMT:   *date("2026-12-01"),
		CurrentSalary: "3089300",
		NextSalary:    "3186600",
		UnitName:      "SDN 3 GROBOGAN",
		PangkatGol:    "III/b",
	}
}

func TestValidateDraftPNSLengkap(t *testing.T) {
	if err := ValidateDraft(validPNS()); err != nil {
		t.Fatalf("draft PNS lengkap ditolak: %v", err)
	}
}

func TestValidateDraftPNSTanpaKarpeg(t *testing.T) {
	d := validPNS()
	d.Karpeg = ""
	err := ValidateDraft(d)
	if err == nil {
		t.Fatal("draft tanpa karpeg harus ditolak")
	}
	if !strings.Contains(err.Error(), "Karpeg") {
		t.Fatalf("pesan error = %q, ingin menyebut Karpeg", err)
	}
}

func TestValidateDraftMenolakStripDanSpasi(t *testing.T) {
	d := validPNS()
	d.BirthPlace = "—"
	if err := ValidateDraft(d); err == nil {
		t.Fatal("placeholder — tidak boleh lolos")
	}
	d = validPNS()
	d.Jabatan = "   "
	if err := ValidateDraft(d); err == nil {
		t.Fatal("spasi saja tidak boleh lolos")
	}
}

func TestValidateDraftPPPKTanpaMasaPerjanjian(t *testing.T) {
	d := validPNS()
	d.ASNType = "pppk"
	d.Karpeg = "-"
	d.MasaPerjanjian = ""
	d.PerpanjanganDash = true
	err := ValidateDraft(d)
	if err == nil {
		t.Fatal("PPPK tanpa masa perjanjian harus ditolak")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "perjanjian") {
		t.Fatalf("pesan error = %q, ingin menyebut perjanjian", err)
	}
}

func TestValidateDraftPPPKPerpanjanganDash(t *testing.T) {
	d := validPNS()
	d.ASNType = "pppk"
	d.Karpeg = "-"
	d.MasaPerjanjian = "5 tahun"
	d.PerpanjanganDash = true
	if err := ValidateDraft(d); err != nil {
		t.Fatalf("PPPK dengan perpanjangan - harus diterima: %v", err)
	}
}

func TestValidateDraftTMTBukanAnniversaryDitolak(t *testing.T) {
	d := validPNS()
	d.ProposedTMT = *date("2026-09-01")
	if err := ValidateDraft(d); err == nil {
		t.Fatal("TMT 1 September dari SK 1 Desember harus ditolak")
	}
}
