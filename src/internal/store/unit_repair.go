package store

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// unitRow adalah satu baris units beserta jumlah pemakainya (teachers, users,
// submissions) yang dipakai perhitungan perbaikan identitas.
type unitRow struct {
	id       int64
	code     string
	name     string
	typ      string
	district string
	kd       string
	used     int
}

// UnitRepairPlan merangkum rencana/ hasil perbaikan identitas unit dari SIPP ASN.
type UnitRepairPlan struct {
	Planned    int      `json:"planned"`
	Reassigned int      `json:"reassigned"` // kode unit dikoreksi ke bentuk berbasis kd_unker
	Districted int      `json:"districted"` // kecamatan dikoreksi dari kode
	Inserted   int      `json:"inserted"`   // unit baru (sekolah yang sebelumnya hilang)
	Merged     int      `json:"merged"`     // unit duplikat yang dilebur
	Unmapped   int      `json:"unmapped"`   // unit yang tidak ketemu padanan SIPP ASN
	Notes      []string `json:"notes,omitempty"`
}

// RepairUnitIdentity memperbaiki identitas unit memakai kd_unker dari SIPP ASN
// sebagai kunci kanonik. Idempoten: jalan berulang tidak mengubah apa pun lagi.
//
// Langkah per unit SIPP ASN (sd/tk/smp/skb, bukan korwil/dinas):
//  1. Cocokkan ke baris units yang sudah ada — via kd_unker, lalu via kode lama
//     (sippASNLegacyUnitCode), lalu via (kode baru, nama).
//  2. Set kd_unker + kecamatan + type; koreksi kode ke bentuk berbasis kd_unker.
//     Kecamatan unit Korwil/Dinas tidak disentuh (bukan bagian temuan ini).
//  3. Sekolah yang belum ada disisipkan (parent Korwil sekecamatan dipasang).
//  4. Baris units sekolah yang tidak dipakai unit SIPP ASN mana pun (kode lama
//     hasil bentrok nama) dilebur ke unit yang benar bila targetnya jelas.
//
// execute=false hanya menghitung tanpa menulis (dry-run).
func RepairUnitIdentity(ctx context.Context, pool *pgxpool.Pool, officers []SIPPASNOfficer, execute bool) (UnitRepairPlan, error) {
	plan := UnitRepairPlan{}
	// 1. Unit unik dari baris SIPP ASN di lingkup Dinas Pendidikan (sama seperti
	//    sinkron malam). Filter penting: tanpa ini unit non-pendidikan (RSUD,
	//    puskesmas, kecamatan) yang tipenya jatuh ke default "sd" ikut tersisip.
	cfg := SIPPASNSyncConfig{HanyaUnitPendidikan: true}
	unitByKd := map[string]ImportedTeacher{}
	for _, o := range officers {
		t, ok := MapSIPPASNOfficer(o, cfg)
		if !ok || t.KdUnker == "" || !isSchoolUnitType(t.UnitType) {
			continue
		}
		unitByKd[t.KdUnker] = t
	}
	if len(unitByKd) == 0 {
		return plan, fmt.Errorf("tidak ada unit sekolah ber-kd_unker pada data SIPP ASN")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return plan, err
	}
	defer tx.Rollback(ctx)

	// 2. Muat seluruh unit + jumlah pemakai (teachers/users/submissions).
	rows, err := tx.Query(ctx, `
		SELECT u.id, u.code, u.name, u.type, COALESCE(u.district,''), COALESCE(u.kd_unker,''),
		       (SELECT count(*) FROM teachers t WHERE t.unit_id=u.id)
		     + (SELECT count(*) FROM users us WHERE us.unit_id=u.id)
		     + (SELECT count(*) FROM submissions s WHERE s.proposed_unit_id=u.id) AS used
		FROM units u`)
	if err != nil {
		return plan, err
	}
	var all []unitRow
	byKd := map[string]*unitRow{}
	byCode := map[string]*unitRow{}
	byLegacy := map[string]*unitRow{}
	byLegacyDist := map[string]*unitRow{}
	for rows.Next() {
		var u unitRow
		if err := rows.Scan(&u.id, &u.code, &u.name, &u.typ, &u.district, &u.kd, &u.used); err != nil {
			rows.Close()
			return plan, err
		}
		all = append(all, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return plan, err
	}
	for i := range all {
		u := &all[i]
		if u.kd != "" {
			byKd[u.kd] = betterUnit(byKd[u.kd], u)
		}
		byCode[u.code] = u
		legacy := sippASNLegacyUnitCode(u.name)
		byLegacyDist[legacyKey(legacy, u.district)] = betterUnit(byLegacyDist[legacyKey(legacy, u.district)], u)
		byLegacy[legacy] = betterUnit(byLegacy[legacy], u)
	}

	claimed := map[int64]bool{}
	type pendingUpdate struct {
		id                            int64
		code, name, typ, district, kd string
	}
	var updates []pendingUpdate
	kecamatanKorwil := map[string]int64{}
	for _, u := range all {
		if u.typ == "korwil" && u.district != "" {
			kecamatanKorwil[u.district] = u.id
		}
	}

	// 3. Tentukan target untuk tiap unit SIPP ASN; urutkan agar stabil.
	kds := make([]string, 0, len(unitByKd))
	for kd := range unitByKd {
		kds = append(kds, kd)
	}
	sort.Strings(kds)

	// Pencocokan dua tahap. Tahap 1 memakai kunci paling spesifik — kd_unker,
	// kode baru, lalu "kode lama + kecamatan" — supaya baris yang sudah ada tidak
	// direbut sekolah bernama sama di kecamatan lain (temuan produksi: SDN 1
	// Karanganyar GEYER sempat berpindah ke PURWODADI). Tahap 2 baru memakai kode
	// lama saja untuk baris sisa (nama unik, mis. SDN 1-4 Tanggungharjo).
	target := map[string]*unitRow{}
	for _, kd := range kds {
		t := unitByKd[kd]
		var found *unitRow
		switch {
		case byKd[kd] != nil && !claimed[byKd[kd].id]:
			found = byKd[kd]
		case byCode[t.UnitCode] != nil && !claimed[byCode[t.UnitCode].id]:
			found = byCode[t.UnitCode]
		default:
			if u := byLegacyDist[legacyKey(sippASNLegacyUnitCode(t.UnitName), t.UnitDistrict)]; u != nil && !claimed[u.id] {
				found = u
			}
		}
		if found != nil {
			claimed[found.id] = true
			target[kd] = found
		}
	}
	for _, kd := range kds {
		if target[kd] != nil {
			continue
		}
		t := unitByKd[kd]
		if u := byLegacy[sippASNLegacyUnitCode(t.UnitName)]; u != nil && !claimed[u.id] {
			claimed[u.id] = true
			target[kd] = u
		}
	}

	for _, kd := range kds {
		t := unitByKd[kd]
		target := target[kd]
		if target == nil {
			// Sekolah belum ada: sisipkan sebagai unit baru (parent Korwil kecamatan).
			plan.Inserted++
			plan.Planned++
			if !execute {
				continue
			}
			var parentID any
			if pid, ok := kecamatanKorwil[t.UnitDistrict]; ok {
				parentID = pid
			}
			var newID int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO units (code, name, type, district, parent_id, kd_unker) VALUES ($1,$2,$3,$4,$5,$6)
				RETURNING id`, t.UnitCode, t.UnitName, t.UnitType, nullIfEmpty(t.UnitDistrict), parentID, t.KdUnker).Scan(&newID); err != nil {
				return plan, fmt.Errorf("sisip unit %s (%s): %w", t.UnitName, t.KdUnker, err)
			}
			continue
		}
		// Unit ditemukan: koreksi kode + kecamatan + type + kd_unker bila perlu.
		want := pendingUpdate{id: target.id, code: target.code, name: target.name, typ: target.typ, district: target.district, kd: target.kd}
		changed := false
		if target.kd != t.KdUnker {
			want.kd = t.KdUnker
			changed = true
		}
		if target.code != t.UnitCode {
			want.code = t.UnitCode
			plan.Reassigned++
			changed = true
		}
		if target.typ != t.UnitType {
			want.typ = t.UnitType
			changed = true
		}
		if t.UnitDistrict != "" && target.district != t.UnitDistrict {
			want.district = t.UnitDistrict
			plan.Districted++
			changed = true
		}
		if !changed {
			continue
		}
		plan.Planned++
		if execute {
			updates = append(updates, want)
		}
	}

	// 4. Unit sekolah sisa (tidak diklaim unit SIPP ASN mana pun). Bila ada unit
	//    kanonik (ber-kd_unker) dengan nama + kecamatan sama persis, ia duplikat:
	//    alihkan pemakainya lalu hapus. Bila tidak ada padanan pasti, hanya
	//    dihapus jika tanpa pemakai; selain itu dibiarkan + dicatat.
	canonical := map[string]*unitRow{}
	for _, kd := range kds {
		if u := target[kd]; u != nil {
			canonical[unitKey(unitByKd[kd].UnitName, unitByKd[kd].UnitDistrict)] = u
		}
	}
	for i := range all {
		u := &all[i]
		if !isSchoolUnitType(u.typ) || claimed[u.id] {
			continue
		}
		if dup := canonical[unitKey(u.name, u.district)]; dup != nil && dup.id != u.id {
			plan.Merged++
			plan.Planned++
			if execute {
				if err := remapUnitReferences(ctx, tx, u.id, dup.id); err != nil {
					return plan, err
				}
				if _, err := tx.Exec(ctx, `DELETE FROM units WHERE id=$1`, u.id); err != nil {
					return plan, fmt.Errorf("hapus unit duplikat id %d: %w", u.id, err)
				}
			}
			continue
		}
		if u.used > 0 {
			plan.Unmapped++
			plan.Notes = append(plan.Notes, fmt.Sprintf("unit id %d (%s) tanpa padanan SIPP ASN tetapi masih dipakai %d pengguna — dilewati", u.id, u.name, u.used))
			continue
		}
		plan.Merged++
		plan.Planned++
		if execute {
			if _, err := tx.Exec(ctx, `DELETE FROM units WHERE id=$1`, u.id); err != nil {
				return plan, fmt.Errorf("hapus unit yatim id %d: %w", u.id, err)
			}
		}
	}

	if execute {
		for _, up := range updates {
			if _, err := tx.Exec(ctx, `
				UPDATE units SET code=$2, name=$3, type=$4, district=NULLIF($5,''), kd_unker=NULLIF($6,''), updated_at=now() WHERE id=$1`,
				up.id, up.code, up.name, up.typ, up.district, up.kd); err != nil {
				return plan, fmt.Errorf("perbarui unit id %d: %w", up.id, err)
			}
		}
		// Setelah kode/kecamatan berubah, rapikan parent Korwil sekolah.
		if err := reparentSchoolsToKorwil(ctx, tx); err != nil {
			return plan, err
		}
		if err := tx.Commit(ctx); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

// reparentSchoolsToKorwil memastikan sekolah (type != korwil/dinas/skb) yang
// punya kecamatan tertaut ke Korwil kecamatan yang sama.
func reparentSchoolsToKorwil(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
		UPDATE units s SET parent_id = k.id, updated_at = now()
		FROM units k
		WHERE k.type='korwil' AND k.district IS NOT NULL
		  AND s.type IN ('sd','tk')
		  AND s.district = k.district
		  AND (s.parent_id IS DISTINCT FROM k.id)`)
	return err
}

// legacyKey menggabungkan kode lama dengan kecamatan sebagai kunci pencocokan.
func legacyKey(legacy, district string) string {
	return legacy + "\x00" + district
}

// unitKey menormalkan (nama, kecamatan) sebagai kunci deteksi duplikat: nama
// tanpa akhiran " - Dinas Pendidikan", huruf besar, hanya alfanumerik.
func unitKey(name, district string) string {
	n := strings.ToUpper(strings.TrimSpace(name))
	if i := strings.Index(n, " - DINAS PENDIDIKAN"); i >= 0 {
		n = n[:i]
	}
	n = sippASNLegacyUnitCode(n)
	return n + "\x00" + strings.ToUpper(strings.TrimSpace(district))
}

// remapUnitReferences memindahkan seluruh tautan dari unit dup ke unit keep
// (teachers, users, submissions) sebelum unit dup dihapus.
func remapUnitReferences(ctx context.Context, tx pgx.Tx, dup, keep int64) error {
	if _, err := tx.Exec(ctx, `UPDATE teachers SET unit_id=$2, updated_at=now() WHERE unit_id=$1`, dup, keep); err != nil {
		return fmt.Errorf("alihkan teachers %d -> %d: %w", dup, keep, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET unit_id=$2, updated_at=now() WHERE unit_id=$1`, dup, keep); err != nil {
		return fmt.Errorf("alihkan users %d -> %d: %w", dup, keep, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE submissions SET proposed_unit_id=$2 WHERE proposed_unit_id=$1`, dup, keep); err != nil {
		return fmt.Errorf("alihkan submissions %d -> %d: %w", dup, keep, err)
	}
	return nil
}

// betterUnit memilih baris yang lebih layak dijadikan target saat beberapa baris
// lama berbagi kunci sama (mis. pasangan kembar SMPN X vs "SMPN X - Dinas
// Pendidikan"): yang punya lebih banyak pemakai menang agar tautan tetap utuh.
func betterUnit(a, b *unitRow) *unitRow {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if b.used > a.used {
		return b
	}
	if b.used == a.used && len(b.code) > len(a.code) {
		return b
	}
	return a
}

// FormatUnitRepairPlan merangkum hasil perbaikan untuk dicetak ke log.
func FormatUnitRepairPlan(p UnitRepairPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "unit SIPP ASN: direncanakan=%d, kode dikoreksi=%d, kecamatan dikoreksi=%d, disisipkan=%d, dilebur=%d, tanpa padanan=%d",
		p.Planned, p.Reassigned, p.Districted, p.Inserted, p.Merged, p.Unmapped)
	for _, n := range p.Notes {
		fmt.Fprintf(&b, "\n  catatan: %s", n)
	}
	return b.String()
}
