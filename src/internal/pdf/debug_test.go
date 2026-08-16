package pdf

import (
	"context"
	"testing"
)

func TestRenderLetter(t *testing.T) {
	renderer := NewRenderer()
	if renderer.Binary == "" {
		t.Skip("binary PDF renderer tidak tersedia")
	}
	data, err := renderer.RenderLetter(context.Background(), LetterData{Number: "800/001/4.2/2026", TeacherName: "Guru", NIP: "198001012005011001", UnitName: "Unit", ProposedTMT: "01-09-2026", CurrentSalary: "3497300", NextSalary: "3607500", IssuedAt: "16 August 2026", SignerName: "Kepala Dinas"})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		t.Fatalf("hasil bukan PDF valid: %d bytes", len(data))
	}
}
