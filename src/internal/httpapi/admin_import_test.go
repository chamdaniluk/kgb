package httpapi

import (
	"testing"

	"sicendikia/internal/store"
)

func TestParseImportStaffMapsExcelRolesAndScopes(t *testing.T) {
	rows := [][]string{
		{"Nama Institusi", "Role", "Username", "Password"},
		{"Dinas Pendidikan", "Admin TTE", "admin.tte", "secret"},
		{"Dinas Pendidikan", "Admin Dinas", "admin.disdik1", "secret"},
		{"KORWILCAM BRATI", "Admin Korwil", "kwc_bra", "secret"},
		{"SMP NEGERI 1 BRATI", "Admin SMP", "smpn1_bra", "secret"},
		{"SPNF SKB GROBOGAN", "Admin SMP", "skb_gbg", "secret"},
	}

	got, err := parseImportStaff(rows)
	if err != nil {
		t.Fatalf("parseImportStaff() error = %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("len(got) = %d, want 5", len(got))
	}

	if got[0].Role != "pimpinan" || !got[0].NeedsSignerProfile {
		t.Fatalf("Admin TTE mapping = role %q needs_signer=%v, want pimpinan/true", got[0].Role, got[0].NeedsSignerProfile)
	}
	if got[1].Role != "admin_dinas" || got[1].UnitName != "" {
		t.Fatalf("Admin Dinas mapping = role %q unit=%q, want admin_dinas/no unit", got[1].Role, got[1].UnitName)
	}
	if got[2].Role != "verifikator_unit" || got[2].UnitType != "korwil" || got[2].UnitCode != "KORWILCAM-BRATI" {
		t.Fatalf("Admin Korwil mapping = %#v", got[2])
	}
	if got[3].Role != "verifikator_unit" || got[3].UnitType != "smp" || got[3].ParentUnitName != "KORWILCAM BRATI" {
		t.Fatalf("Admin SMP mapping = %#v", got[3])
	}
	if got[4].UnitType != "skb" || got[4].ParentUnitName != "KORWILCAM GROBOGAN" {
		t.Fatalf("Admin SKB mapping = %#v", got[4])
	}
	for _, item := range got {
		if store.IsNIPUsername(item.Username) {
			t.Errorf("username %q unexpectedly treated as NIP", item.Username)
		}
	}
}

func TestParseImportStaffMapsSignerProfile(t *testing.T) {
	rows := [][]string{
		{"Nama Institusi", "Role", "Username", "Password", "NIP", "Jabatan"},
		{"Dinas Pendidikan", "Admin TTE", "admin.tte", "secret", "198001012005011002", "Kepala Dinas Pendidikan"},
	}

	got, err := parseImportStaff(rows)
	if err != nil {
		t.Fatalf("parseImportStaff() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].EmployeeNumber != "198001012005011002" {
		t.Errorf("EmployeeNumber = %q, want 198001012005011002", got[0].EmployeeNumber)
	}
	if got[0].JobTitle != "Kepala Dinas Pendidikan" {
		t.Errorf("JobTitle = %q, want Kepala Dinas Pendidikan", got[0].JobTitle)
	}
}

func TestParseImportStaffRejectsUnknownRole(t *testing.T) {
	rows := [][]string{
		{"Nama Institusi", "Role", "Username", "Password"},
		{"Dinas Pendidikan", "Role Tidak Dikenal", "petugas-x", "secret"},
	}
	if _, err := parseImportStaff(rows); err == nil {
		t.Fatal("parseImportStaff() error = nil, want unknown role error")
	}
}
