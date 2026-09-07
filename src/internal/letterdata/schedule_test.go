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

func TestNextDueTMTEpatWaktuDariSKPertama(t *testing.T) {
	// SK pengangkatan/KGB pertama 1 Des 2020, usul Agustus 2026 → 1 Des 2026.
	got := NextDueTMT(d("2020-12-01"), d("2020-12-01"), d("2026-08-17"))
	if !got.Equal(d("2026-12-01")) {
		t.Fatalf("TMT = %s, ingin 2026-12-01", got.Format("2006-01-02"))
	}
}

func TestNextDueTMTTelatSatuBulanTetapDiPeriode(t *testing.T) {
	// Lupa usul sampai Januari 2027 → tetap 1 Des 2026, TIDAK melompat ke 2028.
	got := NextDueTMT(d("2020-12-01"), d("2024-12-01"), d("2027-01-05"))
	if !got.Equal(d("2026-12-01")) {
		t.Fatalf("TMT = %s, ingin 2026-12-01", got.Format("2006-01-02"))
	}
}

func TestNextDueTMTSetelahTerbitNaikSiklus(t *testing.T) {
	// SK KGB 1 Des 2026 sudah terbit → tagihan berikutnya 1 Des 2028.
	got := NextDueTMT(d("2020-12-01"), d("2026-12-01"), d("2027-01-05"))
	if !got.Equal(d("2028-12-01")) {
		t.Fatalf("TMT = %s, ingin 2028-12-01", got.Format("2006-01-02"))
	}
}

func TestNextDueTMTSKNaikPangkatOffGrid(t *testing.T) {
	// SK naik pangkat 1 Jul 2026 tidak menggeser jadwal berkala.
	got := NextDueTMT(d("2020-12-01"), d("2026-07-01"), d("2026-08-17"))
	if !got.Equal(d("2026-12-01")) {
		t.Fatalf("TMT = %s, ingin 2026-12-01", got.Format("2006-01-02"))
	}
}

func TestNextDueTMTStaleDuaSiklusIkutBerjalan(t *testing.T) {
	// Tagihan 2022 dibiarkan sampai Juni 2028 (lewat satu siklus penuh):
	// ekspektasi bergeser ke anniversary berjalan berikutnya, 1 Des 2028.
	got := NextDueTMT(d("2020-12-01"), d("2020-12-01"), d("2028-06-01"))
	if !got.Equal(d("2028-12-01")) {
		t.Fatalf("TMT = %s, ingin 2028-12-01", got.Format("2006-01-02"))
	}
}

func TestEvenYearFloor(t *testing.T) {
	if EvenYear(5) != 4 || EvenYear(6) != 6 || EvenYear(0) != 0 || EvenYear(-1) != 0 {
		t.Fatalf("EvenYear salah: 5=%d 6=%d 0=%d -1=%d", EvenYear(5), EvenYear(6), EvenYear(0), EvenYear(-1))
	}
}

func TestMasaKerjaFromTMTGenapDariTMTCPNS(t *testing.T) {
	// PNS TMT CPNS 1 Jan 2020 → TMT berlaku 1 Jan 2026 = 6 tahun genap.
	if got := MasaKerjaFromTMT(d("2020-01-01"), d("2026-01-01")); got != 6 {
		t.Fatalf("masa kerja = %d, ingin 6", got)
	}
}

func TestMasaKerjaFromTMTTahunGanjilDibulatkanKeBawah(t *testing.T) {
	// TMT CPNS 1 Jun 2021 → TMT berlaku 1 Des 2026 = 5 tahun aktual → 4 (genap).
	if got := MasaKerjaFromTMT(d("2021-06-01"), d("2026-12-01")); got != 4 {
		t.Fatalf("masa kerja = %d, ingin 4", got)
	}
}

func TestMasaKerjaFromTMTAwalSetelahBerlakuJadiNol(t *testing.T) {
	// TMT awal setelah TMT berlaku tidak boleh menghasilkan masa kerja negatif.
	if got := MasaKerjaFromTMT(d("2027-01-01"), d("2026-01-01")); got != 0 {
		t.Fatalf("masa kerja = %d, ingin 0", got)
	}
}

func TestNominationBucketJendelaEnamSampaiTigaBulan(t *testing.T) {
	// TMT 1 Des 2026, hari ini 17 Agu 2026 ≈ 3,5 bulan → nominasi.
	if got := NominationBucket(d("2026-12-01"), d("2026-08-17")); got != BucketNominasi {
		t.Fatalf("bucket = %s, ingin nominasi", got)
	}
}

func TestNominationBucketTigaBulanTerakhir(t *testing.T) {
	if got := NominationBucket(d("2026-12-01"), d("2026-10-01")); got != BucketSegera {
		t.Fatalf("bucket = %s, ingin segera", got)
	}
}

func TestNominationBucketTerlambatTetapBisaUsul(t *testing.T) {
	if got := NominationBucket(d("2026-06-01"), d("2026-08-17")); got != BucketTerlambat {
		t.Fatalf("bucket = %s, ingin terlambat", got)
	}
}

func TestNominationBucketMendatangDanKosong(t *testing.T) {
	if got := NominationBucket(d("2028-12-01"), d("2026-08-17")); got != BucketMendatang {
		t.Fatalf("bucket = %s, ingin mendatang", got)
	}
	if got := NominationBucket(time.Time{}, d("2026-08-17")); got != BucketBelumLengkap {
		t.Fatalf("bucket kosong = %s", got)
	}
}

// Peninjauan masa kerja: MKG SK sebelumnya + 2, bukan dari TMT CPNS.
func TestMasaKerjaKGBMemakaiSKSebelumnya(t *testing.T) {
	delapan := 8
	// TMT CPNS Des 2020 + peninjauan 4 th → SK sebelumnya MKG 8.
	// KGB 2026: 8 + 2 = 10 (bukan 6 dari TMT).
	got := MasaKerjaKGB(&delapan, time.Date(2020, 12, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))
	if got != 10 {
		t.Errorf("MKG peninjauan = %d, want 10", got)
	}
	// Tanpa SK: fallback hitung TMT (6).
	got = MasaKerjaKGB(nil, time.Date(2020, 12, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))
	if got != 6 {
		t.Errorf("MKG fallback = %d, want 6", got)
	}
}
