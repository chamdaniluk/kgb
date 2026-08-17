// Package pdf merender naskah SK KGB dari template DOCX dinas.
package pdf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LetterData membawa snapshot naskah yang sudah divalidasi.
type LetterData struct {
	Number              string
	TanggalNaskah       string
	IssuedAt            string
	ASNType             string
	TeacherName         string
	NIP                 string
	Karpeg              string
	BirthPlace          string
	BirthDate           string
	Pangkat             string
	PangkatGol          string
	Jabatan             string
	UnitName            string
	CurrentSalary       string
	NextSalary          string
	MasaKerjaLamaTahun  int
	MasaKerjaLamaBulan  int
	MasaKerjaBaruTahun  int
	MasaKerjaBaruBulan  int
	Golongan            string
	ProposedTMT         string
	NextKGBDate         string
	LastSKPejabat       string
	LastSKTanggal       string
	LastSKNomor         string
	LastSKTMTBerlaku    string
	MasaPerjanjian      string
	PerpanjanganKontrak string
	SignerName          string
	SignerNIP           string
	SignerJob           string
}

type Renderer struct {
	Binary      string
	TempDir     string
	TemplateDir string
}

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
	templateDir := os.Getenv("SK_TEMPLATE_DIR")
	if templateDir == "" {
		if _, file, _, ok := runtime.Caller(0); ok {
			templateDir = filepath.Join(filepath.Dir(file), "templates")
		}
	}
	if v := os.Getenv("PDF_BIN"); v != "" {
		return &Renderer{Binary: v, TempDir: tempDir, TemplateDir: templateDir}
	}
	for _, candidate := range []string{"soffice", "libreoffice"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return &Renderer{Binary: path, TempDir: tempDir, TemplateDir: templateDir}
		}
	}
	return &Renderer{TempDir: tempDir, TemplateDir: templateDir}
}

func (r *Renderer) templatePath(kind string) string {
	name := "pns.docx"
	if strings.ToLower(kind) == "pppk" {
		name = "pppk.docx"
	}
	return filepath.Join(r.TemplateDir, name)
}

func letterValues(data LetterData) map[string]string {
	pangkatJabatan := strings.TrimSpace(data.Pangkat)
	if data.PangkatGol != "" {
		if pangkatJabatan != "" {
			pangkatJabatan = pangkatJabatan + " (" + data.PangkatGol + ")"
		} else {
			pangkatJabatan = data.PangkatGol
		}
	}
	if data.Jabatan != "" {
		if pangkatJabatan != "" {
			pangkatJabatan += " / " + data.Jabatan
		} else {
			pangkatJabatan = data.Jabatan
		}
	}
	signerEmployee := strings.TrimSpace(data.SignerNIP)
	ttd := strings.TrimSpace(data.SignerName)
	if signerEmployee != "" {
		if ttd != "" {
			ttd += "\n"
		}
		ttd += "NIP. " + FormatNIP(signerEmployee)
	}
	tahun := time.Now().Format("2006")
	if data.TanggalNaskah != "" {
		parts := strings.Fields(data.TanggalNaskah)
		if len(parts) > 0 {
			tahun = parts[len(parts)-1]
		}
	}
	perpanjangan := strings.TrimSpace(data.PerpanjanganKontrak)
	if perpanjangan == "" {
		perpanjangan = "-"
	}
	return map[string]string{
		"nama":                          data.TeacherName,
		"tempat_lahir":                  data.BirthPlace,
		"tanggal_lahir":                 data.BirthDate,
		"nip":                           FormatNIP(data.NIP),
		"karpeg":                        data.Karpeg,
		"pangkat_jabatan":               pangkatJabatan,
		"unit_kerja":                    data.UnitName,
		"gaji_lama":                     FormatRupiah(data.CurrentSalary),
		"gaji_baru":                     FormatRupiah(data.NextSalary),
		"previous_sk_official":          data.LastSKPejabat,
		"previous_sk_date":              data.LastSKTanggal,
		"previous_sk_number":            data.LastSKNomor,
		"previous_sk_effective_date":    data.LastSKTMTBerlaku,
		"previous_mkg_years":            pad2(data.MasaKerjaLamaTahun),
		"previous_mkg_months":           pad2(data.MasaKerjaLamaBulan),
		"next_mkg":                      pad2(data.MasaKerjaBaruTahun),
		"next_mkg_months":               pad2(data.MasaKerjaBaruBulan),
		"grade":                         data.Golongan,
		"effective_date":                data.ProposedTMT,
		"next_tmt":                      data.NextKGBDate,
		"nomor_naskah":                  data.Number,
		"tanggal_naskah":                data.TanggalNaskah,
		"tahun_naskah":                  tahun,
		"ttd_pengirim":                  ttd,
		"signer_name":                   data.SignerName,
		"signer_employee":               FormatNIP(signerEmployee),
		"signer_job":                    data.SignerJob,
		"masa_perjanjian_kerja":         data.MasaPerjanjian,
		"perpanjangan_perjanjian_kerja": perpanjangan,
	}
}

func RenderLetter(ctx context.Context, r *Renderer, data LetterData) ([]byte, error) {
	if r == nil || r.Binary == "" {
		return nil, errors.New("LibreOffice tidak ditemukan; set PDF_BIN")
	}
	if data.TanggalNaskah == "" {
		data.TanggalNaskah = TodayID()
	}
	if data.IssuedAt == "" {
		data.IssuedAt = data.TanggalNaskah
	}
	kind := "pns"
	if strings.ToLower(data.ASNType) == "pppk" {
		kind = "pppk"
	}
	templatePath := r.templatePath(kind)
	if _, err := os.Stat(templatePath); err != nil {
		return nil, fmt.Errorf("template SK %s tidak ditemukan: %w", kind, err)
	}
	dir, err := os.MkdirTemp(r.TempDir, "si-cendikia-pdf-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	docxPath := filepath.Join(dir, "letter.docx")
	if err := fillDOCX(templatePath, docxPath, letterValues(data)); err != nil {
		return nil, err
	}
	loHome := filepath.Join(dir, "lo-home")
	if err := os.MkdirAll(loHome, 0o700); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, r.Binary,
		"--headless", "--nologo", "--nofirststartwizard", "--norestore", "--invisible",
		"-env:UserInstallation=file://"+loHome,
		"--convert-to", "pdf", "--outdir", dir, docxPath)
	cmd.Env = append(os.Environ(),
		"HOME="+loHome,
		"XDG_CACHE_HOME="+loHome,
		"XDG_CONFIG_HOME="+loHome,
		"TMPDIR="+loHome,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("convert DOCX ke PDF: %w: %s", err, strings.TrimSpace(string(output)))
	}
	pdfPath := filepath.Join(dir, "letter.pdf")
	body, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("baca PDF hasil render: %w", err)
	}
	if len(body) < 5 || string(body[:5]) != "%PDF-" {
		return nil, errors.New("renderer menghasilkan file bukan PDF")
	}
	return body, nil
}
