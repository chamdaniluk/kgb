package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncSourceSIPPASN menandai baris teachers yang terakhir diperbarui dari SIPP ASN.
const SyncSourceSIPPASN = "sippasn"

// SIPPASNSyncSystemActor mencari akun admin pertama sebagai aktor audit
// sinkron terjadwal; 0 bila belum ada (kolom imported_by memakai 0 = sistem).
func SIPPASNSyncSystemActor(pool *pgxpool.Pool) int64 {
	var id int64
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE role IN ('admin','admin_dinas') AND is_active ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		return 0
	}
	return id
}

// SyncResult merangkum satu kali jalan sinkronisasi ASN dari SIPP ASN.
type SyncResult struct {
	RowsTotal   int      `json:"rows_total"`
	RowsCreated int      `json:"rows_created"`
	RowsUpdated int      `json:"rows_updated"`
	RowsSkipped int      `json:"rows_skipped"`
	Notes       []string `json:"notes,omitempty"`
}

// SIPPASNSyncConfig mengatur perilaku sinkronisasi.
type SIPPASNSyncConfig struct {
	// HanyaUnitPendidikan membatasi sinkron pada baris unit kerja yang
	// mengandung "dinas pendidikan" (piloting guru Dinas Pendidikan).
	HanyaUnitPendidikan bool
	// HanyaStatusAktif melewatkan baris dengan status selain "1".
	HanyaStatusAktif bool
	// BatasiJumlah membatasi jumlah baris sumber yang diproses (0 = semua);
	// berguna untuk uji coba bertahap.
	BatasiJumlah int
}

// Kategori pegawai Dinas Pendidikan: fungsional guru atau non-guru
// (pelaksana, pengawas sekolah, penilik, pamong belajar, struktural).
const (
	KategoriGuru    = "guru"
	KategoriNonGuru = "non_guru"
)

// ASNTypeFromNIP menurunkan jenis ASN dari segmen bulan NIP (digit 13-14):
// 01-12 = PNS (bulan lahir), 21-22 = PPPK. SIPPASN tidak menyediakan kolom
// jenis ASN, tetapi pola ini konsisten di seluruh 15.463 baris (probing
// 2026-09-03: non-21/22 selalu bergolongan; bulk kosong-golongan = 2025+21).
func ASNTypeFromNIP(nip string) (string, bool) {
	if len(nip) != 18 {
		return "", false
	}
	seg := nip[12:14]
	if seg >= "01" && seg <= "12" {
		return "pns", true
	}
	if seg == "21" || seg == "22" {
		return "pppk", true
	}
	return "", false
}

// KategoriFromJabatan menentukan guru vs non-guru dari jenis jabatan dan
// nama jabatan SIPPASN.
func KategoriFromJabatan(jobKind, jobTitle string) string {
	if jobKind == "2" && strings.Contains(strings.ToLower(jobTitle), "guru") {
		return KategoriGuru
	}
	return KategoriNonGuru
}

