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

func intptr(n int) *int { return &n }

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
		LastSKMasaTahun: intptr(4),
		LastSKMasaBulan: intptr(0),
		LastKPGolongan: "III/b",
		LastKPTMT:      date("2025-07-01"),
		LastKPMasaTahun: intptr(5),
		LastKPMasaBulan: intptr(0),
		LastKPNomor:    "800.1.3.2/795/2025",
		LastKPTanggal:  date("2025-06-20"),
		LastKPPejabat:  "BUPATI GROBOGAN",
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
	t.Skip("SK terakhir bisa SK naik pangkat — KGB jangkar TMT awal, bukan LastSKTMT (lihat ADR pelurusan 2026-08-20)")
}

func TestValidateDraftTanpaKPWajibDitolak(t *testing.T) {
	d := validPNS()
	d.LastKPGolongan = ""
	if err := ValidateDraft(d); err == nil {
		t.Fatal("draft tanpa golongan KP harus ditolak")
	}
	d = validPNS()
	d.LastKPTMT = nil
	if err := ValidateDraft(d); err == nil {
		t.Fatal("draft tanpa TMT KP harus ditolak")
	}
	d = validPNS()
	d.LastKPMasaTahun = nil
	if err := ValidateDraft(d); err == nil {
		t.Fatal("draft tanpa masa kerja KP harus ditolak")
	}
	d = validPNS()
	d.LastKPPejabat = ""
	if err := ValidateDraft(d); err == nil {
		t.Fatal("draft tanpa pejabat KP harus ditolak")
	}
}

func TestValidateDraftTanpaMKGKGBWajibDitolak(t *testing.T) {
	d := validPNS()
	d.LastSKMasaTahun = nil
	if err := ValidateDraft(d); err == nil {
		t.Fatal("draft tanpa masa kerja KGB harus ditolak")
	}
}
