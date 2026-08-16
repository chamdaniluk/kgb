package httpapi

import (
	"testing"
	"time"

	"sicendikia/internal/store"
)

func TestResolveKGBInputsUsesTeacherDataWhenComplete(t *testing.T) {
	teacher := store.Teacher{MasaKerjaTahun: 15, MasaKerjaSource: "tmt_cpns"}
	proposed := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	mass, last, err := resolveKGBInputs(teacher, nil, nil, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if mass != 15 || last != nil {
		t.Fatalf("resolved = %d/%v, want 15/nil", mass, last)
	}
}

func TestResolveKGBInputsRequiresMissingMasaKerja(t *testing.T) {
	teacher := store.Teacher{MasaKerjaTahun: 0, MasaKerjaSource: "belum_tersedia"}
	proposed := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, _, err := resolveKGBInputs(teacher, nil, nil, proposed); err == nil {
		t.Fatal("resolveKGBInputs() error = nil, want missing masa kerja error")
	}
}

func TestResolveKGBInputsAcceptsUpdateAtSubmission(t *testing.T) {
	teacher := store.Teacher{MasaKerjaTahun: 0, MasaKerjaSource: "belum_tersedia"}
	masa := 4
	last := time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC)
	proposed := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	mass, gotLast, err := resolveKGBInputs(teacher, &masa, &last, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if mass != 4 || gotLast == nil || !gotLast.Equal(last) {
		t.Fatalf("resolved = %d/%v, want 4/%v", mass, gotLast, last)
	}
}

func TestResolveKGBInputsRejectsLastKGBAfterProposedTMT(t *testing.T) {
	teacher := store.Teacher{MasaKerjaTahun: 4, MasaKerjaSource: "tmt_gol"}
	masa := 4
	last := time.Date(2027, 8, 1, 0, 0, 0, 0, time.UTC)
	if _, _, err := resolveKGBInputs(teacher, &masa, &last, time.Date(2027, 7, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("resolveKGBInputs() error = nil, want invalid last KGB date")
	}
}

func TestResolveKGBInputsRejectsNegativeMasaKerja(t *testing.T) {
	teacher := store.Teacher{MasaKerjaTahun: 4, MasaKerjaSource: "tmt_gol"}
	masa := -1
	proposed := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, _, err := resolveKGBInputs(teacher, &masa, nil, proposed); err == nil {
		t.Fatal("resolveKGBInputs() error = nil, want negative masa kerja error")
	}
}