// MapSIPPASNOfficer mengubah satu baris SIPP ASN menjadi ImportedTeacher.
// SIPPASN adalah sumber utama data induk (keputusan owner 2026-09-03):
// seluruh pegawai aktif Dinas Pendidikan (guru + non-guru) layak sinkron.
//
// Aturan pemetaan (hasil probing endpoint /api/pegawai):
//   - NIP 18 digit numerik wajib; jenis ASN dari segmen bulan NIP.
//   - PNS: golongan SIPPASN dipakai apa adanya.
//   - PPPK bergolongan: golongan SIPPASN dipakai (bukan dipaksa IX — data
//     menunjukkan PPPK memakai II/a, II/c, III/a, III/b sesuai jenjang).
//   - PPPK tanpa golongan (angkatan baru): golongan "IX" hanya bila jabatan
//     fungsional guru; non-guru tanpa golongan dilewat (golongan tidak
//     dapat ditebak) dan menunggu penetapan SIPPASN.
func MapSIPPASNOfficer(o SIPPASNOfficer, cfg SIPPASNSyncConfig) (ImportedTeacher, bool) {
	if len(o.NIP) != 18 {
		return ImportedTeacher{}, false
	}
	for _, r := range o.NIP {
		if r < '0' || r > '9' {
			return ImportedTeacher{}, false
		}
	}
	if strings.TrimSpace(o.Name) == "" {
		return ImportedTeacher{}, false
	}
	if cfg.HanyaStatusAktif && strings.TrimSpace(o.Status) != "1" {
		return ImportedTeacher{}, false
	}
	if cfg.HanyaUnitPendidikan && !strings.Contains(strings.ToLower(o.UnitName), "dinas pendidikan") {
		return ImportedTeacher{}, false
	}
	asnType, ok := ASNTypeFromNIP(o.NIP)
	if !ok {
		return ImportedTeacher{}, false
	}
	kategori := KategoriFromJabatan(o.JobKind, o.JobTitle)
	golongan := strings.TrimSpace(o.Golongan)
	mkgSource := "sippasn"
	switch {
	case golongan != "" && asnType == "pns":
		// PNS: golongan SIPPASN fix (I/a-IV/e), tidak dapat diubah.
	case golongan != "" && asnType == "pppk" && ValidPPPKGolongan(golongan):
		// PPPK dengan golongan resmi (I-XVII): dipakai apa adanya.
	case asnType == "pppk":
		// PPPK tercatat dengan padanan PNS (mis. III/a) atau belum
		// bergolongan: default IX, dapat diubah saat usul KGB sesuai SK.
		golongan = "IX"
	default:
		return ImportedTeacher{}, false
	}
	mkg := 0
	if tmt, ok := tmtFromNIP(o.NIP); ok {
		mkg = fullYearsSince(tmt, time.Now())
	} else {
		mkgSource = "belum_tersedia"
	}
	unitCode, unitName, unitType, unitDistrict, kdUnker := sippASNUnitInfo(o)
	return ImportedTeacher{
		NIP:             o.NIP,
		Name:            strings.TrimSpace(o.Name),
		ASNType:         asnType,
		Kategori:        kategori,
		UnitCode:        unitCode,
		UnitName:        unitName,
		UnitType:        unitType,
		UnitDistrict:    unitDistrict,
		KdUnker:         kdUnker,
		PangkatGol:      golongan,
		Pangkat:         strings.TrimSpace(o.Pangkat),
		Jabatan:         strings.TrimSpace(o.JobTitle),
		MasaKerjaTahun:  mkg,
		MasaKerjaSource: mkgSource,
	}, true
}

// tmtFromNIP membaca perkiraan TMT dari 8 digit awal NIP (YYYYMMDD tanggal lahir)
// ditambah 4 digit tahun pengangkatan: NIP[8:12]. Bila tidak valid, ok=false
// dan pemanggil memakai sumber "belum_tersedia".
func tmtFromNIP(nip string) (tmt time.Time, ok bool) {
	if len(nip) != 18 {
		return time.Time{}, false
	}
	year := nip[8:12]
	tmt, err := time.Parse("2006", year)
	if err != nil || tmt.Year() < 1980 || tmt.After(time.Now()) {
		return time.Time{}, false
	}
	return tmt, true
}

func fullYearsSince(from, asOf time.Time) int {
	years := asOf.Year() - from.Year()
	if years < 0 {
		return 0
	}
	return years
}

// sippASNUnitInfo menurunkan identitas unit dari satu baris SIPP ASN:
// kode unit, nama, tipe, kecamatan, dan kd_unker. Dipakai MapSIPPASNOfficer dan
// perbaikan identitas unit agar keduanya memakai aturan yang sama persis.
func sippASNUnitInfo(o SIPPASNOfficer) (code, name, unitType, district, kdUnker string) {
	name = strings.TrimSpace(o.UnitName)
	kdUnker = strings.TrimSpace(o.UnitCode)
	unitType = sippASNUnitType(name)
	code = sippASNUnitCode(name, unitType, kdUnker)
	district = sippASNDistrict(name, unitType, kdUnker)
	return code, name, unitType, district, kdUnker
}

