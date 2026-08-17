package pdf

import (
	"context"
	"testing"
)

func TestRenderLetter(t *testing.T) {
	renderer := NewRenderer()
	if renderer.Binary == "" {
		t.Skip("LibreOffice tidak tersedia")
	}
	data, err := RenderLetter(context.Background(), renderer, LetterData{
		Number: "800/001/4.2/2026", TeacherName: "Guru Uji", NIP: "198001012005011001",
		UnitName: "Unit Uji", ProposedTMT: "01 September 2026", CurrentSalary: "3497300",
		NextSalary: "3607500", IssuedAt: "17 Agustus 2026", TanggalNaskah: "17 Agustus 2026",
		SignerName: "Kepala Dinas", SignerNIP: "196711271995121002", ASNType: "pns",
		Karpeg: "I 123456", BirthPlace: "Grobogan", BirthDate: "13 Februari 1990",
		Pangkat: "Penata Muda Tingkat I", PangkatGol: "III/b", Jabatan: "Guru Ahli Pertama",
		LastSKPejabat: "Kepala Dinas Pendidikan", LastSKTanggal: "01 September 2024",
		LastSKNomor: "800/010/4.2/2024", LastSKTMTBerlaku: "01 September 2024",
		MasaKerjaLamaTahun: 10, MasaKerjaLamaBulan: 0, MasaKerjaBaruTahun: 12, MasaKerjaBaruBulan: 0,
		Golongan: "III/b", NextKGBDate: "01 September 2028",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		t.Fatalf("hasil bukan PDF valid: %d bytes", len(data))
	}
}
