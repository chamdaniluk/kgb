package letterdata

import "time"

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

// NextDueTMT menagih TMT KGB berikutnya memakai satu rumus untuk seluruh
// sistem: anniversary dua tahunan pertama dari tmtAwal yang lebih baru dari
// SK terakhir (KGB terakhir atau SK naik pangkat). Usulan yang terlambat
// tidak pernah melompat ke siklus berikutnya — selama SK periode itu belum
// terbit, TMT yang ditagih tetap anniversary periode tersebut (contoh:
// baru diusulkan Januari 2027 tetap TMT 1 Des 2026). Bila tagihan tertinggal
// lebih dari satu siklus penuh (hari ini sudah melewati b+2 tahun), acuan
// digeser ke anniversary berjalan agar periode tidak menumpuk bolong.
func NextDueTMT(tmtAwal, skTerakhir, hariIni time.Time) time.Time {
	// NextPeriodicTMT inklusif (≥); geser satu hari agar SK terakhir yang
	// jatuh tepat di anniversary ikut tergantikan oleh siklus berikutnya.
	tetap := NextPeriodicTMT(tmtAwal, dateOnly(skTerakhir).AddDate(0, 0, 1))
	if hariIni.After(tetap.AddDate(2, 0, 0)) {
		return NextPeriodicTMT(tmtAwal, hariIni)
	}
	return tetap
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

// MasaKerjaFromTMT menghitung masa kerja golongan dari TMT awal (CPNS atau
// pengangkatan PPPK) sampai TMT KGB berlaku, dibulatkan ke bawah ke kelipatan
// dua tahun. Dipakai sebagai acuan penentu gaji.
func MasaKerjaFromTMT(tmtAwal, tmtBerlaku time.Time) int {
	return EvenYear(yearsBetween(tmtAwal, tmtBerlaku))
}

const (
	BucketBelumLengkap = "belum_lengkap"
	BucketMendatang    = "mendatang"
	BucketNominasi     = "nominasi" // 6–3 bulan sebelum TMT
	BucketSegera       = "segera"   // 3 bulan sampai TMT
	BucketTerlambat    = "terlambat"
)

// NominationBucket mengelompokkan ASN berdasarkan jarak ke TMT KGB berikutnya.
// Jendela nominasi: 6 sampai 3 bulan sebelum TMT. Tiga bulan terakhir untuk
// usul perubahan gaji. Lewat TMT tetap bisa usul (terlambat).
func NominationBucket(nextTMT, asOf time.Time) string {
	if nextTMT.IsZero() {
		return BucketBelumLengkap
	}
	nextTMT = dateOnly(nextTMT)
	asOf = dateOnly(asOf)
	if nextTMT.Before(asOf) {
		return BucketTerlambat
	}
	if !nextTMT.After(asOf.AddDate(0, 3, 0)) {
		return BucketSegera
	}
	if !nextTMT.After(asOf.AddDate(0, 6, 0)) {
		return BucketNominasi
	}
	return BucketMendatang
}