// sippASNUnitCode menurunkan kode unit gaya SI CENDIKIA dari nama unit SIPP ASN.
// Satuan pendidikan (sd/tk/smp/skb) mendapat akhiran kd_unker yang unik, sehingga
// sekolah bernama sama di kecamatan berbeda tidak lagi saling menimpa:
// "SDN 1 Karanganyar" (kd_unker 03.21.02) -> "SDN-1-KARANGANYAR-03-21-02".
// Unit Korwil/Dinas tetap memakai kode berbasis nama karena namanya sudah memuat
// kecamatan/bidang sehingga tidak pernah bentrok.
func sippASNUnitCode(unitName, unitType, kdUnker string) string {
	code := sippASNLegacyUnitCode(unitName)
	if kdUnker == "" || !isSchoolUnitType(unitType) {
		return code
	}
	suffix := dashifyKdUnker(kdUnker)
	if suffix == "" {
		return code
	}
	return code + "-" + suffix
}

// isSchoolUnitType menandai unit yang mewakili satu satuan pendidikan sehingga
// kd_unker-nya dipakai sebagai akhiran kode.
func isSchoolUnitType(unitType string) bool {
	switch unitType {
	case "sd", "tk", "smp", "skb":
		return true
	}
	return false
}

// dashifyKdUnker mengubah kd_unker "03.21.02" menjadi "03-21-02" agar aman
// dipakai sebagai bagian kode unit.
func dashifyKdUnker(kdUnker string) string {
	var out strings.Builder
	prevDash := false
	for _, r := range strings.TrimSpace(kdUnker) {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			out.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && out.Len() > 0 {
				out.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(out.String(), "-")
}

// sippASNLegacyUnitCode adalah kode lama (berbasis nama saja) yang masih dipakai
// untuk mencocokkan unit lama saat perbaikan identitas, sebelum kd_unker ada.
func sippASNLegacyUnitCode(unitName string) string {
	name := unitName
	if i := strings.Index(name, " - "); i >= 0 {
		name = name[:i]
	}
	name = strings.ToUpper(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "SMPN ", "SMP-NEGERI-")
	var out strings.Builder
	prevDash := false
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				out.WriteByte('-')
				prevDash = true
			}
		}
	}
	code := strings.Trim(out.String(), "-")
	for strings.Contains(code, "--") {
		code = strings.ReplaceAll(code, "--", "-")
	}
	if code == "" {
		return "SIPPASN-UNIT"
	}
	return code
}

// sippASNUnitType menebak tipe unit SI CENDIKIA dari nama unit SIPP ASN.
func sippASNUnitType(unitName string) string {
	upper := strings.ToUpper(strings.TrimSpace(unitName))
	switch {
	case strings.Contains(upper, "KOORDINATOR WILAYAH"):
		return "korwil"
	// SKB/SPNF memiliki unit verifikasi sendiri (migrasi 018); cek sebelum
	// "DINAS PENDIDIKAN" agar tidak tertelan klasifikasi unit internal Dinas.
	case strings.Contains(upper, "SKB") || strings.Contains(upper, "SPNF") || strings.Contains(upper, "SANGGAR KEGIATAN BELAJAR"):
		return "skb"
	// TK juga perlu dicek sebelum "DINAS PENDIDIKAN": nama TK berakhiran
	// "- Dinas Pendidikan" sehingga tanpa cabang ini 19 TK ikut tertelan jadi
	// unit Dinas dan usulannya salah masuk antrean Dinas, bukan Korwil.
	// Semua TK SIPP ASN berawalan "TK "/"TAMAN KANAK" (probe 2026-09-14).
	case strings.HasPrefix(upper, "TK ") || strings.HasPrefix(upper, "TAMAN KANAK"):
		return "tk"
	case strings.Contains(upper, "DINAS PENDIDIKAN") && !strings.Contains(upper, "SDN ") && !strings.Contains(upper, "SMP"):
		return "dinas"
	case strings.Contains(upper, "SMP"):
		return "smp"
	default:
		return "sd"
	}
}

