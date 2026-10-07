package httpapi

import (
	"strings"
	"testing"
)

// Regresi 2026-10-07: pada openResubmit, input unggah "SKP 2 tahun" hanya
// dirender bila pengajuan belum menyimpan berkas SKP (needSKP&&!hasSKP).
// SKP memang wajib sejak pengajuan awal PPPK, sehingga form "Perbaiki &
// Kirim Ulang" PPPK selalu tampil tanpa slot SKP — berkas lama tak pernah
// bisa diganti. Pola yang benar mengikuti slot lain (SK Pertama, Perjanjian
// Kerja, KGB): input selalu dirender, atribut required hanya bila berkas
// lama belum ada.
func TestOpenResubmitSlotSKPSelaluTampil(t *testing.T) {
	start := strings.Index(appJS, "async function openResubmit")
	if start < 0 {
		t.Fatal("fungsi openResubmit tidak ditemukan di appJS")
	}
	end := strings.Index(appJS[start:], "function salaryFlow")
	if end < 0 {
		t.Fatal("batas akhir openResubmit tidak ditemukan")
	}
	body := appJS[start : start+end]

	if strings.Contains(body, "needSKP&&!hasSKP") {
		t.Fatal("slot SKP masih digate keberadaan berkas lama (needSKP&&!hasSKP) — input hilang saat SKP sudah pernah diunggah")
	}
	if !strings.Contains(body, `name="file_skp"`) {
		t.Fatal("input unggah SKP 2 tahun tidak ditemukan di openResubmit")
	}
	// Wajib hanya bila berkas lama belum ada, sama seperti slot KP/PK.
	if !strings.Contains(body, "(hasSKP?'':' required')") {
		t.Fatal("input SKP tidak memakai pola required kondisional (hasSKP?'':' required')")
	}
}
