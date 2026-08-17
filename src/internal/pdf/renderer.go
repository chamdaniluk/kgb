// Package pdf merender naskah SK KGB sesuai contoh PNS/PPPK.
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

// LetterData membawa snapshot naskah; dikosongkan jadi "—" bila nil.
type LetterData struct {
	Number             string
	TanggalNaskah      string
	IssuedAt           string
	ASNType            string // pns|pppk
	TeacherName        string
	NIP                string
	Karpeg             string
	BirthPlace         string
	BirthDate          string
	Pangkat            string
	PangkatGol         string
	Jabatan            string
	UnitName           string
	CurrentSalary      string
	NextSalary         string
	MasaKerjaLamaTahun int
	MasaKerjaLamaBulan int
	MasaKerjaBaruTahun int
	MasaKerjaBaruBulan int
	Golongan           string
	ProposedTMT        string
	NextKGBDate        string
	LastSKPejabat      string
	LastSKTanggal      string
	LastSKNomor        string
	LastSKTMTBerlaku   string
	SignerName         string
	SignerNIP          string
}

// Renderer menjalankan weasyprint atau Chromium headless.
type Renderer struct {
	Binary  string
	TempDir string
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

var pnsTemplate = template.Must(template.New("pns").Parse(`<!doctype html>
<html lang="id"><head><meta charset="utf-8"><style>
@page { size: A4; margin: 18mm 20mm 18mm 25mm; }
body { font-family: Arial, sans-serif; font-size: 11pt; color: #111; line-height: 1.35; }
.header { text-align:center; border-bottom:3px double #111; padding-bottom:8px; }
.header h1 { font-size:14pt; margin:0; letter-spacing:.02em; }
.header h2 { font-size:18pt; margin:2px 0; }
.header p { margin:2px 0; font-size:10pt; }
.meta { margin-top:14px; }
.meta table, .data table { border-collapse:collapse; width:100%; }
.meta td { padding:2px 4px; vertical-align:top; }
.data td { padding:3px 6px; vertical-align:top; border:1px solid #ddd; }
.data td:first-child { width:28px; text-align:right; }
.data td:nth-child(2) { width:200px; }
.data td:nth-child(3) { width:12px; text-align:center; }
.justify { text-align:justify; }
.center { text-align:center; }
.small { font-size:10pt; color:#333; }
.signature { margin-top:36px; margin-left:52%; text-align:center; }
.signature-space { height:68px; }
.tembusan { margin-top:28px; font-size:9.5pt; }
.footer { margin-top:28px; font-size:8pt; color:#555; text-align:center; border-top:1px solid #e5e7eb; padding-top:8px; }
</style></head><body>
<div class="header"><h1>PEMERINTAH KABUPATEN GROBOGAN</h1><h2>DINAS PENDIDIKAN</h2><p>Jalan Pemuda Nomor 35, Purwodadi, Grobogan, Jawa Tengah 58111</p><p>Telepon (0292) 421034, Faksimile (0292) 421034</p><p>Laman disdik.grobogan.go.id, Pos-el disdik@grobogan.go.id</p></div>
<div class="meta"><table>
<tr><td></td><td></td><td></td><td style="text-align:right">Purwodadi, {{.TanggalNaskah}}</td></tr>
<tr><td>Nomor</td><td>:</td><td colspan="2">{{.Number}}</td></tr>
<tr><td>Lampiran</td><td>:</td><td colspan="2">-</td></tr>
<tr><td>Perihal</td><td>:</td><td colspan="2">Kenaikan Gaji Berkala / a.n. {{.TeacherName}}</td></tr>
</table></div>
<p>Yth. Kepala BPPKAD Kabupaten Grobogan<br>di —<br>Purwodadi</p>
<p class="justify">Bersama ini diberitahukan bahwa dengan telah terpenuhinya masa kerja golongan dan syarat - syarat lainnya kepada</p>
<table class="data" style="margin-top:8px;">
<tr><td>1.</td><td>Nama</td><td>:</td><td>{{.TeacherName}}</td></tr>
<tr><td>2.</td><td>Tempat / Tgl. Lahir</td><td>:</td><td>{{.BirthPlace}}, {{.BirthDate}}</td></tr>
<tr><td>3.</td><td>NIP / Karpeg</td><td>:</td><td>{{.NIP}} / {{.Karpeg}}</td></tr>
<tr><td>4.</td><td>Pangkat / Jabatan</td><td>:</td><td>{{.Pangkat}} ({{.PangkatGol}}) / {{.Jabatan}}</td></tr>
<tr><td>5.</td><td>Kantor / Tempat bekerja</td><td>:</td><td>{{.UnitName}} - Dinas Pendidikan Kab. Grobogan</td></tr>
<tr><td>6.</td><td>Gaji Pokok lama</td><td>:</td><td>Rp {{.CurrentSalary}}</td></tr>
<tr><td></td><td colspan="3" class="small">(Atas dasar Surat Keputusan terakhir tentang Gaji / Pangkat yang ditetapkan)</td></tr>
<tr><td></td><td>Oleh Pejabat</td><td>:</td><td>{{.LastSKPejabat}}</td></tr>
<tr><td></td><td>Tanggal</td><td>:</td><td>{{.LastSKTanggal}}</td></tr>
<tr><td></td><td>Nomor</td><td>:</td><td>{{.LastSKNomor}}</td></tr>
<tr><td></td><td>Tgl. mulai berlakunya SK tersebut</td><td>:</td><td>{{.LastSKTMTBerlaku}}</td></tr>
<tr><td></td><td>Masa kerja golongan pada tgl. tsb</td><td>:</td><td>{{.MasaKerjaLamaTahun}} Tahun {{.MasaKerjaLamaBulan}} Bulan</td></tr>
<tr><td></td><td colspan="3" style="text-align:center; font-weight:700; padding:8px 0;">DIBERIKAN KENAIKAN GAJI BERKALA HINGGA MEMPEROLEH</td></tr>
<tr><td>7.</td><td>Gaji Pokok Baru</td><td>:</td><td>Rp {{.NextSalary}}</td></tr>
<tr><td>8.</td><td>Berdasarkan Masa Kerja</td><td>:</td><td>{{.MasaKerjaBaruTahun}} Tahun {{.MasaKerjaBaruBulan}} Bulan</td></tr>
<tr><td>9.</td><td>Dalam Golongan</td><td>:</td><td>{{.Golongan}}</td></tr>
<tr><td>10.</td><td>Mulai Berlakunya</td><td>:</td><td>{{.ProposedTMT}}</td></tr>
<tr><td>11.</td><td>Keterangan</td><td>:</td><td>Kenaikan gaji yang akan datang tanggal : {{.NextKGBDate}}</td></tr>
</table>
<p class="justify">Diharapkan agar kepada pegawai tersebut dapat dibayarkan penghasilannya berdasarkan gaji pokok yang baru.</p>
<div class="signature">Kepala Dinas Pendidikan<br>Kabupaten Grobogan<br><div class="signature-space"></div><b>{{.SignerName}}</b>{{if .SignerNIP}}<br>NIP. {{.SignerNIP}}{{end}}</div>
<div class="tembusan"><b>Tembusan :</b><ol><li>Kepala Kantor Cabang Utama PT. Taspen (Persero) di Semarang</li><li>Kepala Dinas Pendidikan Kabupaten Grobogan</li><li>Pegawai yang bersangkutan</li></ol></div>
<div class="footer">Dokumen diterbitkan secara elektronik oleh SI CENDIKIA dan ditandatangani dengan Tanda Tangan Elektronik.</div>
</body></html>`))

var pppkTemplate = template.Must(template.New("pppk").Parse(`<!doctype html>
<html lang="id"><head><meta charset="utf-8"><style>
@page { size: A4; margin: 18mm 14mm 18mm 18mm; }
body { font-family: Arial, sans-serif; font-size: 10.5pt; color: #111; line-height:1.32; }
.header { text-align:center; border-bottom:3px double #111; padding-bottom:8px; }
.header h1 { font-size:13.5pt; margin:0; }
.header h2 { font-size:17pt; margin:2px 0; }
.header p { margin:1px 0; font-size:9.5pt; }
h3 { text-align:center; margin:10px 0 2px; font-size:12pt; }
.center { text-align:center; }
.small { font-size:9.5pt; }
table.grid { border-collapse:collapse; width:100%; }
table.grid td, table.grid th { border:1px solid #bbb; padding:4px 5px; vertical-align:top; }
.noborder td { border:none !important; padding:2px 4px; }
.justify { text-align:justify; }
.signature { margin-top:28px; margin-left:52%; text-align:center; }
.signature-space { height:64px; }
.footer { margin-top:18px; font-size:8pt; color:#555; text-align:center; border-top:1px solid #e5e7eb; padding-top:6px; }
</style></head><body>
<div class="header"><h1>PEMERINTAH KABUPATEN GROBOGAN</h1><h2>DINAS PENDIDIKAN</h2><p>Jalan Pemuda Nomor 35, Purwodadi, Grobogan, Jawa Tengah 58111</p><p>Telepon (0292) 421034, Faksimile (0292) 421034</p><p>Laman disdik.grobogan.go.id, Pos-el disdik@grobogan.go.id</p></div>
<h3>KEPUTUSAN<br>KEPALA DINAS PENDIDIKAN KABUPATEN GROBOGAN<br>NOMOR {{.Number}}</h3>
<p class="center"><b>TENTANG<br>KENAIKAN GAJI BERKALA</b><br>DENGAN RAHMAT TUHAN YANG MAHA ESA<br>KEPALA DINAS PENDIDIKAN KABUPATEN GROBOGAN</p>
<table class="noborder" style="margin-top:10px;">
<tr><td style="width:90px;">Menimbang</td><td>:</td><td class="justify">bahwa untuk melaksanakan ketentuan Pasal 3 Peraturan Menteri Pendayagunaan Aparatur Negara dan Reformasi Birokrasi Nomor 7 Tahun 2023 tentang Kenaikan Gaji Berkala dan Kenaikan Gaji Istimewa Pegawai Pemerintah dengan Perjanjian Kerja, perlu menetapkan Keputusan Kepala Dinas Pendidikan Kabupaten Grobogan tentang Kenaikan Gaji Berkala;</td></tr>
<tr><td>Mengingat</td><td>:</td><td class="small">1. Undang-Undang Nomor 5 Tahun 2014 tentang Aparatur Sipil Negara; 2. Peraturan Pemerintah Nomor 49 Tahun 2018 tentang Manajemen PPPK; 3. Peraturan Presiden Nomor 47 Tahun 2021; 4. Peraturan Menteri PANRB Nomor 60 Tahun 2021 jo 39 Tahun 2022; 5. Peraturan Menteri PAN Nomor 7 Tahun 2023;</td></tr>
</table>
<p class="center"><b>MEMUTUSKAN:</b></p>
<table class="grid">
<tr><td style="width:80px;">Menetapkan</td><td>:</td><td colspan="3">KEPUTUSAN KEPALA DINAS PENDIDIKAN KABUPATEN GROBOGAN NOMOR {{.Number}} TAHUN {{.TanggalNaskah}} TENTANG KENAIKAN GAJI BERKALA.</td></tr>
<tr><td>KESATU</td><td>:</td><td colspan="3" class="justify">Memberikan Kenaikan Gaji Berkala kepada Pegawai Pemerintah dengan Perjanjian Kerja di bawah ini :</td></tr>
<tr><td></td><td></td><td>Nama / NIP / Golongan-Jabatan</td><td>:</td><td>{{.TeacherName}} / {{.NIP}} / {{.PangkatGol}}-{{.Jabatan}}</td></tr>
<tr><td></td><td></td><td>Kantor / Gaji Lama</td><td>:</td><td>{{.UnitName}} / Rp {{.CurrentSalary}} (Atas dasar SK terakhir: {{.LastSKPejabat}}, {{.LastSKTanggal}}, No. {{.LastSKNomor}}, TMT {{.LastSKTMTBerlaku}}, Masa kerja {{.MasaKerjaLamaTahun}} th {{.MasaKerjaLamaBulan}} bln)</td></tr>
<tr><td></td><td></td><td colspan="3" style="text-align:center; font-weight:700;">diberikan kenaikan gaji berkala hingga memperoleh :</td></tr>
<tr><td></td><td></td><td>Gaji Baru / Masa kerja / Mulai tanggal</td><td>:</td><td>Rp {{.NextSalary}} / {{.MasaKerjaBaruTahun}} th {{.MasaKerjaBaruBulan}} bln / {{.ProposedTMT}}</td></tr>
<tr><td>KEDUA</td><td>:</td><td colspan="3" class="justify">Petikan keputusan ini diberikan kepada yang bersangkutan, dan yang berkepentingan untuk dapat diketahui dan dipergunakan sebagaimana mestinya.</td></tr>
<tr><td>KETIGA</td><td>:</td><td colspan="3" class="justify">Segala biaya yang diperlukan dalam rangka pelaksanaan Keputusan ini, dibebankan pada APBD Kabupaten Grobogan.</td></tr>
<tr><td>KEEMPAT</td><td>:</td><td colspan="3" class="justify">Keputusan Kepala Dinas Pendidikan Kabupaten Grobogan ini mulai berlaku sejak tanggal ditetapkan.</td></tr>
</table>
<p>Ditetapkan di Purwodadi<br>Pada tanggal {{.TanggalNaskah}}</p>
<div class="signature">Kepala Dinas Pendidikan<br>Kabupaten Grobogan<br><div class="signature-space"></div><b>{{.SignerName}}</b>{{if .SignerNIP}}<br>Pembina Utama Muda<br>NIP. {{.SignerNIP}}{{end}}</div>
<div class="small" style="margin-top:18px;"><b>Tembusan :</b> 1. Kepala Kantor Regional I BKN di Yogyakarta, 2. Bupati Grobogan, 3. Kepala Dinas KOMINFO Kab. Grobogan, 4. Kepala Dinas Pendidikan Kab. Grobogan, 5. Pegawai yang bersangkutan, 6. Arsip</div>
<div class="footer">Dokumen diterbitkan secara elektronik oleh SI CENDIKIA dan ditandatangani dengan Tanda Tangan Elektronik.</div>
</body></html>`))

func dash(v string) string {
	if v == "" {
		return "—"
	}
	return v
}

func fmtTanggalID(t time.Time) string {
	return t.Format("02 January 2006")
}

func fmtRupiah(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, ".", "")
	return v
}