// sippASNDistricts adalah 19 kecamatan resmi Grobogan untuk tebakan kecamatan
// dari nama unit SIPP ASN.
var sippASNDistricts = []string{
	"BRATI", "GABUS", "GEYER", "GODONG", "GROBOGAN", "GUBUG",
	"KARANGRAYUNG", "KEDUNGJATI", "KLAMBU", "KRADENAN", "NGARINGAN",
	"PENAWANGAN", "PULOKULON", "PURWODADI", "TANGGUNGHARJO", "TAWANGHARJO",
	"TEGOWANU", "TOROH", "WIROSARI",
}

// sippASNKecamatanKode memetakan kode kecamatan Dinas Pendidikan pada kd_unker
// (segmen "03.NN") ke nama kecamatan resmi. Diturunkan dari unit Korwil SIPP ASN
// (probe 2026-09-14) dan dipakai sebagai sumber kecamatan yang pasti.
var sippASNKecamatanKode = map[string]string{
	"03.07": "PURWODADI",
	"03.08": "TOROH",
	"03.09": "GEYER",
	"03.10": "GROBOGAN",
	"03.11": "BRATI",
	"03.12": "KLAMBU",
	"03.13": "WIROSARI",
	"03.14": "TAWANGHARJO",
	"03.15": "NGARINGAN",
	"03.16": "KRADENAN",
	"03.17": "GABUS",
	"03.18": "PULOKULON",
	"03.19": "GODONG",
	"03.20": "PENAWANGAN",
	"03.21": "KARANGRAYUNG",
	"03.22": "GUBUG",
	"03.23": "KEDUNGJATI",
	"03.24": "TEGOWANU",
	"03.25": "TANGGUNGHARJO",
}

// sippASNDistrict menentukan kecamatan unit. Bila kd_unker tersedia, kecamatan
// dibaca dari kode (03.NN) sehingga sekolah bernama desa yang sama tidak lagi
// salah kecamatan (mis. SDN 1-4 Tanggungharjo ber-kd_unker 03.10.x = Kec.
// Grobogan, bukan Kec. Tanggungharjo). Bila kode tak ada, jatuh ke tebakan nama
// (dipakai impor BKN manual). SKB berkedudukan di Kec. Purwodadi (migrasi 018).
func sippASNDistrict(unitName, unitType, kdUnker string) string {
	if unitType == "skb" {
		return "PURWODADI"
	}
	if d := sippASNDistrictFromKode(kdUnker); d != "" {
		return d
	}
	return sippASNDistrictFromName(unitName, unitType)
}

// sippASNDistrictFromKode memetakan kode kecamatan Dinas Pendidikan (segmen
// kedua kd_unker, mis. "03.21" = Karangrayung) ke nama kecamatan resmi.
func sippASNDistrictFromKode(kdUnker string) string {
	parts := strings.Split(strings.TrimSpace(kdUnker), ".")
	if len(parts) < 2 || parts[0] != "03" {
		return ""
	}
	if name, ok := sippASNKecamatanKode[parts[0]+"."+parts[1]]; ok {
		return name
	}
	return ""
}

// sippASNDistrictFromName adalah tebakan kecamatan dari nama unit (fallback
// dipakai bila kd_unker tidak ada, mis. impor BKN manual).
func sippASNDistrictFromName(unitName, unitType string) string {
	upper := strings.ToUpper(strings.TrimSpace(unitName))
	if unitType == "dinas" || unitType == "korwil" {
		for _, d := range sippASNDistricts {
			if strings.Contains(upper, d) {
				if unitType == "korwil" {
					return d
				}
				break
			}
		}
		if unitType == "korwil" {
			return ""
		}
		return "DINAS"
	}
	for _, d := range sippASNDistricts {
		if strings.Contains(upper, " "+d) || strings.HasSuffix(upper, " "+d) || strings.Contains(upper, "KECAMATAN "+d) {
			return d
		}
	}
	return ""
}

