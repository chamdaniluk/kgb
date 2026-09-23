package httpapi

import (
	"regexp"
	"strings"
	"testing"
)

// Regresi untuk bug 2026-09-23: template openKoreksi memuat satu </div>
// berlebih tepat setelah input hidden, sehingga parser HTML5 menutup
// <form id="koreksi-form"> lebih awal. Tombol "Simpan Koreksi" pun
// berada di luar form: klik tidak mengirim apa pun tanpa peringatan.
//
// Uji ini mengekstrak template openKoreksi dari appJS lalu memeriksa
// keseimbangan div/form dengan penelusuran tumpukan sederhana, dan
// memastikan tombol submit serta semua input bernama berada di dalam form.
func extractOpenKoreksiTemplate(t *testing.T) string {
	t.Helper()
	start := strings.Index(appJS, "async function openKoreksi")
	if start < 0 {
		t.Fatal("fungsi openKoreksi tidak ditemukan di appJS")
	}
	end := strings.Index(appJS[start:], "async function review(")
	if end < 0 {
		t.Fatal("batas akhir openKoreksi tidak ditemukan")
	}
	body := appJS[start : start+end]
	m := regexp.MustCompile(`setView\('`).
		FindStringIndex(body)
	if m == nil {
		t.Fatal("pemanggilan setView tidak ditemukan di openKoreksi")
	}
	// Template berakhir pada "');" terakhir sebelum baris wiring onsubmit.
	cut := strings.Index(body, "document.querySelector('#koreksi-form')")
	if cut < 0 {
		t.Fatal("wiring onsubmit tidak ditemukan")
	}
	tpl := body[m[1]:cut]
	tpl = strings.TrimSuffix(tpl, ");\n")
	return tpl
}

// stripTemplateExpr menghapus ekspresi konkatenasi JS ('+...+') sehingga
// tersisa markup statis untuk dianalisis. Ekspresi dinamis openKoreksi
// hanya menghasilkan teks/atribut, bukan tag, jadi aman diganti placeholder.
func stripTemplateExpr(t *testing.T, tpl string) string {
	t.Helper()
	// ganti '+...+' (konkatenasi) dengan atribut placeholder
	re := regexp.MustCompile(`'\s*\+[^+]*\+\s*'`)
	out := re.ReplaceAllString(tpl, "X")
	return out
}

func TestOpenKoreksiFormTidakDitutupPrematur(t *testing.T) {
	tpl := extractOpenKoreksiTemplate(t)
	staticHTML := stripTemplateExpr(t, tpl)

	// Ekspresi dinamis bisa memuat tag (blok isPPPK), kembalikan blok itu
	// dengan markup literal-nya agar analisis realistis.
	staticHTML = strings.ReplaceAll(staticHTML,
		"'+(isPPPK?'<div class=\"two-col\"><label>Masa perjanjian kerja<input required name=\"masa_perjanjian_kerja\" value=\"X\"></label><label>Perpanjangan (tanggal / -)<input required type=\"date\" name=\"perpanjangan_perjanjian_kerja\" value=\"X\"></label></div>':'')+'",
		"<div class=\"two-col\"><label>Masa perjanjian kerja<input required name=\"masa_perjanjian_kerja\" value=\"X\"></label><label>Perpanjangan (tanggal / -)<input required type=\"date\" name=\"perpanjangan_perjanjian_kerja\" value=\"X\"></label></div>")

	openDiv := strings.Count(staticHTML, "<div")
	closeDiv := strings.Count(staticHTML, "</div>")
	if openDiv != closeDiv {
		t.Fatalf("div tidak seimbang di template koreksi: buka=%d tutup=%d", openDiv, closeDiv)
	}

	iForm := strings.Index(staticHTML, "<form id=\"koreksi-form\"")
	iFormEnd := strings.Index(staticHTML, "</form>")
	if iForm < 0 || iFormEnd < 0 {
		t.Fatal("form koreksi tidak lengkap")
	}
	inside := staticHTML[iForm:iFormEnd]
	if !strings.Contains(inside, "type=\"submit\"") {
		t.Fatal("tombol Simpan Koreksi (type=submit) berada di luar <form> — klik tidak akan mengirim apa pun")
	}
	for _, name := range []string{
		"tmt_awal", "last_kp_tmt", "last_kp_tanggal", "last_kp_golongan",
		"last_kp_masa_tahun", "last_kp_masa_bulan", "last_kp_nomor", "last_kp_pejabat",
		"last_sk_tmt", "last_sk_tanggal", "last_kgb_golongan", "last_kgb_masa_tahun",
		"last_kgb_masa_bulan", "last_sk_nomor", "last_sk_pejabat",
		"birth_place", "birth_date", "karpeg", "jabatan",
		"unit_id", "pangkat_gol", "pangkat", "koreksi_note",
	} {
		if !strings.Contains(inside, "name=\""+name+"\"") {
			t.Errorf("input %s berada di luar <form> koreksi", name)
		}
	}
}
