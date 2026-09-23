package pdf

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func fillDOCX(templatePath, outputPath string, values map[string]string) error {
	src, err := zip.OpenReader(templatePath)
	if err != nil {
		return fmt.Errorf("buka template DOCX: %w", err)
	}
	defer src.Close()
	var buf bytes.Buffer
	dst := zip.NewWriter(&buf)
	for _, file := range src.File {
		rc, err := file.Open()
		if err != nil {
			return err
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return err
		}
		if file.Name == "word/document.xml" {
			body = []byte(replacePlaceholders(string(body), values))
		}
		if file.Name == "word/settings.xml" {
			body = []byte(stripMailMergeSettings(string(body)))
		}
		if strings.HasSuffix(file.Name, ".xml.rels") {
			body = []byte(stripMailMergeRelationships(string(body)))
		}
		w, err := dst.Create(file.Name)
		if err != nil {
			return err
		}
		if _, err := w.Write(body); err != nil {
			return err
		}
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return err
	}
	return os.WriteFile(outputPath, buf.Bytes(), 0o640)
}

func replacePlaceholders(xml string, values map[string]string) string {
	xml = collapseSplitPlaceholders(xml)
	for key, value := range values {
		xml = strings.ReplaceAll(xml, "${"+key+"}", escapeXML(value))
	}
	return removeMergeFields(xml)
}

// removeMergeFields menghapus field mail merge (MERGEFIELD) yang tersisa di
// document.xml — biasanya sisa template lama berdampingan dengan placeholder
// ${...} untuk data yang sama. Field dibuang utuh mulai w:fldChar begin
// hingga w:fldChar end sehingga Word tidak menampilkan sumber data lagi;
// nilai sebenarnya sudah hadir lewat placeholder ${...} di teks sekitarnya.
func removeMergeFields(xml string) string {
	var out strings.Builder
	for {
		b := strings.Index(xml, `<w:fldChar w:fldCharType="begin"`)
		if b < 0 {
			out.WriteString(xml)
			return out.String()
		}
		// Akhir tag pembuka field (bisa "/>" atau ">").
		bEnd := strings.Index(xml[b:], ">")
		if bEnd < 0 {
			out.WriteString(xml)
			return out.String()
		}
		stop := b + bEnd + 1
		e := strings.Index(xml[stop:], `w:fldCharType="end"`)
		if e < 0 {
			out.WriteString(xml)
			return out.String()
		}
		eEnd := strings.Index(xml[stop+e:], ">")
		if eEnd < 0 {
			out.WriteString(xml)
			return out.String()
		}
		fieldEnd := stop + e + eEnd + 1
		if !strings.Contains(xml[stop:fieldEnd], "MERGEFIELD") {
			out.WriteString(xml[:fieldEnd])
			xml = xml[fieldEnd:]
			continue
		}
		out.WriteString(xml[:b])
		xml = xml[fieldEnd:]
	}
}

func collapseSplitPlaceholders(xml string) string {
	const maxGap = 400
	var b strings.Builder
	b.Grow(len(xml))
	i := 0
	for i < len(xml) {
		start := strings.Index(xml[i:], "${")
		if start < 0 {
			b.WriteString(xml[i:])
			break
		}
		start += i
		b.WriteString(xml[i:start])
		endRel := strings.Index(xml[start:], "}")
		if endRel < 0 || endRel > maxGap {
			b.WriteString("${")
			i = start + 2
			continue
		}
		chunk := xml[start : start+endRel+1]
		plain := stripXMLTags(chunk)
		if strings.HasPrefix(plain, "${") && strings.HasSuffix(plain, "}") && !strings.Contains(plain[2:len(plain)-1], "${") {
			b.WriteString(plain)
			i = start + endRel + 1
			continue
		}
		b.WriteString("${")
		i = start + 2
	}
	return b.String()
}

func stripXMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// stripMailMergeSettings menghapus elemen mailMerge pada settings.xml beserta
// seluruh isinya (tipe dokumen utama dan koneksi sumber data lama, mis. ACE
// OLEDB ke berkas xlsx di komputer penyusun template). Sisa koneksi ini
// membuat Word menampilkan peringatan sumber data ketika hasil generate
// dibuka, padahal dokumen sudah tidak memakai mail merge.
func stripMailMergeSettings(xml string) string {
	const openTag = "<w:mailMerge>"
	for {
		start := strings.Index(xml, openTag)
		if start < 0 {
			return xml
		}
		end := strings.Index(xml[start:], "</w:mailMerge>")
		if end < 0 {
			return xml[:start]
		}
		xml = xml[:start] + xml[start+end+len("</w:mailMerge>"):]
	}
}

// stripMailMergeRelationships membuang relasi mailMergeSource pada berkas
// .rels (mis. tautan eksternal ke mm_suratkuasa.xlsx) supaya paket hasil
// generate bersih dari jejak sumber data mail merge.
func stripMailMergeRelationships(xml string) string {
	const marker = "officeDocument/2006/relationships/mailMergeSource"
	var out strings.Builder
	for {
		start := strings.Index(xml, "<Relationship ")
		if start < 0 {
			out.WriteString(xml)
			return out.String()
		}
		endSelf := strings.Index(xml[start:], "/>")
		endFull := strings.Index(xml[start:], "</Relationship>")
		var cutEnd int
		if endFull >= 0 && (endSelf < 0 || endFull < endSelf) {
			cutEnd = start + endFull + len("</Relationship>")
		} else if endSelf >= 0 {
			cutEnd = start + endSelf + len("/>")
		} else {
			out.WriteString(xml)
			return out.String()
		}
		seg := xml[start:cutEnd]
		if !strings.Contains(seg, marker) {
			out.WriteString(xml[:cutEnd])
			xml = xml[cutEnd:]
			continue
		}
		xml = xml[:start] + xml[cutEnd:]
	}
}
