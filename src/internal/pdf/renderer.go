// Package pdf merender konsep surat KGB ke PDF melalui renderer sistem.
package pdf

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LetterData adalah data snapshot yang masuk ke template surat.
type LetterData struct {
	Number        string
	TeacherName   string
	NIP           string
	UnitName      string
	ProposedTMT   string
	CurrentSalary string
	NextSalary    string
	IssuedAt      string
	SignerName    string
}

// Renderer menjalankan wkhtmltopdf atau Chromium headless.
type Renderer struct {
	Binary  string
	TempDir string
}

// NewRenderer memilih binary wkhtmltopdf jika tersedia, kemudian Chromium.
func NewRenderer() *Renderer {
	tempDir := os.Getenv("PDF_TMP_DIR")
	if tempDir == "" {
		if homeDir, err := os.UserHomeDir(); err == nil {
			tempDir = filepath.Join(homeDir, "si-cendikia-pdf")
		}
	}
	if tempDir != "" {
		_ = os.MkdirAll(tempDir, 0o750)
	}
	if v := os.Getenv("PDF_BIN"); v != "" {
		return &Renderer{Binary: v, TempDir: tempDir}
	}
	for _, candidate := range []string{"wkhtmltopdf", "chromium", "chromium-browser", "/snap/bin/chromium"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return &Renderer{Binary: path, TempDir: tempDir}
		}
	}
	return &Renderer{}
}

var letterTemplate = template.Must(template.New("letter").Parse(`<!doctype html>
<html lang="id"><head><meta charset="utf-8"><style>
@page { size: A4; margin: 18mm 20mm 18mm 25mm; }
body { font-family: DejaVu Sans, Arial, sans-serif; font-size: 12pt; color: #111; }
.header { text-align:center; border-bottom:3px double #111; padding-bottom:8px; }
.header h1 { font-size:15pt; margin:0; } .header p { margin:2px 0; }
.meta { margin-top:22px; } table { border-collapse:collapse; width:100%; }
td { vertical-align:top; padding:3px 0; } td:first-child { width:36%; }
.subject { margin:22px 0 16px; } .body { line-height:1.6; text-align:justify; }
.signature { margin-top:45px; margin-left:58%; text-align:center; width:40%; }
.signature-space { height:70px; } .footer { margin-top:35px; font-size:8pt; color:#555; text-align:center; }
</style></head><body>
<div class="header"><h1>PEMERINTAH KABUPATEN GROBOGAN</h1><h1>DINAS PENDIDIKAN</h1><p>Jl. Bhayangkara No. 1 Purwodadi</p></div>
<div class="meta"><table><tr><td>Nomor</td><td>: {{.Number}}</td></tr><tr><td>Lampiran</td><td>: -</td></tr><tr><td>Perihal</td><td>: Kenaikan Gaji Berkala</td></tr></table></div>
<div class="subject">Kepada Yth.<br>Nama: <b>{{.TeacherName}}</b><br>NIP: {{.NIP}}<br>Unit kerja: {{.UnitName}}</div>
<div class="body"><p>Dengan ini diberitahukan bahwa berdasarkan ketentuan yang berlaku, kepada pegawai tersebut diberikan kenaikan gaji berkala terhitung mulai tanggal <b>{{.ProposedTMT}}</b>.</p><p>Gaji pokok lama sebesar <b>Rp {{.CurrentSalary}}</b> menjadi gaji pokok baru sebesar <b>Rp {{.NextSalary}}</b>.</p><p>Demikian surat ini dibuat untuk dipergunakan sebagaimana mestinya.</p></div>
<div class="signature">Purwodadi, {{.IssuedAt}}<br>Kepala Dinas Pendidikan<div class="signature-space"></div><b>{{.SignerName}}</b></div>
<div class="footer">Dokumen ini diterbitkan secara elektronik oleh SI CENDIKIA dan ditandatangani dengan Tanda Tangan Elektronik.</div>
</body></html>`))

// RenderLetter membuat PDF immutable untuk konsep atau surat final.
func (r *Renderer) RenderLetter(ctx context.Context, data LetterData) ([]byte, error) {
	if r == nil || r.Binary == "" {
		return nil, errors.New("binary PDF renderer tidak ditemukan; set PDF_BIN")
	}
	dir, err := os.MkdirTemp(r.TempDir, "si-cendikia-pdf-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	htmlPath := filepath.Join(dir, "letter.html")
	pdfPath := filepath.Join(dir, "letter.pdf")
	f, err := os.Create(htmlPath)
	if err != nil {
		return nil, err
	}
	if err := letterTemplate.Execute(f, data); err != nil {
		_ = f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	name := strings.ToLower(filepath.Base(r.Binary))
	var cmd *exec.Cmd
	if strings.Contains(name, "chrom") {
		profileDir := filepath.Join(dir, "profile")
		if err := os.MkdirAll(profileDir, 0o700); err != nil {
			return nil, fmt.Errorf("buat profile PDF: %w", err)
		}
		cmd = exec.CommandContext(ctx, r.Binary, "--headless", "--no-sandbox", "--disable-gpu", "--no-pdf-header-footer", "--user-data-dir="+profileDir, "--print-to-pdf="+pdfPath, "file://"+htmlPath)
	} else {
		cmd = exec.CommandContext(ctx, r.Binary, "--quiet", htmlPath, pdfPath)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("render PDF: %w: %s", err, strings.TrimSpace(string(output)))
	}
	body, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("baca PDF hasil render: %w", err)
	}
	if len(body) < 5 || string(body[:5]) != "%PDF-" {
		return nil, errors.New("renderer menghasilkan file bukan PDF")
	}
	return body, nil
}

// TodayID adalah tanggal surat yang aman untuk tampilan lokal.
func TodayID() string { return time.Now().Format("02 January 2006") }
