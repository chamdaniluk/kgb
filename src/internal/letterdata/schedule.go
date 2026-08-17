package letterdata

import "time"

// PeriodicInput adalah data yang dipakai menghitung TMT dan MKG berkala.
type PeriodicInput struct {
	LastTMT     time.Time
	NewTMT      time.Time
	LastMKGYear int
}

// PeriodicMasaKerja adalah masa kerja lama/baru yang selalu genap, 0 bulan.
type PeriodicMasaKerja struct {
	LamaTahun int
	LamaBulan int
	BaruTahun int
	BaruBulan int
}

// EvenYear membulatkan ke bawah ke kelipatan 2 tahun. Nilai negatif jadi 0.
func EvenYear(v int) int {
	if v < 0 {
		return 0
	}
	return v - (v % 2)
}

// NextPeriodicTMT mengembalikan anniversary dua tahunan berikutnya dari TMT
// SK/KGB terakhir. Usulan di bulan apa pun tidak menggeser tanggal berlaku.
func NextPeriodicTMT(lastTMT, asOf time.Time) time.Time {
	lastTMT = dateOnly(lastTMT)
	asOf = dateOnly(asOf)
	next := lastTMT.AddDate(2, 0, 0)
	for next.Before(asOf) {
		next = next.AddDate(2, 0, 0)
	}
	return next
}

// ComputePeriodicMasaKerja menghasilkan masa kerja lama = MKG SK terakhir
// (genap) dan masa kerja baru = lama + 2, keduanya 0 bulan.
func ComputePeriodicMasaKerja(in PeriodicInput) PeriodicMasaKerja {
	lama := EvenYear(in.LastMKGYear)
	if lama == 0 && !in.LastTMT.IsZero() && !in.NewTMT.IsZero() {
		years := yearsBetween(in.LastTMT, in.NewTMT) - 2
		if years < 0 {
			years = 0
		}
		lama = EvenYear(years)
	}
	return PeriodicMasaKerja{
		LamaTahun: lama,
		LamaBulan: 0,
		BaruTahun: lama + 2,
		BaruBulan: 0,
	}
}

func yearsBetween(start, end time.Time) int {
	start = dateOnly(start)
	end = dateOnly(end)
	if end.Before(start) {
		return 0
	}
	years := end.Year() - start.Year()
	anniv := start.AddDate(years, 0, 0)
	if anniv.After(end) {
		years--
	}
	if years < 0 {
		return 0
	}
	return years
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// IsPeriodicTMT benar bila proposed adalah anniversary +2n tahun dari last.
func IsPeriodicTMT(last, proposed time.Time) bool {
	last = dateOnly(last)
	proposed = dateOnly(proposed)
	if !proposed.After(last) {
		return false
	}
	cand := last.AddDate(2, 0, 0)
	for !cand.After(proposed) {
		if cand.Equal(proposed) {
			return true
		}
		cand = cand.AddDate(2, 0, 0)
	}
	return false
}
