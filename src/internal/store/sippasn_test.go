package store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMapSIPPASNOfficerGuruPNS(t *testing.T) {
	officer := SIPPASNOfficer{
		NIP:      "199103312005011076",
		Name:     "GURU CONTOH, S.Pd.",
		JobKind:  "2",
		JobTitle: "Guru Ahli Muda",
		Golongan: "III/c",
		Pangkat:  "Penata",
		UnitName: "SDN 3 Krangganharjo - Dinas Pendidikan",
		Status:   "1",
	}
	got, ok := MapSIPPASNOfficer(officer, SIPPASNSyncConfig{HanyaUnitPendidikan: true, HanyaStatusAktif: true})
	if !ok {
		t.Fatal("guru PNS aktif pendidikan harus layak sinkron")
	}
	if got.ASNType != "pns" || got.PangkatGol != "III/c" {
		t.Fatalf("asn=%q gol=%q, want pns/III/c", got.ASNType, got.PangkatGol)
	}
	if got.Jabatan != "Guru Ahli Muda" || got.Name != "GURU CONTOH, S.Pd." {
		t.Fatalf("jabatan/nama = %q/%q", got.Jabatan, got.Name)
	}
	if got.MasaKerjaSource != "sippasn" || got.MasaKerjaTahun <= 0 {
		t.Fatalf("masa kerja = %d/%q, want >0/sippasn", got.MasaKerjaTahun, got.MasaKerjaSource)
	}
	if err := ValidateImportedTeacher(got); err != nil {
		t.Fatalf("hasil map harus lolos validasi impor: %v", err)
	}
}

func TestMapSIPPASNOfficerPPPKBaruTanpaGolongan(t *testing.T) {
	officer := SIPPASNOfficer{
		NIP:      "199106242025212070",
		Name:     "PPPK BARU, S.Pd",
		JobKind:  "2",
		JobTitle: "Guru Ahli Pertama",
		UnitName: "SDN 3 Lemahputih - Dinas Pendidikan",
		Status:   "1",
	}
	got, ok := MapSIPPASNOfficer(officer, SIPPASNSyncConfig{HanyaUnitPendidikan: true, HanyaStatusAktif: true})
	if !ok {
		t.Fatal("guru PPPK baru tanpa golongan harus layak sinkron")
	}
	if got.ASNType != "pppk" || got.PangkatGol != "IX" {
		t.Fatalf("asn=%q gol=%q, want pppk/IX", got.ASNType, got.PangkatGol)
	}
	if err := ValidateImportedTeacher(got); err != nil {
		t.Fatalf("hasil map harus lolos validasi impor: %v", err)
	}
}

func TestMapSIPPASNOfficerNonGuruDisdik(t *testing.T) {
	cfg := SIPPASNSyncConfig{HanyaUnitPendidikan: true, HanyaStatusAktif: true}
	// PNS non-guru: golongan SIPPASN dipakai apa adanya.
	got, ok := MapSIPPASNOfficer(SIPPASNOfficer{
		NIP: "197801012006041002", Name: "PENGAWAS CONTOH", JobKind: "2",
		JobTitle: "Pengawas Sekolah Ahli Muda", Golongan: "III/c",
		UnitName: "Koordinator Wilayah Kecamatan Bidang Pendidikan Kecamatan Grobogan - Dinas Pendidikan", Status: "1",
	}, cfg)
	if !ok {
		t.Fatal("pengawas PNS aktif pendidikan harus layak sinkron")
	}
	if got.ASNType != "pns" || got.Kategori != KategoriNonGuru || got.PangkatGol != "III/c" {
		t.Fatalf("asn=%q kat=%q gol=%q, want pns/non_guru/III/c", got.ASNType, got.Kategori, got.PangkatGol)
	}
	if err := ValidateImportedTeacher(got); err != nil {
		t.Fatalf("hasil map harus lolos validasi impor: %v", err)
	}
	// PPPK tercatat dengan padanan PNS (mis. II/c): default IX, dapat
	// diubah saat usul KGB sesuai SK pengangkatan.
	got, ok = MapSIPPASNOfficer(SIPPASNOfficer{
		NIP: "199001012025212001", Name: "OPERATOR CONTOH", JobKind: "3",
		JobTitle: "Operator Layanan Operasional", Golongan: "II/c",
		UnitName: "Dinas Pendidikan", Status: "1",
	}, cfg)
	if !ok {
		t.Fatal("operator PPPK bergolongan harus layak sinkron")
	}
	if got.ASNType != "pppk" || got.Kategori != KategoriNonGuru || got.PangkatGol != "IX" {
		t.Fatalf("asn=%q kat=%q gol=%q, want pppk/non_guru/IX", got.ASNType, got.Kategori, got.PangkatGol)
	}
	if err := ValidateImportedTeacher(got); err != nil {
		t.Fatalf("hasil map harus lolos validasi impor: %v", err)
	}
	// PPPK tanpa golongan: default IX (dapat diubah saat usul KGB).
	got, ok = MapSIPPASNOfficer(SIPPASNOfficer{
		NIP: "199001012025212002", Name: "OPERATOR BARU", JobKind: "3",
		JobTitle: "Operator Layanan Operasional",
		UnitName: "Dinas Pendidikan", Status: "1",
	}, cfg)
	if !ok {
		t.Fatal("operator PPPK tanpa golongan harus layak sinkron")
	}
	if got.ASNType != "pppk" || got.PangkatGol != "IX" {
		t.Fatalf("asn=%q gol=%q, want pppk/IX", got.ASNType, got.PangkatGol)
	}
}

