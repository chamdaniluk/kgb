package httpapi

import (
	"os"
	"testing"

	"sicendikia/internal/store"

	"github.com/xuri/excelize/v2"
)

func TestActualBKNWorkbookDryRun(t *testing.T) {
	path := "/home/ubuntu/.hermes/cache/documents/doc_e9889a9f4c11_dataguru.xlsx"
	file, err := os.Open(path)
	if err != nil {
		t.Skipf("workbook BKN tidak tersedia: %v", err)
	}
	defer file.Close()
	book, err := excelize.OpenReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	sheets := book.GetSheetList()
	if len(sheets) != 1 {
		t.Fatalf("sheet = %v, want one sheet", sheets)
	}
	rows, err := book.GetRows(sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	teachers, err := parseImportTeachers(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(teachers) != 7301 {
		t.Fatalf("parsed rows = %d, want 7301", len(teachers))
	}
	seen := make(map[string]bool, len(teachers))
	counts := map[string]int{}
	sources := map[string]int{}
	units := map[string]bool{}
	for _, teacher := range teachers {
		if seen[teacher.NIP] {
			t.Fatalf("duplicate NIP detected in dry-run")
		}
		seen[teacher.NIP] = true
		counts[teacher.ASNType]++
		sources[teacher.MasaKerjaSource]++
		units[teacher.UnitCode] = true
		if err := validateDryRunTeacher(teacher); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("dry-run rows=%d pns=%d pppk=%d units=%d sources=%v", len(teachers), counts["pns"], counts["pppk"], len(units), sources)
}

func validateDryRunTeacher(teacher store.ImportedTeacher) error {
	return store.ValidateImportedTeacher(teacher)
}
