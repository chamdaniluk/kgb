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
	kgb2024 := date("2024-12-01")
	cases := []struct {
		name string
		kp   KPLast
		kgb  *time.Time
		gol  string
		want string
	}{
		// Contoh owner: CPNS Des 2020, KP IIIa->IIIb, usul KGB Des 2026.
		{"KP lebih baru menang", KPLast{Golongan: "III/b", TMT: date("2025-07-01")}, kgb2024, "III/a", "III/b"},
		// Tanpa KP: golongan sama.
		{"tanpa KP pakai KGB", KPLast{}, kgb2024, "III/a", "III/a"},
		// KP lebih lama dari KGB: KGB menang.
		{"KP lama diabaikan", KPLast{Golongan: "III/b", TMT: date("2022-04-01")}, kgb2024, "III/a", "III/a"},
		// KP sama tanggal dengan KGB: KP menang (tidak sebelum KGB).
		{"KP sama tanggal menang", KPLast{Golongan: "III/b", TMT: date("2024-12-01")}, kgb2024, "III/a", "III/b"},
		// Tanpa KGB terakhir: KP menang bila ada.
		{"tanpa KGB, KP menang", KPLast{Golongan: "III/b", TMT: date("2025-07-01")}, nil, "III/a", "III/b"},
	}
	for _, c := range cases {
		if got := EffectiveGolongan(c.kp, c.kgb, c.gol); got != c.want {
			t.Errorf("%s = %q, want %q", c.name, got, c.want)
		}
	}
	if (KPLast{}).HasData() {
		t.Error("KP kosong harus HasData()=false")
	}
	if !(KPLast{Golongan: "III/b"}).HasData() && true {
		// golongan tanpa TMT bukan data lengkap
	} else {
		t.Error("KP tanpa TMT harus HasData()=false")
	}
}
