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
	return xml
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
