package store

// Page menyimpan parameter pagination yang sudah dinormalkan.
// Limit == 0 berarti "semua baris" (tanpa LIMIT/OFFSET di SQL).
type Page struct {
	Limit  int
	Offset int
}

// allowedPageSizes adalah pilihan baris per halaman yang sah. 0 == semua.
var allowedPageSizes = map[int]bool{0: true, 20: true, 50: true, 100: true}

// NewPage menormalkan limit/offset dari input mentah.
// Limit di luar {0,20,50,100} dipaksa ke default 20; offset negatif jadi 0.
func NewPage(limit, offset int) Page {
	if !allowedPageSizes[limit] {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return Page{Limit: limit, Offset: offset}
}

// clause mengembalikan potongan SQL LIMIT/OFFSET dan argumennya mulai dari $start.
// Untuk limit==0 (semua), tidak ada klausa yang ditambahkan.
func (p Page) clause(start int) (string, []any) {
	if p.Limit == 0 {
		return "", nil
	}
	return " LIMIT $" + itoa(start) + " OFFSET $" + itoa(start+1), []any{p.Limit, p.Offset}
}

// slice memotong hasil in-memory sesuai halaman (dipakai saat filtering
// dilakukan di Go, mis. nominasi). Mengembalikan potongan untuk halaman ini.
func (p Page) slice(n int) (lo, hi int) {
	if p.Limit == 0 {
		return 0, n
	}
	lo = p.Offset
	if lo > n {
		lo = n
	}
	hi = lo + p.Limit
	if hi > n {
		hi = n
	}
	return lo, hi
}
