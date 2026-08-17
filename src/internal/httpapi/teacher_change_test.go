package httpapi

import (
	"testing"
	"time"

	"sicendikia/internal/store"
)

func TestResolveTeacherChangeCapturesApprovedFieldsAndAudit(t *testing.T) {
	teacher := store.Teacher{
		PangkatGol: "III/b",
		Pangkat:    "Penata Muda Tingkat I",
		Jabatan:    "Guru Ahli Pertama",
		UnitID:     10,
		UnitName:   "SDN LAMA",
	}
	effective := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	change, err := resolveTeacherChange(teacher, ProposedTeacherChangeInput{
		PangkatGol:    "III/c",
		Pangkat:       "Penata",
		Jabatan:       "Kepala Sekolah",
		UnitID:        int64Ptr(20),
		EffectiveDate: &effective,
		Note:          "SK kenaikan pangkat dan mutasi",
	}, &store.Unit{ID: 20, Code: "SMP-BARU", Name: "SMP BARU", Type: "smp"}, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if change.PangkatGol == nil || *change.PangkatGol != "III/c" {
		t.Fatalf("pangkat gol = %#v", change.PangkatGol)
	}
	if change.Pangkat == nil || *change.Pangkat != "Penata" {
		t.Fatalf("pangkat = %#v", change.Pangkat)
	}
	if change.Jabatan == nil || *change.Jabatan != "Kepala Sekolah" {
		t.Fatalf("jabatan = %#v", change.Jabatan)
	}
	if change.UnitID == nil || *change.UnitID != 20 {
		t.Fatalf("unit = %#v", change.UnitID)
	}
	if change.AuditDetails["pangkat_gol_baru"] != "III/c" || change.AuditDetails["unit_baru"] != "SMP BARU" {
		t.Fatalf("audit perubahan = %#v", change.AuditDetails)
	}
}

func TestResolveTeacherChangeRequiresSKEffectiveDate(t *testing.T) {
	_, err := resolveTeacherChange(store.Teacher{PangkatGol: "III/b"}, ProposedTeacherChangeInput{
		PangkatGol: "III/c",
	}, &store.Unit{ID: 20, Name: "SMP BARU", Type: "smp"}, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("resolveTeacherChange() error = nil, want effective date error")
	}
}

func TestResolveTeacherChangeRejectsNonSchoolTargetUnit(t *testing.T) {
	effective := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	_, err := resolveTeacherChange(store.Teacher{UnitID: 10}, ProposedTeacherChangeInput{
		UnitID:        int64Ptr(30),
		EffectiveDate: &effective,
	}, &store.Unit{ID: 30, Name: "KORWILCAM BARU", Type: "korwil"}, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("resolveTeacherChange() error = nil, want invalid target unit error")
	}
}

func int64Ptr(v int64) *int64 { return &v }
