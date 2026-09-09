package store

import (
	"testing"
)

// Aturan SK terbaru (keputusan owner 2026-09-09): pemenang ditentukan dari
// TANGGAL SK, bukan TMT berlaku. Pemenang mengisi seluruh kolom identitas
// SK terakhir naskah; MKG lama tetap dari SK KGB.
func TestSKNewer(t *testing.T) {
	a := date("2026-03-06")
	b := date("2024-06-09")
	if !SKNewer(a, b) {
		t.Error("2026-03-06 harus lebih baru dari 2024-06-09")
	}
	if SKNewer(b, a) {
		t.Error("2024-06-09 tidak boleh lebih baru dari 2026-03-06")
	}
	if !SKNewer(a, a) {
		t.Error("tanggal sama harus menang (tidak sebelum)")
	}
	if SKNewer(nil, b) {
		t.Error("tanpa tanggal KP tidak boleh menang")
	}
	if !SKNewer(a, nil) {
		t.Error("tanpa tanggal KGB, KP harus menang")
	}
	if SKNewer(nil, nil) {
		t.Error("keduanya tanpa tanggal tidak boleh menang")
	}
}

func TestEffectiveGolonganTanggalSK(t *testing.T) {
	kgbTanggal := date("2024-06-09")
	// Kasus PDF uji coba: SK KP 06 Mar 2026 > SK KGB 09 Jun 2024 → KP menang.
	kp := KPLast{Golongan: "III/b", TMT: date("2026-01-07"), Tanggal: date("2026-03-06")}
	if got := EffectiveGolongan(kp, kgbTanggal, "III/a"); got != "III/b" {
		t.Errorf("KP menang = %q, ingin III/b", got)
	}
	// SK KP lebih lama dari SK KGB → KGB menang meski TMT KP lebih baru.
	kpLama := KPLast{Golongan: "III/b", TMT: date("2026-01-07"), Tanggal: date("2023-01-02")}
	if got := EffectiveGolongan(kpLama, kgbTanggal, "III/a"); got != "III/a" {
		t.Errorf("KGB menang = %q, ingin III/a", got)
	}
	// Tanpa tanggal SK KP → KGB menang.
	kpTanpaTgl := KPLast{Golongan: "III/b", TMT: date("2026-01-07")}
	if got := EffectiveGolongan(kpTanpaTgl, kgbTanggal, "III/a"); got != "III/a" {
		t.Errorf("tanpa tanggal SK KP = %q, ingin III/a", got)
	}
	// Tanpa KP → KGB.
	if got := EffectiveGolongan(KPLast{}, kgbTanggal, "III/a"); got != "III/a" {
		t.Errorf("tanpa KP = %q, ingin III/a", got)
	}
}
