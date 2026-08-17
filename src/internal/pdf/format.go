package pdf

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

var bulanID = [...]string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

func FormatTanggalID(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02") + " " + bulanID[t.Month()] + " " + t.Format("2006")
}

func FormatRupiah(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, ".", "")
	v = strings.ReplaceAll(v, ",", "")
	v = strings.ReplaceAll(v, "Rp", "")
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return "Rp. " + v + ",00"
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return "Rp. " + b.String() + ",00"
}

func FormatNIP(nip string) string {
	digits := make([]rune, 0, 18)
	for _, r := range nip {
		if unicode.IsDigit(r) {
			digits = append(digits, r)
		}
	}
	if len(digits) != 18 {
		return strings.TrimSpace(nip)
	}
	s := string(digits)
	return s[:8] + " " + s[8:14] + " " + s[14:15] + " " + s[15:]
}

func FormatMasaKerja(tahun, bulan int) string {
	return pad2(tahun) + " Tahun " + pad2(bulan) + " Bulan"
}

func pad2(v int) string {
	if v < 0 {
		v = 0
	}
	if v < 10 {
		return "0" + strconv.Itoa(v)
	}
	return strconv.Itoa(v)
}

func TodayID() string { return FormatTanggalID(time.Now()) }
