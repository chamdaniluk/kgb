package letterdata

import (
	"testing"
	"time"
)

func d(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNextPeriodicTMTDariSKPertamaKeAnniversaryBerikutnya(t *testing.T) {
	// SK pertama 1 Des 2020, usul di Agustus 2026 → mulai berlaku 1 Des 2026.
	got := NextPeriodicTMT(d("2020-12-01"), d("2026-08-17"))
	if !got.Equal(d("2026-12-01")) {
		t.Fatalf("TMT = %s, ingin 2026-12-01", got.Format("2006-01-02"))
	}
}

func TestNextPeriodicTMTTidakMundurJikaSudahLewatAnniversary(t *testing.T) {
	// Setelah 1 Des 2026 lewat, jadwal berikutnya 1 Des 2028.
	got := NextPeriodicTMT(d("2020-12-01"), d("2026-12-02"))
	if !got.Equal(d("2028-12-01")) {
		t.Fatalf("TMT = %s, ingin 2028-12-01", got.Format("2006-01-02"))
	}
}

func TestNextPeriodicTMTDariKGBTerakhirPlusDuaTahun(t *testing.T) {
	got := NextPeriodicTMT(d("2024-12-01"), d("2026-08-17"))
	if !got.Equal(d("2026-12-01")) {
		t.Fatalf("TMT = %s, ingin 2026-12-01", got.Format("2006-01-02"))
	}
}

func TestPeriodicMasaKerjaGenapNolBulan(t *testing.T) {
	// SK pertama 1 Des 2020 → TMT 1 Des 2026, masa kerja baru 6 th 0 bln.
	s := ComputePeriodicMasaKerja(PeriodicInput{
		LastTMT:     d("2020-12-01"),
		NewTMT:      d("2026-12-01"),
		LastMKGYear: 0,
	})
	if s.LamaTahun != 4 || s.LamaBulan != 0 || s.BaruTahun != 6 || s.BaruBulan != 0 {
		t.Fatalf("mkg = %d/%d → %d/%d, ingin 4/0 → 6/0", s.LamaTahun, s.LamaBulan, s.BaruTahun, s.BaruBulan)
	}
}

func TestPeriodicMasaKerjaDariKGBTerakhir(t *testing.T) {
	s := ComputePeriodicMasaKerja(PeriodicInput{
		LastTMT:     d("2024-12-01"),
		NewTMT:      d("2026-12-01"),
		LastMKGYear: 4,
	})
	if s.LamaTahun != 4 || s.BaruTahun != 6 || s.LamaBulan != 0 || s.BaruBulan != 0 {
		t.Fatalf("mkg = %d/%d → %d/%d, ingin 4/0 → 6/0", s.LamaTahun, s.LamaBulan, s.BaruTahun, s.BaruBulan)
	}
}

func TestEvenYearFloor(t *testing.T) {
	if EvenYear(5) != 4 || EvenYear(6) != 6 || EvenYear(0) != 0 || EvenYear(-1) != 0 {
		t.Fatalf("EvenYear salah: 5=%d 6=%d 0=%d -1=%d", EvenYear(5), EvenYear(6), EvenYear(0), EvenYear(-1))
	}
}

func TestIsPeriodicTMT(t *testing.T) {
	if !IsPeriodicTMT(d("2020-12-01"), d("2026-12-01")) {
		t.Fatal("1 Des 2020 → 1 Des 2026 harus genap")
	}
	if IsPeriodicTMT(d("2020-12-01"), d("2026-09-01")) {
		t.Fatal("1 Sep 2026 bukan anniversary 1 Des")
	}
	if IsPeriodicTMT(d("2020-12-01"), d("2021-12-01")) {
		t.Fatal("jarak 1 tahun tidak genap")
	}
}