// ImportSIPPASNOfficers memasukkan baris SIPP ASN yang sudah dipetakan ke master
// memakai UpsertImportedTeacher yang sama dengan impor BKN, lalu menandai
// sumbernya dan mencatat ringkasan ke bkn_imports + audit_logs.
func ImportSIPPASNOfficers(ctx context.Context, pool *pgxpool.Pool, actorID int64, officers []SIPPASNOfficer, cfg SIPPASNSyncConfig, hashPassword func(string) (string, error)) (SyncResult, error) {
	result := SyncResult{Notes: make([]string, 0)}
	mapped := make([]ImportedTeacher, 0, len(officers))
	for i, o := range officers {
		if cfg.BatasiJumlah > 0 && len(mapped) >= cfg.BatasiJumlah {
			break
		}
		t, ok := MapSIPPASNOfficer(o, cfg)
		if !ok {
			result.RowsSkipped++
			continue
		}
		if err := ValidateImportedTeacher(t); err != nil {
			result.RowsSkipped++
			result.Notes = append(result.Notes, fmt.Sprintf("baris %d NIP %s: %v", i+1, t.NIP, err))
			continue
		}
		mapped = append(mapped, t)
	}
	result.RowsTotal = len(mapped) + result.RowsSkipped
	if len(mapped) == 0 {
		return result, nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return SyncResult{}, err
	}
	defer tx.Rollback(ctx)
	nips := make([]string, 0, len(mapped))
	for i, t := range mapped {
		created, err := UpsertImportedTeacher(ctx, tx, t, hashPassword)
		if err != nil {
			return SyncResult{}, fmt.Errorf("baris %d NIP %s: %w", i+1, t.NIP, err)
		}
		if created {
			result.RowsCreated++
		} else {
			result.RowsUpdated++
		}
		nips = append(nips, t.NIP)
	}
	if _, err := tx.Exec(ctx, `UPDATE teachers SET masa_kerja_source=$1, updated_at=now() WHERE nip = ANY($2) AND masa_kerja_source <> 'kgb_terbit'`, SyncSourceSIPPASN, nips); err != nil {
		return SyncResult{}, fmt.Errorf("tandai sumber sinkron: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO bkn_imports (file_name, imported_by, rows_total, rows_created, rows_updated, rows_skipped, notes, asal_data) VALUES ($1,$2,$3,$4,$5,$6,$7,'sippasn')`, "sippasn:sync", actorID, result.RowsTotal, result.RowsCreated, result.RowsUpdated, result.RowsSkipped, nullIfEmpty(joinNotes(result.Notes))); err != nil {
		return SyncResult{}, err
	}
	details, err := json.Marshal(map[string]any{
		"sumber":       "sippasn",
		"rows_total":   result.RowsTotal,
		"rows_created": result.RowsCreated,
		"rows_updated": result.RowsUpdated,
		"rows_skip":    result.RowsSkipped,
	})
	if err != nil {
		return SyncResult{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_user_id,action,details) VALUES ($1,'sinkron_sippasn',$2)`, actorID, details); err != nil {
		return SyncResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SyncResult{}, err
	}
	return result, nil
}

// ListSyncHistory mengambil riwayat sinkronisasi SIPP ASN terbaru.
func ListSyncHistory(ctx context.Context, q Querier, limit int) ([]BKNImport, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := q.Query(ctx, `SELECT id, file_name, imported_by, rows_total, rows_created, rows_updated, rows_skipped, notes, imported_at FROM bkn_imports WHERE file_name LIKE 'sippasn:%' ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]BKNImport, 0)
	for rows.Next() {
		var item BKNImport
		if err := rows.Scan(&item.ID, &item.FileName, &item.ImportedBy, &item.RowsTotal, &item.RowsCreated, &item.RowsUpdated, &item.RowsSkipped, &item.Notes, &item.ImportedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// BKNImport adalah satu baris riwayat impor (dipakai ulang untuk sinkron).
type BKNImport struct {
	ID          int64     `json:"id"`
	FileName    string    `json:"file_name"`
	ImportedBy  int64     `json:"imported_by"`
	RowsTotal   int       `json:"rows_total"`
	RowsCreated int       `json:"rows_created"`
	RowsUpdated int       `json:"rows_updated"`
	RowsSkipped int       `json:"rows_skipped"`
	Notes       *string   `json:"notes,omitempty"`
	ImportedAt  time.Time `json:"imported_at"`
}