func RenderLetter(ctx context.Context, r *Renderer, data LetterData) ([]byte, error) {
	if r == nil || r.Binary == "" {
		return nil, errors.New("binary PDF renderer tidak ditemukan; set PDF_BIN")
	}
	// normalisasi kosong -> strip
	data.Number = strings.TrimSpace(data.Number)
	data.TeacherName = strings.TrimSpace(data.TeacherName)
	tmpl := pnsTemplate
	if strings.ToLower(data.ASNType) == "pppk" {
		tmpl = pppkTemplate
	}
	if data.TanggalNaskah == "" {
		data.TanggalNaskah = fmtTanggalID(time.Now())
	}
	if data.IssuedAt == "" {
		data.IssuedAt = data.TanggalNaskah
	}
	data.Karpeg = dash(data.Karpeg)
	data.BirthPlace = dash(data.BirthPlace)
	data.BirthDate = dash(data.BirthDate)
	data.LastSKPejabat = dash(data.LastSKPejabat)
	data.LastSKTanggal = dash(data.LastSKTanggal)
	data.LastSKNomor = dash(data.LastSKNomor)
	data.LastSKTMTBerlaku = dash(data.LastSKTMTBerlaku)
	data.Pangkat = dash(data.Pangkat)
	data.Jabatan = dash(data.Jabatan)
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
	if err := tmpl.Execute(f, data); err != nil {
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
	// weasyprint: binary is python script, Exec works directly
	if strings.Contains(name, "weasyprint") {
		cmd = exec.CommandContext(ctx, r.Binary, htmlPath, pdfPath)
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

func TodayID() string { return time.Now().Format("02 January 2006") }
