package store

import (
	"testing"
	"time"
)

func date(s string) *time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return &t
}

func TestEffectiveGolongan(t *testing.T) {
	kgbTanggal := date("2024-06-09")
	cases := []struct {
		name string
		kp   KPLast
		kgb  *time.Time
		gol  string
		want string
	}{
		// Acuan SK terbaru = TANGGAL SK (keputusan owner 2026-09-09).
		// Contoh owner: CPNS Des 2020, KP IIIa->IIIb, usul KGB Des 2026.
		{"SK KP lebih baru menang", KPLast{Golongan: "III/b", TMT: date("2025-07-01"), Tanggal: date("2025-08-04")}, kgbTanggal, "III/a", "III/b"},
		// Tanpa KP: golongan sama.
		{"tanpa KP pakai KGB", KPLast{}, kgbTanggal, "III/a", "III/a"},
		// SK KP lebih lama dari SK KGB: KGB menang meski TMT KP lebih baru.
		{"SK KP lama diabaikan", KPLast{Golongan: "III/b", TMT: date("2025-07-01"), Tanggal: date("2023-01-02")}, kgbTanggal, "III/a", "III/a"},
		// SK KP sama tanggal dengan SK KGB: KP menang (tidak sebelum KGB).
		{"SK sama tanggal menang", KPLast{Golongan: "III/b", TMT: date("2025-07-01"), Tanggal: date("2024-06-09")}, kgbTanggal, "III/a", "III/b"},
		// Tanpa tanggal SK KGB: KP menang bila ada tanggal SK.
		{"tanpa tanggal KGB, KP menang", KPLast{Golongan: "III/b", TMT: date("2025-07-01"), Tanggal: date("2025-08-04")}, nil, "III/a", "III/b"},
		// Tanpa tanggal SK KP: KGB menang.
		{"tanpa tanggal SK KP, KGB menang", KPLast{Golongan: "III/b", TMT: date("2025-07-01")}, kgbTanggal, "III/a", "III/a"},
	}
	for _, c := range cases {
		if got := EffectiveGolongan(c.kp, c.kgb, c.gol); got != c.want {
			t.Errorf("%s = %q, want %q", c.name, got, c.want)
		}
	}
	if (KPLast{}).HasData() {
		t.Error("KP kosong harus HasData()=false")
	}
	if (KPLast{Golongan: "III/b"}).HasData() {
		t.Error("KP tanpa TMT harus HasData()=false")
	}
}
