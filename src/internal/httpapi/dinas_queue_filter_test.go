package httpapi

import "testing"

// Filter tab antrean Dinas harus dipetakan ke whitelist status; nilai tak
// dikenal jatuh ke antrean aktif (menunggu Dinas + menunggu TTE).
func TestDinasQueueStatuses(t *testing.T) {
	cases := []struct {
		filter string
		want   []string
	}{
		{"", []string{"menunggu_dinas", "menunggu_tte"}},
		{"menunggu", []string{"menunggu_dinas", "menunggu_tte"}},
		{"  MENUNGGU ", []string{"menunggu_dinas", "menunggu_tte"}},
		{"dikembalikan", []string{"dikembalikan_unit", "dikembalikan_dinas"}},
		{"terbit", []string{"terbit"}},
		{"semua", []string{"menunggu_dinas", "menunggu_tte", "dikembalikan_unit", "dikembalikan_dinas", "terbit"}},
		{"ngawur", []string{"menunggu_dinas", "menunggu_tte"}},
	}
	for _, c := range cases {
		got := dinasQueueStatuses(c.filter)
		if len(got) != len(c.want) {
			t.Fatalf("filter %q: dapat %v, ingin %v", c.filter, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("filter %q: dapat %v, ingin %v", c.filter, got, c.want)
			}
		}
	}
}
