package pdf

import (
	"archive/zip"
	"os"
	"strings"
	"testing"
)

// bukaBagianDOCX membaca satu bagian (mis. word/document.xml) dari DOCX.
func bukaBagianDOCX(t *testing.T, path, name string) string {
	t.Helper()
	content, err := cobaBukaBagianDOCX(path, name)
	if err != nil {
		t.Fatalf("bagian %s tidak ditemukan di %s", name, path)
	}
	return content
}

// cobaBukaBagianDOCX seperti bukaBagianDOCX tetapi melaporkan error alih-alih
// menggagalkan test — untuk bagian yang tidak selalu ada di semua template.
func cobaBukaBagianDOCX(path, name string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		buf := new(strings.Builder)
		chunk := make([]byte, 32*1024)
		for {
			n, err := rc.Read(chunk)
			if n > 0 {
				buf.Write(chunk[:n])
			}
			if err != nil {
				break
			}
		}
		return buf.String(), nil
	}
	return "", os.ErrNotExist
}

func TestDraftManualTTEPlaceholderUtuhDanMailMergeBersih(t *testing.T) {
	for _, tpl := range []string{"templates/pns.docx", "templates/pppk.docx"} {
		if _, err := os.Stat(tpl); err != nil {
			t.Skip(err)
		}
		out := t.TempDir() + "/manual.docx"
		values := map[string]string{
			"nama":           "GURU UJI",
			"nomor_naskah":   "800/099/4.2/2026",
			"tanggal_naskah": "23 September 2026",
			"ttd_pengirim":   "Kepala Dinas\nNIP. 196711271995121002",
		}
		// Simulasi render TTE manual: tiga placeholder tidak boleh terisi.
		delete(values, "nomor_naskah")
		delete(values, "tanggal_naskah")
		delete(values, "ttd_pengirim")
		if err := fillDOCX(tpl, out, values); err != nil {
			t.Fatal(err)
		}
		doc := bukaBagianDOCX(t, out, "word/document.xml")
		for _, ph := range []string{"${nomor_naskah}", "${tanggal_naskah}", "${ttd_pengirim}"} {
			if !strings.Contains(doc, ph) {
				t.Errorf("%s: placeholder %s hilang — harus utuh untuk TTE manual", tpl, ph)
			}
		}
		if !strings.Contains(doc, "GURU UJI") {
			t.Errorf("%s: data naskah tidak terisi", tpl)
		}
		if strings.Contains(doc, "MERGEFIELD") || strings.Contains(doc, "instrText") {
			t.Errorf("%s: sisa field mail merge di document.xml", tpl)
		}
		if strings.Contains(doc, "fldChar") {
			t.Errorf("%s: masih ada fldChar di document.xml", tpl)
		}
		settings := bukaBagianDOCX(t, out, "word/settings.xml")
		if strings.Contains(settings, "mailMerge") || strings.Contains(settings, "OLEDB") {
			t.Errorf("%s: konfigurasi mailMerge masih ada di settings.xml", tpl)
		}
		if strings.Contains(settings, "62877") {
			t.Errorf("%s: jejak koneksi sumber data lama masih ada", tpl)
		}
		if rels, err := cobaBukaBagianDOCX(out, "word/_rels/settings.xml.rels"); err == nil {
			if strings.Contains(rels, "mailMergeSource") {
				t.Errorf("%s: relasi mailMergeSource masih ada", tpl)
			}
		}
	}
}

func TestRenderLetterDOCXManualTTE(t *testing.T) {
	if _, err := os.Stat("templates/pns.docx"); err != nil {
		t.Skip(err)
	}
	r := &Renderer{TempDir: t.TempDir(), TemplateDir: "templates"}
	body, err := RenderLetterDOCXManualTTE(r, LetterData{
		ASNType:     "pns",
		TeacherName: "GURU UJI",
		Number:      "800/123/4.2/2026",
	})
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir() + "/out.docx"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		t.Fatal(err)
	}
	doc := bukaBagianDOCX(t, tmp, "word/document.xml")
	if !strings.Contains(doc, "${nomor_naskah}") {
		t.Fatal("${nomor_naskah} harus tetap utuh pada draft TTE manual")
	}
	if !strings.Contains(doc, "GURU UJI") {
		t.Fatal("nama guru tidak terisi")
	}

	// Mode reguler tetap mengisi semua placeholder.
	body2, err := RenderLetterDOCX(r, LetterData{ASNType: "pns", TeacherName: "GURU UJI", Number: "800/123/4.2/2026"})
	if err != nil {
		t.Fatal(err)
	}
	tmp2 := t.TempDir() + "/out2.docx"
	if err := os.WriteFile(tmp2, body2, 0o600); err != nil {
		t.Fatal(err)
	}
	doc2 := bukaBagianDOCX(t, tmp2, "word/document.xml")
	if strings.Contains(doc2, "${nomor_naskah}") {
		t.Fatal("mode reguler harus mengisi ${nomor_naskah}")
	}
	if !strings.Contains(doc2, "800/123/4.2/2026") {
		t.Fatal("nomor tidak terisi pada mode reguler")
	}
}
