package httpapi

import (
	"testing"
	"time"
)

func TestDeriveMasaKerjaUsesBKNRules(t *testing.T) {
	asOf := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	cpns := time.Date(2010, 8, 17, 0, 0, 0, 0, time.UTC)
	gol := time.Date(2021, 9, 1, 0, 0, 0, 0, time.UTC)

	got, source := deriveMasaKerja("pns", &cpns, &gol, asOf)
	if got != 15 || source != "tmt_cpns" {
		t.Fatalf("PNS masa kerja = %d/%q, want 15/tmt_cpns", got, source)
	}

	got, source = deriveMasaKerja("pppk", nil, &gol, asOf)
	if got != 4 || source != "tmt_gol" {
		t.Fatalf("PPPK fallback masa kerja = %d/%q, want 4/tmt_gol", got, source)
	}

	got, source = deriveMasaKerja("pppk", &cpns, &gol, asOf)
	if got != 15 || source != "tmt_cpns" {
		t.Fatalf("PPPK preferred masa kerja = %d/%q, want 15/tmt_cpns", got, source)
	}

	got, source = deriveMasaKerja("pns", nil, nil, asOf)
	if got != 0 || source != "belum_tersedia" {
		t.Fatalf("missing masa kerja = %d/%q, want 0/belum_tersedia", got, source)
	}
}

func TestParseBKNUnitNormalizesSchoolAndKorwil(t *testing.T) {
	got, err := parseBKNUnit("SDN 1 CANDISARI - Koordinator Wilayah Kecamatan Bidang Pendidikan Kecamatan Purwodadi - Dinas Pendidikan")
	if err != nil {
		t.Fatal(err)
	}
	if got.UnitName != "SDN 1 CANDISARI" || got.UnitType != "sd" || got.UnitCode != "SDN-1-CANDISARI" {
		t.Fatalf("unit = %#v", got)
	}
	if got.ParentUnitName != "KORWILCAM PURWODADI" || got.ParentUnitCode != "KORWILCAM-PURWODADI" {
		t.Fatalf("parent = %#v", got)
	}

	got, err = parseBKNUnit("SMPN 2 SATU ATAP KLAMBU KELOMPOK SEKOLAH MENENGAH PERTAMA NEGERI")
	if err != nil {
		t.Fatal(err)
	}
	if got.UnitType != "smp" || got.ParentUnitCode != "KORWILCAM-KLAMBU" {
		t.Fatalf("SMP fallback unit = %#v", got)
	}

	got, err = parseBKNUnit("SMPN 1 PURWODADI - DINAS PENDIDIKAN")
	if err != nil || got.ParentUnitCode != "KORWILCAM-PURWODADI" {
		t.Fatalf("SMP Dinas-only mapping = %#v, err=%v", got, err)
	}
}

func TestParseImportTeachersAcceptsActualBKNHeaders(t *testing.T) {
	rows := [][]string{
		{"NO", "NAMA", "NIP", "STATUS PEGAWAI", "TMT CPNS", "TMT PNS", "GOL.", "PANGKAT", "TMT GOL", "INSTANSI SUB UNIT"},
		{"1", "Guru PNS", "198001012010011001", "PNS", "01-01-2010", "01-01-2011", "III/b", "Penata Muda Tingkat I", "01-01-2024", "SDN 1 CANDISARI - KOORDINATOR WILAYAH KECAMATAN BIDANG PENDIDIKAN KECAMATAN PURWODADI - DINAS PENDIDIKAN"},
		{"2", "Guru PPPK", "199001012022011001", "PPPK", "30-11--0001", "30-11--0001", "IX", "Penata Muda", "01-03-2022", "SDN 2 CANDISARI - KOORDINATOR WILAYAH KECAMATAN BIDANG PENDIDIKAN KECAMATAN PURWODADI - DINAS PENDIDIKAN"},
	}
	got, err := parseImportTeachers(rows)
	if err != nil {
		t.Fatalf("parseImportTeachers() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ASNType != "pns" || got[0].MasaKerjaSource != "tmt_cpns" || got[0].UnitType != "sd" {
		t.Fatalf("PNS row = %#v", got[0])
	}
	if got[1].ASNType != "pppk" || got[1].MasaKerjaSource != "tmt_gol" || got[1].MasaKerjaTahun <= 0 {
		t.Fatalf("PPPK row = %#v", got[1])
	}
}

func TestParseBKNUnitMapsDinasInstitution(t *testing.T) {
	got, err := parseBKNUnit("DINAS PENDIDIKAN")
	if err != nil {
		t.Fatal(err)
	}
	if got.UnitCode != "DINAS-PENDIDIKAN" || got.UnitType != "dinas" || got.ParentUnitCode != "" {
		t.Fatalf("Dinas unit = %#v", got)
	}
}
