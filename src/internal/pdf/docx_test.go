package pdf

import (
	"archive/zip"
	"os"
	"strings"
	"testing"
)

func TestFillDOCXMenggantiPlaceholderPNS(t *testing.T) {
	src := "templates/pns.docx"
	if _, err := os.Stat(src); err != nil {
		t.Skip(err)
	}
	out := t.TempDir() + "/out.docx"
	if err := fillDOCX(src, out, map[string]string{
		"nama":         "GURU UJI",
		"nomor_naskah": "800/099/4.2/2026",
	}); err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var xml string
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			buf := make([]byte, f.UncompressedSize64+1)
			n, _ := rc.Read(buf)
			_ = rc.Close()
			xml = string(buf[:n])
		}
	}
	if strings.Contains(xml, "${nama}") {
		t.Fatal("placeholder ${nama} masih ada")
	}
	if !strings.Contains(xml, "GURU UJI") {
		t.Fatal("nama uji tidak masuk DOCX")
	}
	if strings.Contains(xml, "MUHTAR ARIFIN") {
		t.Fatal("contoh lama masih tertinggal")
	}
}
