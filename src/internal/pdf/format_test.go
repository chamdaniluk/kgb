package pdf

import (
	"testing"
	"time"
)

func TestFormatRupiah(t *testing.T) {
	if got := FormatRupiah("3089300"); got != "Rp. 3.089.300,00" {
		t.Fatalf("FormatRupiah = %q", got)
	}
}

func TestFormatNIP(t *testing.T) {
	if got := FormatNIP("199002132020121010"); got != "19900213 202012 1 010" {
		t.Fatalf("FormatNIP = %q", got)
	}
}

func TestFormatTanggalID(t *testing.T) {
	tm := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	if got := FormatTanggalID(tm); got != "17 Agustus 2026" {
		t.Fatalf("FormatTanggalID = %q", got)
	}
}

func TestFormatMasaKerja(t *testing.T) {
	if got := FormatMasaKerja(5, 0); got != "05 Tahun 00 Bulan" {
		t.Fatalf("FormatMasaKerja = %q", got)
	}
}