func TestMapSIPPASNOfficerMenolakDiLuarDisdik(t *testing.T) {
	cases := []SIPPASNOfficer{
		{NIP: "198001012005011001", Name: "Struktural", JobKind: "20", JobTitle: "Sekretaris Daerah", Golongan: "IV/d", UnitName: "Sekretariat Daerah", Status: "1"},
		{NIP: "198001012005011003", Name: "Perawat", JobKind: "2", JobTitle: "Perawat Terampil", Golongan: "II/c", UnitName: "RSUD", Status: "1"},
		{NIP: "198001012005011004", Name: "Guru Nonaktif", JobKind: "2", JobTitle: "Guru Ahli Pertama", Golongan: "III/a", UnitName: "SDN 1 X - Dinas Pendidikan", Status: "2"},
		{NIP: "198001012005011005", Name: "Guru Luar Dinas", JobKind: "2", JobTitle: "Guru Ahli Pertama", Golongan: "III/a", UnitName: "SMA Negeri 1 X", Status: "1"},
		{NIP: "pendek", Name: "NIP Rusak", JobKind: "2", JobTitle: "Guru Ahli Pertama", Golongan: "III/a", UnitName: "SDN 1 X - Dinas Pendidikan", Status: "1"},
	}
	cfg := SIPPASNSyncConfig{HanyaUnitPendidikan: true, HanyaStatusAktif: true}
	for i, c := range cases {
		if _, ok := MapSIPPASNOfficer(c, cfg); ok {
			t.Errorf("kasus %d (%s) seharusnya ditolak", i, c.Name)
		}
	}
}

func TestASNTypeFromNIP(t *testing.T) {
	cases := []struct {
		nip  string
		want string
		ok   bool
	}{
		{"199103312005011076", "pns", true},  // segmen 01
		{"198001012010121002", "pns", true},  // segmen 12
		{"199001012025212001", "pppk", true}, // segmen 21
		{"199001012025222001", "pppk", true}, // segmen 22
		{"199001012025132001", "", false},    // segmen 13 tak dikenal
		{"pendek", "", false},
	}
	for _, c := range cases {
		got, ok := ASNTypeFromNIP(c.nip)
		if got != c.want || ok != c.ok {
			t.Errorf("NIP %s = %q/%v, want %q/%v", c.nip, got, ok, c.want, c.ok)
		}
	}
}

func TestFetchOfficersMemakaiDecoderToleran(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pegawai" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Karakter tab mentah di dalam string: tidak valid untuk encoding/json ketat.
		_, _ = w.Write([]byte("{\"pegawai\": [{\"status\":\"1\",\t\"nip_pejabat\":\"199103312005011076\", \"nama_pejabat\":\"GURU CONTOH\", \"kd_jns_jab\":\"2\", \"kd_jabatan\":\"30001\", \"jabatan\":\"Guru Ahli Muda\", \"kd_golongan\":\"32\", \"golongan\":\"III/c\", \"pangkat\":\"Penata\", \"kd_unker\":\"03.08.46\", \"unker\":\"SDN 3 Krangganharjo - Dinas Pendidikan\", \"kd_esselon\":\"\", \"esselon\":\"\"}]}"))
	}))
	defer srv.Close()
	client := SIPPASNClient{BaseURL: srv.URL, HTTP: srv.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	got, err := client.FetchOfficers(ctx)
	if err != nil {
		t.Fatalf("FetchOfficers: %v", err)
	}
	if len(got) != 1 || got[0].NIP != "199103312005011076" || got[0].Golongan != "III/c" {
		t.Fatalf("hasil = %#v", got)
	}
}

func TestFetchOfficersStatusNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	client := SIPPASNClient{BaseURL: srv.URL, HTTP: srv.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := client.FetchOfficers(ctx); err == nil {
		t.Fatal("status 500 harus mengembalikan error")
	}
}

func TestSIPPASNSnapshotStaleSaatUpstreamMati(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"pegawai": [{"status":"1","nip_pejabat":"199103312005011076","nama_pejabat":"GURU CONTOH","kd_jns_jab":"2","kd_jabatan":"30001","jabatan":"Guru Ahli Muda","kd_golongan":"32","golongan":"III/c","pangkat":"Penata","kd_unker":"03.08.46","unker":"SDN 3 Krangganharjo - Dinas Pendidikan","kd_esselon":"","esselon":""}]}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	snap := NewSIPPASNSnapshot(SIPPASNClient{BaseURL: srv.URL, HTTP: srv.Client()}, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := snap.Ensure(ctx); err != nil {
		t.Fatalf("ensure pertama: %v", err)
	}
	if _, ok := snap.Lookup("199103312005011076"); !ok {
		t.Fatal("lookup setelah ensure pertama harus ketemu")
	}
	// Kadaluarsakan paksa lalu matikan upstream: data lama harus tetap dipakai.
	snap.mu.Lock()
	snap.fetchedAt = time.Now().Add(-2 * time.Hour)
	snap.mu.Unlock()
	if err := snap.Ensure(ctx); err != nil {
		t.Fatalf("ensure saat upstream mati harus memakai data lama: %v", err)
	}
	if _, ok := snap.Lookup("199103312005011076"); !ok {
		t.Fatal("lookup stale harus tetap ketemu")
	}
}
