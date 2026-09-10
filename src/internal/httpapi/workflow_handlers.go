package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sicendikia/internal/esign"
	"sicendikia/internal/files"
	"sicendikia/internal/letterdata"
	"sicendikia/internal/pdf"
	"sicendikia/internal/store"
)

const maxMultipartMemory = 6 << 20

func parsePathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("id tidak valid")
	}
	return id, nil
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func mapStoreError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "Data tidak ditemukan.")
	case errors.Is(err, store.ErrForbidden):
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Data bukan dalam kewenangan Anda.")
	case errors.Is(err, store.ErrConflict):
		writeErr(w, http.StatusConflict, "INVALID_STATUS_TRANSITION", "Status pengajuan sudah berubah atau tidak sesuai. Bila Anda masih punya pengajuan aktif, tunggu prosesnya selesai sebelum mengusulkan lagi.")
	case errors.Is(err, letterdata.ErrDraftIncomplete):
		writeErr(w, http.StatusUnprocessableEntity, "LETTER_DRAFT_INCOMPLETE", err.Error())
	default:
		return false
	}
	return true
}

func formatTanggalID(t *time.Time) string {
	if t == nil {
		return ""
	}
	return pdf.FormatTanggalID(*t)
}

func (s *Server) teacherForUser(r *http.Request) (store.Teacher, bool) {
	t, err := store.GetTeacherByUserID(r.Context(), s.Pool, userFrom(r).ID)
	if err != nil {
		return store.Teacher{}, false
	}
	return t, true
}

type preparedKGB struct {
	ProposedTMT time.Time
	LastTMT     *time.Time
	TMTAwal     *time.Time
	MasaBaru    int
	Draft       store.LetterDraft
	Current     string
	Next        string
}

func (s *Server) prepareKGBFromForm(r *http.Request, t store.Teacher, asOf time.Time) (preparedKGB, error) {
	proposedLast, err := parseOptionalFormDate(firstNonEmptyForm(r, "last_sk_tmt", "tmt_kgb_last"))
	if err != nil {
		return preparedKGB{}, errors.New("TMT SK terakhir tidak valid")
	}
	// TMT awal (CPNS/pengangkatan) — acuan masa kerja & gaji, tidak dicetak.
	tmtAwal, err := parseOptionalFormDate(firstNonEmptyForm(r, "tmt_awal"))
	if err != nil {
		return preparedKGB{}, errors.New("TMT awal tidak valid")
	}
	if tmtAwal == nil {
		tmtAwal = t.TMTAwal
	}
	if tmtAwal == nil {
		return preparedKGB{}, errors.New("TMT CPNS/pengangkatan wajib diisi sebagai acuan gaji")
	}
	draft, err := letterDraftFromForm(r, t, letterdata.EvenYear(t.MasaKerjaTahun))
	if err != nil {
		return preparedKGB{}, err
	}
	// KP terakhir (PNS) / SK Pertama atau Perpanjangan Kontrak (PPPK):
	// seluruh kolom wajib. Golongan dikunci SIPPASN untuk PNS; PPPK bebas
	// I-XVII sesuai SK. Bila SK ini lebih baru dari KGB terakhir, gaji KGB
	// mengacu golongan SK ini; jangka waktu KGB tetap 2 tahun dari KGB.
	kp, err := parseKPLast(r)
	if err != nil {
		return preparedKGB{}, err
	}
	if kp.HasData() {
		if t.ASNType == "pns" && kp.Golongan != t.PangkatGol {
			return preparedKGB{}, errors.New("golongan KP mengikuti SIPPASN dan tidak dapat diubah")
		}
		if t.ASNType == "pppk" && !store.ValidPPPKGolongan(kp.Golongan) {
			return preparedKGB{}, errors.New("golongan KP PPPK harus I sampai XVII")
		}
	}
	draft.LastKPGolongan = kp.Golongan
	draft.LastKPTMT = kp.TMT
	draft.LastKPMasaTahun = kp.MasaTahun
	draft.LastKPMasaBulan = kp.MasaBulan
	draft.LastKPNomor = kp.Nomor
	draft.LastKPTanggal = kp.Tanggal
	draft.LastKPPejabat = kp.Pejabat
	// KGB terakhir: golongan ruang saat KGB terakhir dapat diubah (bisa
	// berbeda dari KP bila ada perubahan di tengah masa KGB); masa kerja
	// mengikuti SK yang diterbitkan.
	kgb, err := parseKGBLast(r, t)
	if err != nil {
		return preparedKGB{}, err
	}
	if t.ASNType == "pns" && !validPNSGolongan(kgb.Golongan) {
		return preparedKGB{}, errors.New("golongan KGB tidak valid")
	}
	if t.ASNType == "pppk" {
		if !store.ValidPPPKGolongan(kgb.Golongan) {
			return preparedKGB{}, errors.New("golongan KGB PPPK harus I sampai XVII sesuai SK")
		}
		t.PangkatGol = kgb.Golongan
	}
	draft.LastSKTMT = kgb.TMT
	draft.LastSKMasaTahun = kgb.MasaTahun
	draft.LastSKMasaBulan = kgb.MasaBulan
	draft.LastSKNomor = kgb.Nomor
	draft.LastSKTanggal = kgb.Tanggal
	draft.LastSKPejabat = kgb.Pejabat
	// Naskah SK mengikuti SK terbaru (keputusan owner 2026-09-09): bandingkan
	// TANGGAL SK — KP (PNS) / SK Pertama-Perpanjangan Kontrak (PPPK) vs KGB.
	// Pemenang mengisi SELURUH kolom identitas SK terakhir naskah (pejabat,
	// tanggal, nomor, TMT). Masa kerja golongan lama tetap dari SK KGB
	// (keputusan owner); pangkat mengikuti golongan pemenang.
	kpMenang := kp.HasData() && store.SKNewer(kp.Tanggal, kgb.Tanggal)
	if kpMenang {
		draft.LastSKTMT = kp.TMT
		draft.LastSKNomor = kp.Nomor
		draft.LastSKTanggal = kp.Tanggal
		draft.LastSKPejabat = kp.Pejabat
		if t.ASNType == "pns" {
			draft.Pangkat = pangkatForGolongan(kp.Golongan)
		}
		t.PangkatGol = kp.Golongan
	} else {
		if t.ASNType == "pns" {
			draft.Pangkat = pangkatForGolongan(kgb.Golongan)
		}
		t.PangkatGol = kgb.Golongan
	}
	// Jabatan wajib sesuai kategori: guru dari jenjang fungsional guru,
	// non-guru dari daftar jabatan Disdik yang teramati di SIPPASN.
	if j := strings.TrimSpace(draft.Jabatan); j != "" && !validJabatanUntuk(j, t.Kategori) {
		return preparedKGB{}, errors.New("jabatan tidak sesuai kategori pegawai")
	}
	unitRaw := strings.TrimSpace(r.FormValue("unit_id"))
	if unitRaw != "" {
		uid, err := strconv.ParseInt(unitRaw, 10, 64)
		if err != nil || uid <= 0 {
			return preparedKGB{}, errors.New("unit kerja tidak valid")
		}
		u, err := store.GetUnitByID(r.Context(), s.Pool, uid)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return preparedKGB{}, errors.New("unit kerja tidak ditemukan")
			}
			return preparedKGB{}, err
		}
		switch u.Type {
		case "sd", "tk", "smp", "skb", "dinas", "korwil":
		default:
			return preparedKGB{}, errors.New("unit kerja tidak dikenal")
		}
		// Unit tujuan bertipe Dinas hanya masuk akal bagi pegawai Dinas, yang
		// usulannya memang langsung berstatus menunggu_dinas. Bagi pegawai unit
		// lain, memilih unit Dinas akan membuat usulan berstatus menunggu_unit
		// padahal unit Dinas tidak punya verifikator unit — usulan tersangkut
		// tanpa jalur keluar. Tolak di server, jangan hanya andalkan UI.
		if u.Type == "dinas" && t.UnitType != "dinas" {
			return preparedKGB{}, errors.New("unit tujuan Dinas hanya untuk pegawai Dinas; usulan Anda diverifikasi unit kerja Anda sendiri")
		}
		t.UnitID = u.ID
		t.UnitName = u.Name
	}
	// Satu hitungan untuk seluruh sistem (letterdata.NextDueTMT): TMT KGB
	// ditagih dari grid dua tahunan TMT awal, dimulai setelah SK terakhir.
	// Usulan telat tidak menggeser TMT ke siklus berikutnya selama SK periode
	// itu belum terbit. last_sk_tmt juga tetap dicetak sebagai naskah SK lama.
	if draft.LastSKTMT == nil {
		if proposedLast != nil {
			draft.LastSKTMT = proposedLast
		} else if t.LastSKTMTBerlaku != nil {
			draft.LastSKTMT = t.LastSKTMTBerlaku
		} else if t.TMTKGBLast != nil {
			draft.LastSKTMT = t.TMTKGBLast
		}
	}
	prior := *tmtAwal
	if draft.LastSKTMT != nil && draft.LastSKTMT.After(*tmtAwal) {
		prior = *draft.LastSKTMT
	}
	tmt := letterdata.NextDueTMT(*tmtAwal, prior, asOf)
	if tmtAwal.After(tmt) {
		return preparedKGB{}, errors.New("TMT awal tidak boleh setelah TMT KGB berlaku")
	}
	// Baris masa kerja naskah lewat satu modul bersama (letterdata.HitungMKG)
	// agar submit/kirim-ulang, pratinjau gaji, dan pratinjau dasbor tidak bisa
	// menyimpang lagi. Ringkasnya: poin 6 = masa SK pemenang apa adanya,
	// poin 8 = KGB terakhir + 2 tahun 0 bulan, gaji pada grid genap KGB.
	kgbTahun := draft.LastSKMasaTahun
	if kgbTahun == nil {
		kgbTahun = t.LastSKMasaKerjaTahun
	}
	kgbBulan := draft.LastSKMasaBulan
	if kgbBulan == nil {
		kgbBulan = t.LastSKMasaKerjaBulan
	}
	// Bila masa KGB belum tersimpan di master, jangkar dihitung dari TMT awal
	// (keputusan 2026-09-03) supaya submit tidak selalu menolak seeding lama.
	var fallbackTahun *int
	if kgbTahun == nil {
		masa := letterdata.MasaKerjaFromTMT(*tmtAwal, tmt)
		fallbackTahun = &masa
	}
	line := letterdata.HitungMKG(letterdata.SumberMKG{
		KGBTahun:      kgbTahun,
		KGBBulan:      kgbBulan,
		FallbackTahun: fallbackTahun,
		PemenangTahun: draft.LastKPMasaTahun,
		PemenangBulan: draft.LastKPMasaBulan,
		PemenangAktif: kpMenang,
	})
	lama, bulanLama := line.LamaTahun, line.LamaBulan
	baru, bulanBaru := line.BaruTahun, line.BaruBulan
	draft.MKGLamaTahun = &lama
	draft.MKGLamaBulan = &bulanLama
	draft.MKGBaruTahun = &baru
	draft.MKGBaruBulan = &bulanBaru
	masaLama, masaBaru := line.GridLama, line.GridBaru
	// Golongan efektif: KP bila tanggal SK-nya lebih baru dari SK KGB
	// terakhir (jangka waktu KGB tetap 2 tahun dari KGB terakhir),
	// selain itu golongan KGB/guru.
	// Sebutan pangkat PNS mengikuti golongan efektif; PPPK tidak punya
	// sebutan pangkat PNS — naskahnya memakai golongan I-XVII (template PPPK
	// mencetak "pangkat_jabatan" langsung dari golongan).
	gol := store.EffectiveGolongan(kp, draft.LastSKTanggal, t.PangkatGol)
	if t.ASNType == "pns" {
		draft.Pangkat = pangkatForGolongan(gol)
	} else if draft.Pangkat == "" {
		draft.Pangkat = gol
	}
	current, next, err := store.SalaryCurrentNext(r.Context(), s.Pool, t.ASNType, gol, masaLama)
	if err != nil {
		return preparedKGB{}, err
	}
	if err := validateDraftFor(t, draft, tmt, current, next); err != nil {
		return preparedKGB{}, err
	}
	return preparedKGB{
		ProposedTMT: tmt,
		LastTMT:     draft.LastSKTMT,
		TMTAwal:     tmtAwal,
		MasaBaru:    masaBaru,
		Draft:       draft,
		Current:     current,
		Next:        next,
	}, nil
}

func firstNonEmptyForm(r *http.Request, names ...string) string {
	for _, name := range names {
		if v := strings.TrimSpace(r.FormValue(name)); v != "" {
			return v
		}
	}
	return ""
}

// parseKPLast membaca seksi KP Terakhir (PNS) / SK Pertama atau Perpanjangan
// Kontrak (PPPK). Seluruh kolom wajib diisi: golongan, TMT, masa kerja
// (tahun+bulan), nomor, tanggal, dan pejabat SK.
func parseKPLast(r *http.Request) (store.KPLast, error) {
	var kp store.KPLast
	kp.Golongan = strings.TrimSpace(r.FormValue("last_kp_golongan"))
	kp.Nomor = strings.TrimSpace(r.FormValue("last_kp_nomor"))
	kp.Pejabat = strings.TrimSpace(r.FormValue("last_kp_pejabat"))
	tmt, err := parseOptionalFormDate(r.FormValue("last_kp_tmt"))
	if err != nil {
		return kp, errors.New("TMT KP terakhir tidak valid")
	}
	kp.TMT = tmt
	tanggal, err := parseOptionalFormDate(r.FormValue("last_kp_tanggal"))
	if err != nil {
		return kp, errors.New("tanggal SK KP tidak valid")
	}
	kp.Tanggal = tanggal
	masaTahun, err := parseOptionalFormInt(r.FormValue("last_kp_masa_tahun"))
	if err != nil {
		return kp, errors.New("masa kerja KP tidak valid")
	}
	kp.MasaTahun = masaTahun
	masaBulan, err := parseOptionalFormInt(r.FormValue("last_kp_masa_bulan"))
	if err != nil {
		return kp, errors.New("bulan masa kerja KP tidak valid")
	}
	kp.MasaBulan = masaBulan
	if kp.Golongan == "" || kp.TMT == nil || kp.MasaTahun == nil || kp.MasaBulan == nil || kp.Nomor == "" || kp.Tanggal == nil || kp.Pejabat == "" {
		return kp, errors.New("seluruh kolom KP Terakhir wajib diisi")
	}
	return kp, nil
}

// parseKGBLast membaca seksi KGB Terakhir: golongan ruang saat KGB terakhir
// dapat diubah (bisa berbeda dari KP), masa kerja mengikuti SK yang diterbitkan.
func parseKGBLast(r *http.Request, t store.Teacher) (store.KGBLast, error) {
	var kgb store.KGBLast
	kgb.Golongan = strings.TrimSpace(r.FormValue("last_kgb_golongan"))
	kgb.Nomor = strings.TrimSpace(r.FormValue("last_sk_nomor"))
	kgb.Pejabat = strings.TrimSpace(r.FormValue("last_sk_pejabat"))
	tmt, err := parseOptionalFormDate(r.FormValue("last_sk_tmt"))
	if err != nil {
		return kgb, errors.New("TMT KGB terakhir tidak valid")
	}
	kgb.TMT = tmt
	if kgb.TMT == nil {
		kgb.TMT = t.LastSKTMTBerlaku
	}
	if kgb.TMT == nil {
		kgb.TMT = t.TMTKGBLast
	}
	tanggal, err := parseOptionalFormDate(r.FormValue("last_sk_tanggal"))
	if err != nil {
		return kgb, errors.New("tanggal SK KGB tidak valid")
	}
	kgb.Tanggal = tanggal
	masaTahun, err := parseOptionalFormInt(r.FormValue("last_kgb_masa_tahun"))
	if err != nil {
		return kgb, errors.New("masa kerja KGB tidak valid")
	}
	kgb.MasaTahun = masaTahun
	masaBulan, err := parseOptionalFormInt(r.FormValue("last_kgb_masa_bulan"))
	if err != nil {
		return kgb, errors.New("bulan masa kerja KGB tidak valid")
	}
	kgb.MasaBulan = masaBulan
	if kgb.Golongan == "" {
		kgb.Golongan = t.PangkatGol
	}
	if kgb.Golongan == "" || kgb.TMT == nil {
		return kgb, errors.New("golongan dan TMT KGB terakhir wajib diisi")
	}
	return kgb, nil
}

func parseOptionalFormInt(raw string) (*int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil, errors.New("masa kerja harus berupa angka")
	}
	return &value, nil
}

func parseOptionalFormDate(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, errors.New("TMT KGB terakhir tidak valid")
	}
	return &value, nil
}

func parseOptionalEffectiveDate(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, errors.New("tanggal mulai berlaku SK tidak valid")
	}
	return &value, nil
}

func parseOptionalFormInt64(raw string) (*int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return nil, errors.New("unit tujuan tidak valid")
	}
	return &value, nil
}

func parseOptionalDashDate(raw string) (*time.Time, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "-" {
		return nil, raw == "-", nil
	}
	value, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, false, errors.New("tanggal perpanjangan perjanjian tidak valid")
	}
	return &value, false, nil
}

func letterDraftFromForm(r *http.Request, teacher store.Teacher, masaKerja int) (store.LetterDraft, error) {
	birthDate, err := parseOptionalFormDate(r.FormValue("birth_date"))
	if err != nil {
		return store.LetterDraft{}, errors.New("tanggal lahir tidak valid")
	}
	if birthDate == nil {
		birthDate = teacher.BirthDate
	}
	perpanjangan, _, err := parseOptionalDashDate(r.FormValue("perpanjangan_perjanjian_kerja"))
	if err != nil {
		return store.LetterDraft{}, err
	}
	orDefault := func(raw, fallback string) string {
		raw = strings.TrimSpace(raw)
		if raw != "" {
			return raw
		}
		return fallback
	}
	// Naskah SK mengikuti KP terakhir (pangkat/golongan + jabatan diisi dari
	// seksi KP/KGB di prepareKGBFromForm, bukan dari input ganda di sini).
	// Blok bawah form (SK terakhir + MKG) dihapus: sudah tercakup seksi atas.
	return store.LetterDraft{
		BirthPlace:     orDefault(r.FormValue("birth_place"), teacher.BirthPlace),
		BirthDate:      birthDate,
		Karpeg:         orDefault(r.FormValue("karpeg"), teacher.Karpeg),
		Pangkat:        orDefault(r.FormValue("pangkat"), teacher.Pangkat),
		Jabatan:        orDefault(r.FormValue("jabatan"), teacher.Jabatan),
		MasaPerjanjian: strings.TrimSpace(r.FormValue("masa_perjanjian_kerja")),
		Perpanjangan:   perpanjangan,
	}, nil
}

func validateDraftFor(teacher store.Teacher, draft store.LetterDraft, proposedTMT time.Time, current, next string) error {
	input := letterdata.Draft{
		ASNType:         teacher.ASNType,
		BirthPlace:      draft.BirthPlace,
		Karpeg:          draft.Karpeg,
		Pangkat:         draft.Pangkat,
		PangkatGol:      teacher.PangkatGol,
		Jabatan:         draft.Jabatan,
		BirthDate:       draft.BirthDate,
		LastSKPejabat:   draft.LastSKPejabat,
		LastSKNomor:     draft.LastSKNomor,
		LastSKTanggal:   draft.LastSKTanggal,
		LastSKTMT:       draft.LastSKTMT,
		LastSKMasaTahun: draft.LastSKMasaTahun,
		LastSKMasaBulan: draft.LastSKMasaBulan,
		LastKPGolongan:  draft.LastKPGolongan,
		LastKPTMT:       draft.LastKPTMT,
		LastKPMasaTahun: draft.LastKPMasaTahun,
		LastKPMasaBulan: draft.LastKPMasaBulan,
		LastKPNomor:     draft.LastKPNomor,
		LastKPTanggal:   draft.LastKPTanggal,
		LastKPPejabat:   draft.LastKPPejabat,
		MKGLamaTahun:    derefIntPtr(draft.MKGLamaTahun),
		MKGLamaBulan:    derefIntPtr(draft.MKGLamaBulan),
		MKGBaruTahun:    derefIntPtr(draft.MKGBaruTahun),
		MKGBaruBulan:    derefIntPtr(draft.MKGBaruBulan),
		ProposedTMT:     proposedTMT,
		CurrentSalary:   current,
		NextSalary:      next,
		UnitName:        teacher.UnitName,
		MasaPerjanjian:  draft.MasaPerjanjian,
		Perpanjangan:    draft.Perpanjangan,
	}
	if teacher.ASNType == "pppk" && letterdata.Normalize(draft.MasaPerjanjian) != "" && draft.Perpanjangan == nil {
		input.PerpanjanganDash = true
	}
	return letterdata.ValidateDraft(input)
}

func derefIntPtr(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func (s *Server) canAccessSubmission(r *http.Request, sub store.Submission) bool {
	u := userFrom(r)
	switch u.Role {
	case "admin", "admin_dinas", "pimpinan", "verifikator_dinas":
		return true
	case "verifikator_unit":
		if u.UnitID == nil {
			return false
		}
		inScope, err := store.UnitInScope(r.Context(), s.Pool, *u.UnitID, sub.VerificationUnitID())
		return err == nil && inScope
	case "asn":
		t, ok := s.teacherForUser(r)
		return ok && t.ID == sub.TeacherID
	default:
		return false
	}
}

func (s *Server) handleListSubmissions(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u.Role != "asn" {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Daftar ini hanya untuk ASN.")
		return
	}
	t, ok := s.teacherForUser(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "TEACHER_NOT_FOUND", "Data kepegawaian tidak ditemukan.")
		return
	}
	items, err := store.ListSubmissionsForTeacher(r.Context(), s.Pool, t.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil pengajuan.")
		return
	}
	writeData(w, http.StatusOK, items)
}

// handleSalaryPreview menghitung gaji lama/baru dan masa kerja untuk pratinjau
// form usul, memakai TMT awal (acuan) sampai TMT KGB berlaku. Read-only.
func (s *Server) handleSalaryPreview(w http.ResponseWriter, r *http.Request) {
	t, ok := s.teacherForUser(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "TEACHER_NOT_FOUND", "Data kepegawaian tidak ditemukan.")
		return
	}
	q := r.URL.Query()
	tmtAwal, err := parseOptionalFormDate(q.Get("tmt_awal"))
	if err != nil || tmtAwal == nil {
		writeErr(w, http.StatusUnprocessableEntity, "TMT_AWAL_REQUIRED", "TMT CPNS/pengangkatan wajib diisi.")
		return
	}
	// Rumus sama dengan submit (letterdata.NextDueTMT): anniversary setelah
	// SK terakhir di grid TMT awal; usulan telat tidak menggeser siklus.
	prior := *tmtAwal
	if v, errP := parseOptionalFormDate(q.Get("last_sk_tmt")); errP == nil && v != nil && v.After(*tmtAwal) {
		prior = *v
	}
	tmtBerlaku := letterdata.NextDueTMT(*tmtAwal, prior, time.Now())
	if tmtAwal.After(tmtBerlaku) {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_TMT", "TMT awal tidak boleh setelah TMT berlaku.")
		return
	}
	gol := strings.TrimSpace(q.Get("golongan"))
	if t.ASNType == "pppk" {
		if gol == "" {
			gol = t.PangkatGol
		}
		if !store.ValidPPPKGolongan(gol) {
			writeErr(w, http.StatusUnprocessableEntity, "INVALID_GOLONGAN", "Golongan PPPK harus I sampai XVII.")
			return
		}
	} else if gol == "" || !validPNSGolongan(gol) {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_GOLONGAN", "Golongan tidak valid.")
		return
	}
	// KP terakhir (opsional): bila tanggal SK-nya lebih baru dari SK KGB
	// terakhir, gaji mengacu golongan KP; jangka waktu tetap 2 tahun
	// dari KGB terakhir.
	kpGol := strings.TrimSpace(q.Get("last_kp_golongan"))
	kpTMT, err := parseOptionalFormDate(q.Get("last_kp_tmt"))
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_KP", "TMT KP terakhir tidak valid.")
		return
	}
	kgbTanggal, err := parseOptionalFormDate(q.Get("last_sk_tanggal"))
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_KGB", "Tanggal SK KGB tidak valid.")
		return
	}
	kpTanggal, err := parseOptionalFormDate(q.Get("last_kp_tanggal"))
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_KP", "Tanggal SK KP tidak valid.")
		return
	}
	if kpGol != "" || kpTMT != nil {
		if kpGol == "" || kpTMT == nil {
			writeErr(w, http.StatusUnprocessableEntity, "INVALID_KP", "Golongan dan TMT KP wajib diisi bila ada KP terbaru.")
			return
		}
		if t.ASNType == "pppk" && !store.ValidPPPKGolongan(kpGol) {
			writeErr(w, http.StatusUnprocessableEntity, "INVALID_KP", "Golongan KP PPPK harus I sampai XVII.")
			return
		}
		if t.ASNType == "pns" && !validPNSGolongan(kpGol) {
			writeErr(w, http.StatusUnprocessableEntity, "INVALID_KP", "Golongan KP tidak valid.")
			return
		}
		kpTanggal, err2 := parseOptionalFormDate(q.Get("last_kp_tanggal"))
		if err2 != nil {
			writeErr(w, http.StatusUnprocessableEntity, "INVALID_KP", "Tanggal SK KP tidak valid.")
			return
		}
		gol = store.EffectiveGolongan(store.KPLast{Golongan: kpGol, TMT: kpTMT, Tanggal: kpTanggal}, kgbTanggal, gol)
	}
	// Baris masa kerja lewat modul bersama (letterdata.HitungMKG) — aturan
	// identik dengan submit: poin 6 = masa SK pemenang apa adanya, poin 8 =
	// KGB + 2 th 0 bl, gaji pada grid genap KGB. Bulan kedua SK ikut dibaca
	// supaya angka pratinjau sama persis dengan yang akan dicetak.
	kgbTh := optionalQueryInt(q.Get("last_kgb_masa_tahun"))
	if kgbTh == nil {
		kgbTh = t.LastSKMasaKerjaTahun
	}
	kgbBl := optionalQueryInt(q.Get("last_kgb_masa_bulan"))
	if kgbBl == nil {
		kgbBl = t.LastSKMasaKerjaBulan
	}
	kpMasaTh := optionalQueryInt(q.Get("last_kp_masa_tahun"))
	kpMasaBl := optionalQueryInt(q.Get("last_kp_masa_bulan"))
	// Fallback masa kerja dari TMT awal → TMT berlaku bila seksi KGB belum
	// diisi (keputusan 2026-09-03), supaya pratinjau awal tidak menampilkan nol.
	var fallbackTh *int
	if kgbTh == nil {
		masa := letterdata.MasaKerjaFromTMT(*tmtAwal, tmtBerlaku)
		fallbackTh = &masa
	}
	line := letterdata.HitungMKG(letterdata.SumberMKG{
		KGBTahun:      kgbTh,
		KGBBulan:      kgbBl,
		FallbackTahun: fallbackTh,
		PemenangTahun: kpMasaTh,
		PemenangBulan: kpMasaBl,
		PemenangAktif: store.SKNewer(kpTanggal, kgbTanggal) && kpMasaTh != nil,
	})
	masaLama := line.GridLama
	current, next, err := store.SalaryCurrentNext(r.Context(), s.Pool, t.ASNType, gol, masaLama)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "SALARY_SCALE_NOT_FOUND", "Kombinasi golongan/masa kerja tidak ada di skala gaji.")
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"current_salary":   current,
		"next_salary":      next,
		"mkg_lama_tahun":   line.LamaTahun,
		"mkg_lama_bulan":   line.LamaBulan,
		"mkg_baru_tahun":   line.BaruTahun,
		"mkg_baru_bulan":   line.BaruBulan,
		"tmt_berlaku":      tmtBerlaku.Format("2006-01-02"),
		"pangkat":          pangkatForGolongan(gol),
		"golongan_efektif": gol,
	})
}

// optionalQueryInt membaca parameter angka opsional dari query; nilai kosong
// atau bukan angka non-negatif menghasilkan nil.
func optionalQueryInt(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return nil
	}
	return &n
}

func (s *Server) handleGetSubmission(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ID", "ID pengajuan tidak valid.")
		return
	}
	sub, err := store.GetSubmission(r.Context(), s.Pool, id)
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil pengajuan.")
		return
	}
	if !s.canAccessSubmission(r, sub) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Pengajuan bukan dalam kewenangan Anda.")
		return
	}
	audit, err := store.AuditForSubmission(r.Context(), s.Pool, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil riwayat pengajuan.")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"submission": sub, "history": audit})
}

func (s *Server) handleCreateSubmission(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartMemory+1<<20)
	t, ok := s.teacherForUser(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "TEACHER_NOT_FOUND", "Data kepegawaian tidak ditemukan.")
		return
	}
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_FORM", "Form pengajuan tidak valid.")
		return
	}
	prep, err := s.prepareKGBFromForm(r, t, time.Now())
	if err != nil {
		if errors.Is(err, letterdata.ErrDraftIncomplete) {
			writeErr(w, http.StatusUnprocessableEntity, "LETTER_DRAFT_INCOMPLETE", err.Error())
			return
		}
		if errors.Is(err, store.ErrScaleNotFound) {
			writeErr(w, http.StatusUnprocessableEntity, "SALARY_SCALE_NOT_FOUND", "Kombinasi golongan/masa kerja tidak ada di skala gaji.")
			return
		}
		if strings.Contains(err.Error(), "perubahan") || strings.Contains(err.Error(), "unit tujuan") || strings.Contains(err.Error(), "golongan") {
			writeErr(w, http.StatusUnprocessableEntity, "TEACHER_CHANGE_INVALID", err.Error())
			return
		}
		writeErr(w, http.StatusUnprocessableEntity, "KGB_DATA_REQUIRED", err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	hasMain := err == nil
	var mainName, mainPath string
	var mainSize int64
	if hasMain {
		defer file.Close()
		if header.Size > filesMaxUploadSize() {
			writeErr(w, http.StatusUnprocessableEntity, "FILE_TOO_LARGE", "Berkas melebihi 5MB.")
			return
		}
		if s.Files == nil {
			writeErr(w, http.StatusInternalServerError, "FILE_STORAGE_NOT_CONFIGURED", "Penyimpanan berkas belum dikonfigurasi.")
			return
		}
		var saveErr error
		mainPath, mainSize, saveErr = s.Files.SavePDF(r.Context(), file, header.Filename, header.Size)
		if saveErr != nil {
			if errors.Is(saveErr, files.ErrNotPDF) {
				writeErr(w, http.StatusUnprocessableEntity, "FILE_NOT_PDF", "Berkas harus PDF.")
				return
			}
			if errors.Is(saveErr, files.ErrTooLarge) {
				writeErr(w, http.StatusUnprocessableEntity, "FILE_TOO_LARGE", "Berkas melebihi 5MB.")
				return
			}
			writeErr(w, http.StatusInternalServerError, "FILE_SAVE_FAILED", "Berkas gagal disimpan.")
			return
		}
		mainName = files.OriginalName(header.Filename)
	} else if s.Files == nil {
		writeErr(w, http.StatusInternalServerError, "FILE_STORAGE_NOT_CONFIGURED", "Penyimpanan berkas belum dikonfigurasi.")
		return
	}
	// Slot berkas pendukung per jenis ASN (017): PNS = file_kp (SK KP) +
	// file_kgb (KGB terakhir), keduanya wajib; PPPK = file_kp (SK terakhir)
	// wajib + file_kgb (KGB terakhir) opsional + file_skp (SKP 2 tahun) wajib.
	// Kolom "file" utama tidak lagi dipakai form (dihapus 2026-09-08).
	slotFiles, ok := s.saveSlotFiles(w, r, t.ASNType)
	if !ok {
		if hasMain {
			_ = s.Files.Remove(mainPath)
		}
		return
	}
	draft := prep.Draft
	effectiveMasaKerja := prep.MasaBaru
	// Reuse existing proposed_* columns to persist golongan/unit yang dipilih di form
	// (semantics now: nilai efektif form, bukan 'change'); isi via TeacherChange minimal agar tetap kompatibel dengan store
	formGol := strings.TrimSpace(r.FormValue("pangkat_gol"))
	var cg store.TeacherChange
	if formGol != "" {
		cg.PangkatGol = &formGol
	}
	if uidRaw := strings.TrimSpace(r.FormValue("unit_id")); uidRaw != "" {
		if uid, err2 := strconv.ParseInt(uidRaw, 10, 64); err2 == nil && uid > 0 {
			cg.UnitID = &uid
		}
	}
	sub, err := store.CreateSubmission(r.Context(), s.Pool, t.ID, userFrom(r).ID, prep.ProposedTMT, &effectiveMasaKerja, prep.LastTMT, prep.TMTAwal, cg, draft, prep.Current, prep.Next, store.SubmissionFiles{
		Main: store.SubmissionFile{Name: mainName, Path: mainPath, Size: mainSize},
		KP:   slotFiles.KP, KGB: slotFiles.KGB, SKP: slotFiles.SKP,
	}, clientIP(r))
	if err != nil {
		if hasMain {
			_ = s.Files.Remove(mainPath)
		}
		s.removeSlotFiles(slotFiles)
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, "SUBMISSION_CREATE_FAILED", "Pengajuan gagal dibuat.")
		return
	}
	writeData(w, http.StatusCreated, sub)
}

func filesMaxUploadSize() int64 { return files.MaxUploadSize }

// saveSlotFiles menyimpan slot berkas pendukung sesuai jenis ASN.
// PNS: file_kp (SK KP terakhir) + file_kgb (KGB terakhir), wajib.
// PPPK: file_kp (SK terakhir) wajib + file_kgb (KGB terakhir) opsional +
// file_skp (SKP 2 tahun) wajib. Tiap berkas PDF maksimal 5MB.
// Mengembalikan false bila respons error sudah ditulis.
func (s *Server) saveSlotFiles(w http.ResponseWriter, r *http.Request, asnType string) (store.SubmissionFiles, bool) {
	var out store.SubmissionFiles
	save := func(field, label string, required bool) (store.SubmissionFile, bool) {
		f, header, err := r.FormFile(field)
		if err != nil {
			if !required {
				return store.SubmissionFile{}, true
			}
			writeErr(w, http.StatusUnprocessableEntity, "FILE_REQUIRED", "Berkas "+label+" wajib diunggah (PDF, maksimal 5MB).")
			return store.SubmissionFile{}, false
		}
		defer f.Close()
		if header.Size > filesMaxUploadSize() {
			writeErr(w, http.StatusUnprocessableEntity, "FILE_TOO_LARGE", "Berkas "+label+" melebihi 5MB.")
			return store.SubmissionFile{}, false
		}
		path, size, err := s.Files.SavePDF(r.Context(), f, header.Filename, header.Size)
		if err != nil {
			if errors.Is(err, files.ErrNotPDF) {
				writeErr(w, http.StatusUnprocessableEntity, "FILE_NOT_PDF", "Berkas "+label+" harus PDF.")
				return store.SubmissionFile{}, false
			}
			if errors.Is(err, files.ErrTooLarge) {
				writeErr(w, http.StatusUnprocessableEntity, "FILE_TOO_LARGE", "Berkas "+label+" melebihi 5MB.")
				return store.SubmissionFile{}, false
			}
			writeErr(w, http.StatusInternalServerError, "FILE_SAVE_FAILED", "Berkas "+label+" gagal disimpan.")
			return store.SubmissionFile{}, false
		}
		return store.SubmissionFile{Name: files.OriginalName(header.Filename), Path: path, Size: size}, true
	}
	var ok bool
	if asnType == "pppk" {
		if out.KP, ok = save("file_kp", "SK terakhir", true); !ok {
			s.removeSlotFiles(out)
			return out, false
		}
		if out.KGB, ok = save("file_kgb", "KGB terakhir", false); !ok {
			s.removeSlotFiles(out)
			return out, false
		}
		if out.SKP, ok = save("file_skp", "SKP 2 tahun", true); !ok {
			s.removeSlotFiles(out)
			return out, false
		}
		return out, true
	}
	if out.KP, ok = save("file_kp", "SK KP terakhir", true); !ok {
		s.removeSlotFiles(out)
		return out, false
	}
	if out.KGB, ok = save("file_kgb", "KGB terakhir", true); !ok {
		s.removeSlotFiles(out)
		return out, false
	}
	return out, true
}

// saveSlotFileOptional menyimpan satu slot berkas bila diunggah; bila tidak
// ada file baru, mengembalikan slot kosong (pemanggil memakai berkas lama).
func (s *Server) saveSlotFileOptional(w http.ResponseWriter, r *http.Request, field, label string) (store.SubmissionFile, bool) {
	f, header, err := r.FormFile(field)
	if err != nil {
		return store.SubmissionFile{}, true
	}
	defer f.Close()
	if header.Size > filesMaxUploadSize() {
		writeErr(w, http.StatusUnprocessableEntity, "FILE_TOO_LARGE", "Berkas "+label+" melebihi 5MB.")
		return store.SubmissionFile{}, false
	}
	path, size, err := s.Files.SavePDF(r.Context(), f, header.Filename, header.Size)
	if err != nil {
		if errors.Is(err, files.ErrNotPDF) {
			writeErr(w, http.StatusUnprocessableEntity, "FILE_NOT_PDF", "Berkas "+label+" harus PDF.")
			return store.SubmissionFile{}, false
		}
		if errors.Is(err, files.ErrTooLarge) {
			writeErr(w, http.StatusUnprocessableEntity, "FILE_TOO_LARGE", "Berkas "+label+" melebihi 5MB.")
			return store.SubmissionFile{}, false
		}
		writeErr(w, http.StatusInternalServerError, "FILE_SAVE_FAILED", "Berkas "+label+" gagal disimpan.")
		return store.SubmissionFile{}, false
	}
	return store.SubmissionFile{Name: files.OriginalName(header.Filename), Path: path, Size: size}, true
}

// saveSlotFilesResubmit menyimpan slot berkas kirim-ulang: slot yang tidak
// diunggah ulang memakai berkas lama (wajib tetap ada), slot baru wajib
// PDF maksimal 5MB. Gagal bila slot wajib kosong di lama maupun baru.
//
// Mengembalikan berkas gabungan, daftar berkas BARU yang diunggah (untuk
// dibersihkan bila pengajuan gagal disimpan), dan status. Berkas lama tidak
// pernah masuk daftar baru: kirim ulang yang gagal tidak boleh menghapus
// berkas yang masih dipakai pengajuan berjalan.
func (s *Server) saveSlotFilesResubmit(w http.ResponseWriter, r *http.Request, asnType string, id int64) (store.SubmissionFiles, []store.SubmissionFile, bool) {
	old, err := store.GetSubmission(r.Context(), s.Pool, id)
	if err != nil {
		if mapStoreError(w, err) {
			return store.SubmissionFiles{}, nil, false
		}
		writeErr(w, http.StatusInternalServerError, "RESUBMIT_FAILED", "Pengajuan ulang gagal.")
		return store.SubmissionFiles{}, nil, false
	}
	var fresh []store.SubmissionFile
	cleanup := func() {
		if s.Files == nil {
			return
		}
		for _, f := range fresh {
			if f.Path != "" {
				_ = s.Files.Remove(f.Path)
			}
		}
	}
	save := func(field, label string) (store.SubmissionFile, bool) {
		slot, ok := s.saveSlotFileOptional(w, r, field, label)
		if !ok {
			return store.SubmissionFile{}, false
		}
		if slot.Path != "" {
			fresh = append(fresh, slot)
		}
		return slot, true
	}
	needKGB := asnType != "pppk"
	needSKP := asnType == "pppk"
	var out store.SubmissionFiles
	var ok bool
	if out.KP, ok = save("file_kp", "SK terakhir"); !ok {
		cleanup()
		return out, nil, false
	}
	if out.KGB, ok = save("file_kgb", "KGB terakhir"); !ok {
		cleanup()
		return out, nil, false
	}
	if needSKP {
		if out.SKP, ok = save("file_skp", "SKP 2 tahun"); !ok {
			cleanup()
			return out, nil, false
		}
	}
	missing := ""
	switch {
	case out.KP.Path == "" && old.FilePath == "":
		missing = "SK KP terakhir"
	case needKGB && out.KGB.Path == "" && old.FileKGBPath == "":
		missing = "KGB terakhir"
	case needSKP && out.SKP.Path == "":
		missing = "SKP 2 tahun"
	}
	if missing != "" {
		cleanup()
		writeErr(w, http.StatusUnprocessableEntity, "FILE_REQUIRED", "Berkas "+missing+" wajib diunggah (PDF, maksimal 5MB). Semua SK yang dipersyaratkan harus lengkap.")
		return out, nil, false
	}
	keep := func(uploaded store.SubmissionFile, name string, size int, path string) store.SubmissionFile {
		if uploaded.Path != "" {
			return uploaded
		}
		return store.SubmissionFile{Name: name, Path: path, Size: int64(size)}
	}
	out.KP = keep(out.KP, old.FileKPName, old.FileKPSize, old.FileKPPath)
	out.KGB = keep(out.KGB, old.FileKGBName, old.FileKGBSize, old.FileKGBPath)
	out.SKP = keep(out.SKP, old.FileSKPName, old.FileSKPSize, old.FileSKPPath)
	return out, fresh, true
}

func (s *Server) removeSlotFiles(f store.SubmissionFiles) {
	if s.Files == nil {
		return
	}
	for _, slot := range []store.SubmissionFile{f.KP, f.KGB, f.SKP} {
		if slot.Path != "" {
			_ = s.Files.Remove(slot.Path)
		}
	}
}

func (s *Server) handleResubmit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartMemory+1<<20)
	id, err := parsePathID(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ID", "ID pengajuan tidak valid.")
		return
	}
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_FORM", "Form pengajuan tidak valid.")
		return
	}
	t, ok := s.teacherForUser(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "TEACHER_NOT_FOUND", "Data kepegawaian tidak ditemukan.")
		return
	}
	prep, err := s.prepareKGBFromForm(r, t, time.Now())
	if err != nil {
		if errors.Is(err, letterdata.ErrDraftIncomplete) {
			writeErr(w, http.StatusUnprocessableEntity, "LETTER_DRAFT_INCOMPLETE", err.Error())
			return
		}
		if errors.Is(err, store.ErrScaleNotFound) {
			writeErr(w, http.StatusUnprocessableEntity, "SALARY_SCALE_NOT_FOUND", "Skala gaji belum tersedia.")
			return
		}
		if strings.Contains(err.Error(), "perubahan") || strings.Contains(err.Error(), "unit tujuan") || strings.Contains(err.Error(), "golongan") {
			writeErr(w, http.StatusUnprocessableEntity, "TEACHER_CHANGE_INVALID", err.Error())
			return
		}
		writeErr(w, http.StatusUnprocessableEntity, "KGB_DATA_REQUIRED", err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	hasMain := err == nil
	var mainName, mainPath string
	var mainSize int64
	if hasMain {
		defer file.Close()
		if s.Files == nil {
			writeErr(w, http.StatusInternalServerError, "FILE_STORAGE_NOT_CONFIGURED", "Penyimpanan berkas belum dikonfigurasi.")
			return
		}
		var saveErr error
		mainPath, mainSize, saveErr = s.Files.SavePDF(r.Context(), file, header.Filename, header.Size)
		if saveErr != nil {
			writeErr(w, http.StatusUnprocessableEntity, "FILE_INVALID", "Berkas harus PDF dan maksimal 5MB.")
			return
		}
		mainName = filepath.Base(header.Filename)
	} else if s.Files == nil {
		writeErr(w, http.StatusInternalServerError, "FILE_STORAGE_NOT_CONFIGURED", "Penyimpanan berkas belum dikonfigurasi.")
		return
	}
	// Kirim-ulang: slot yang tidak diunggah ulang memakai berkas lama agar
	// pengusul cukup memperbaiki data/berkas yang salah saja. Slot yang belum
	// pernah ada tetap wajib diunggah (semua SK dipersyaratkan lengkap).
	slotFiles, freshSlots, ok := s.saveSlotFilesResubmit(w, r, t.ASNType, id)
	if !ok {
		if hasMain {
			_ = s.Files.Remove(mainPath)
		}
		return
	}
	draft := prep.Draft
	effectiveMasaKerja := prep.MasaBaru
	formGol2 := strings.TrimSpace(r.FormValue("pangkat_gol"))
	var cg2 store.TeacherChange
	if formGol2 != "" {
		cg2.PangkatGol = &formGol2
	}
	if uidRaw2 := strings.TrimSpace(r.FormValue("unit_id")); uidRaw2 != "" {
		if uid2, err2 := strconv.ParseInt(uidRaw2, 10, 64); err2 == nil && uid2 > 0 {
			cg2.UnitID = &uid2
		}
	}
	sub, err := store.Resubmit(r.Context(), s.Pool, id, userFrom(r).ID, prep.ProposedTMT, &effectiveMasaKerja, prep.LastTMT, prep.TMTAwal, cg2, draft, prep.Current, prep.Next, store.SubmissionFiles{
		Main: store.SubmissionFile{Name: mainName, Path: mainPath, Size: mainSize},
		KP:   slotFiles.KP, KGB: slotFiles.KGB, SKP: slotFiles.SKP,
	}, clientIP(r))
	if err != nil {
		if hasMain {
			_ = s.Files.Remove(mainPath)
		}
		// Hanya berkas yang BARU diunggah yang dibersihkan; berkas lama tetap
		// utuh karena pengajuan berjalan masih memakainya.
		s.removeSlotFileList(freshSlots)
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, "RESUBMIT_FAILED", "Pengajuan ulang gagal.")
		return
	}
	writeData(w, http.StatusOK, sub)
}

// removeSlotFileList menghapus berkas fisik pada daftar slot (hanya yang
// benar-benar baru diunggah pada satu permintaan).
func (s *Server) removeSlotFileList(files []store.SubmissionFile) {
	if s.Files == nil {
		return
	}
	for _, f := range files {
		if f.Path != "" {
			_ = s.Files.Remove(f.Path)
		}
	}
}

func (s *Server) handleSubmissionFile(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ID", "ID pengajuan tidak valid.")
		return
	}
	sub, err := store.GetSubmission(r.Context(), s.Pool, id)
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil berkas.")
		return
	}
	if !s.canAccessSubmission(r, sub) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Berkas bukan dalam kewenangan Anda.")
		return
	}
	// Slot unduhan: ?slot=kp|kgb|skp, default berkas utama (fallback slot KP
	// untuk data lama yang hanya punya satu berkas).
	slot := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("slot")))
	path, name := sub.Slot(slot)
	if slot != "" && slot != "kp" && slot != "kgb" && slot != "skp" {
		writeErr(w, http.StatusBadRequest, "INVALID_SLOT", "Slot berkas tidak dikenal (kp, kgb, skp).")
		return
	}
	if path == "" || s.Files == nil {
		writeErr(w, http.StatusNotFound, "FILE_NOT_FOUND", "Berkas tidak tersedia.")
		return
	}
	f, err := s.Files.Open(path)
	if err != nil {
		writeErr(w, http.StatusNotFound, "FILE_NOT_FOUND", "Berkas tidak tersedia.")
		return
	}
	defer f.Close()
	if err := store.InsertAudit(r.Context(), s.Pool, store.AuditEntry{ActorUserID: ptrInt64(userFrom(r).ID), SubmissionID: ptrInt64(id), Action: "unduh_berkas", IP: clientIP(r)}); err != nil {
		writeErr(w, http.StatusInternalServerError, "AUDIT_FAILED", "Unduhan gagal dicatat.")
		return
	}
	if name == "" {
		name = "berkas.pdf"
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+safeFilename(name, "berkas.pdf")+`"`)
	// Berkas ditampilkan dalam iframe same-origin di halaman pemeriksaan;
	// header frame global (DENY) harus dilonggarkan khusus respons ini.
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'self'; frame-src 'self'; img-src 'self' data:; base-uri 'self'")
	http.ServeContent(w, r, name, time.Time{}, f)
}

func (s *Server) handleUnitQueue(w http.ResponseWriter, r *http.Request) {
	s.handleQueue(w, r, "verifikator_unit", "menunggu_unit")
}

// handleUnitMonitor: pantauan unit — seluruh pengajuan dalam scope unit
// (semua status) agar unit tetap bisa mengawal usulannya setelah ACC:
// menunggu_dinas, menunggu_tte, dikembalikan_*, terbit. Read-only:
// detail memakai reviewDetail yang sama, tanpa tombol aksi (diputus di UI).
func (s *Server) handleUnitMonitor(w http.ResponseWriter, r *http.Request) {
	unitID := userFrom(r).UnitID
	if unitID == nil {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Akun verifikator belum memiliki scope unit.")
		return
	}
	status := r.URL.Query().Get("status")
	page := pageFrom(r)
	items, total, err := store.ListUnitMonitor(r.Context(), s.Pool, *unitID, status, page)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil pantauan unit.")
		return
	}
	writeDataMeta(w, http.StatusOK, items, map[string]any{"limit": page.Limit, "offset": page.Offset, "total": total})
}
func (s *Server) handleDinasQueue(w http.ResponseWriter, r *http.Request) {
	s.handleQueue(w, r, "verifikator_dinas", "menunggu_dinas")
}
func (s *Server) handlePendingTTE(w http.ResponseWriter, r *http.Request) {
	s.handleQueue(w, r, "pimpinan", "menunggu_tte")
}

func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request, role, status string) {
	page := pageFrom(r)
	items, total, err := store.ListQueue(r.Context(), s.Pool, role, userFrom(r).UnitID, status, page)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil antrean.")
		return
	}
	writeDataMeta(w, http.StatusOK, items, map[string]any{"limit": page.Limit, "offset": page.Offset, "total": total})
}

func (s *Server) reviewDetail(w http.ResponseWriter, r *http.Request, role string) (store.Submission, bool) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ID", "ID pengajuan tidak valid.")
		return store.Submission{}, false
	}
	sub, err := store.GetSubmission(r.Context(), s.Pool, id)
	if err != nil {
		if !mapStoreError(w, err) {
			writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil pengajuan.")
		}
		return store.Submission{}, false
	}
	if role == "verifikator_unit" {
		unitID := userFrom(r).UnitID
		if unitID == nil {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", "Akun verifikator belum memiliki scope unit.")
			return store.Submission{}, false
		}
		inScope, err := store.UnitInScope(r.Context(), s.Pool, *unitID, sub.VerificationUnitID())
		if err != nil || !inScope {
			writeErr(w, http.StatusForbidden, "FORBIDDEN", "Pengajuan bukan dalam scope unit Anda.")
			return store.Submission{}, false
		}
	}
	return sub, true
}

func (s *Server) handleUnitDetail(w http.ResponseWriter, r *http.Request) {
	sub, ok := s.reviewDetail(w, r, "verifikator_unit")
	if !ok {
		return
	}
	s.writeReviewDetail(w, r, sub)
}
func (s *Server) handleDinasDetail(w http.ResponseWriter, r *http.Request) {
	sub, ok := s.reviewDetail(w, r, "verifikator_dinas")
	if !ok {
		return
	}
	s.writeReviewDetail(w, r, sub)
}
func (s *Server) writeReviewDetail(w http.ResponseWriter, r *http.Request, sub store.Submission) {
	audit, err := store.AuditForSubmission(r.Context(), s.Pool, sub.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil riwayat.")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"submission": sub, "history": audit})
}

func (s *Server) handleUnitApprove(w http.ResponseWriter, r *http.Request) {
	s.handleTransition(w, r, "verifikator_unit", "menunggu_unit", "menunggu_dinas", "setuju_unit", false)
}
func (s *Server) handleUnitReject(w http.ResponseWriter, r *http.Request) {
	s.handleTransition(w, r, "verifikator_unit", "menunggu_unit", "dikembalikan_unit", "tolak_unit", true)
}
func (s *Server) handleDinasApprove(w http.ResponseWriter, r *http.Request) {
	s.handleDinasDecision(w, r, "menunggu_tte", "setuju_dinas", false)
}
func (s *Server) handleDinasReject(w http.ResponseWriter, r *http.Request) {
	s.handleDinasDecision(w, r, "dikembalikan_dinas", "tolak_dinas", true)
}

// koreksiNoteMin adalah panjang minimum catatan bila koreksi mengubah nilai
// yang menentukan gaji (TMT berlaku, gaji lama/baru). Keputusan owner
// 2026-09-11: kolom penentu gaji wajib beralasan, bukan sekadar dicentang.
const koreksiNoteMin = 10

// handleDinasKoreksi memperbaiki data usulan pada tahap menunggu_dinas sebelum
// diteruskan ke pimpinan untuk TTE (keputusan owner 2026-09-11).
//
// Form menerima field yang SAMA dengan form usulan sehingga memakai ulang
// prepareKGBFromForm: gaji, masa kerja naskah, dan TMT tetap dihitung oleh satu
// mesin yang sama. Berkas tidak diganti di sini — berkas keliru tetap lewat
// mekanisme tolak-kembali. Kolom yang mengubah gaji/TMT wajib disertai catatan.
func (s *Server) handleDinasKoreksi(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartMemory+1<<20)
	id, err := parsePathID(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ID", "ID pengajuan tidak valid.")
		return
	}
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_FORM", "Form koreksi tidak valid.")
		return
	}
	sub, err := store.GetSubmission(r.Context(), s.Pool, id)
	if err != nil {
		if !mapStoreError(w, err) {
			writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil pengajuan.")
		}
		return
	}
	if sub.Status != "menunggu_dinas" {
		writeErr(w, http.StatusConflict, "INVALID_STATUS_TRANSITION", "Koreksi hanya tersedia sebelum usulan diteruskan ke pimpinan (status menunggu Dinas).")
		return
	}
	t, err := store.GetTeacherByID(r.Context(), s.Pool, sub.TeacherID)
	if err != nil {
		if !mapStoreError(w, err) {
			writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil data kepegawaian.")
		}
		return
	}
	prep, err := s.prepareKGBFromForm(r, t, time.Now())
	if err != nil {
		if errors.Is(err, letterdata.ErrDraftIncomplete) {
			writeErr(w, http.StatusUnprocessableEntity, "LETTER_DRAFT_INCOMPLETE", err.Error())
			return
		}
		if errors.Is(err, store.ErrScaleNotFound) {
			writeErr(w, http.StatusUnprocessableEntity, "SALARY_SCALE_NOT_FOUND", "Kombinasi golongan/masa kerja tidak ada di skala gaji.")
			return
		}
		writeErr(w, http.StatusUnprocessableEntity, "KGB_DATA_REQUIRED", err.Error())
		return
	}
	// Catatan wajib bila nilai penentu gaji berubah: gaji lama/baru atau TMT
	// berlaku. Perubahan identitas naskah saja tidak mewajibkan catatan.
	note := strings.TrimSpace(r.FormValue("koreksi_note"))
	gajiBerubah := prep.Current != sub.CurrentSalary ||
		prep.Next != sub.NextSalary ||
		!prep.ProposedTMT.Equal(sub.ProposedTMT)
	if gajiBerubah && len([]rune(note)) < koreksiNoteMin {
		writeErr(w, http.StatusBadRequest, "NOTE_REQUIRED",
			"Koreksi yang mengubah gaji atau TMT wajib disertai catatan alasan (minimal 10 karakter).")
		return
	}
	// Berkas dipertahankan apa adanya: koreksi ini khusus data naskah.
	files := store.SubmissionFiles{
		Main: store.SubmissionFile{Name: sub.FileName, Path: sub.FilePath, Size: int64(sub.FileSize)},
		KP:   store.SubmissionFile{Name: sub.FileKPName, Path: sub.FileKPPath, Size: int64(sub.FileKPSize)},
		KGB:  store.SubmissionFile{Name: sub.FileKGBName, Path: sub.FileKGBPath, Size: int64(sub.FileKGBSize)},
		SKP:  store.SubmissionFile{Name: sub.FileSKPName, Path: sub.FileSKPPath, Size: int64(sub.FileSKPSize)},
	}
	formGol := strings.TrimSpace(r.FormValue("pangkat_gol"))
	var cg store.TeacherChange
	if formGol != "" {
		cg.PangkatGol = &formGol
	}
	if uidRaw := strings.TrimSpace(r.FormValue("unit_id")); uidRaw != "" {
		if uid, err2 := strconv.ParseInt(uidRaw, 10, 64); err2 == nil && uid > 0 {
			cg.UnitID = &uid
		}
	}
	updated, err := store.KoreksiDinas(r.Context(), s.Pool, id, userFrom(r).ID, prep.ProposedTMT, &prep.MasaBaru,
		prep.LastTMT, prep.TMTAwal, cg, prep.Draft, prep.Current, prep.Next, files, note, clientIP(r))
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, "KOREKSI_FAILED", "Koreksi data gagal disimpan.")
		return
	}
	writeData(w, http.StatusOK, updated)
}

// handleTTEReject: pimpinan menolak di tahap TTE — usulan kembali ke
// antrean Dinas (menunggu_dinas) untuk diperbaiki/diteruskan ulang
// (keputusan owner 2026-09-09). Catatan penolakan wajib.
func (s *Server) handleTTEReject(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "submission_id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ID", "ID pengajuan tidak valid.")
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if r.Body != nil {
		if err := decodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
			writeErr(w, http.StatusBadRequest, "INVALID_JSON", "Payload keputusan tidak valid.")
			return
		}
	}
	req.Note = strings.TrimSpace(req.Note)
	if req.Note == "" {
		writeErr(w, http.StatusBadRequest, "NOTE_REQUIRED", "Catatan penolakan wajib diisi.")
		return
	}
	updated, err := store.Transition(r.Context(), s.Pool, id, userFrom(r).ID, "menunggu_tte", "menunggu_dinas", "tolak_tte", req.Note, clientIP(r))
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, "TRANSITION_FAILED", "Perubahan status gagal.")
		return
	}
	writeData(w, http.StatusOK, updated)
}

// handleDinasDecision memproses keputusan dinas. Keputusan owner 2026-09-09:
// admin dinas (admin.disdik1-6) memverifikasi SEMUA usulan yang masuk
// dari unit (TK/SD/SMP/SKB) maupun usulan langsung pegawai Dinas;
// usulan pegawai Dinas tetap langsung menunggu_dinas tanpa tahap unit.
func (s *Server) handleDinasDecision(w http.ResponseWriter, r *http.Request, next, action string, noteRequired bool) {
	sub, ok := s.reviewDetail(w, r, "verifikator_dinas")
	if !ok {
		return
	}
	me := userFrom(r)
	var req struct {
		Note string `json:"note"`
	}
	if r.Body != nil {
		if err := decodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
			writeErr(w, http.StatusBadRequest, "INVALID_JSON", "Payload keputusan tidak valid.")
			return
		}
	}
	req.Note = strings.TrimSpace(req.Note)
	if noteRequired && req.Note == "" {
		writeErr(w, http.StatusBadRequest, "NOTE_REQUIRED", "Catatan penolakan wajib diisi.")
		return
	}
	updated, err := store.Transition(r.Context(), s.Pool, sub.ID, me.ID, "menunggu_dinas", next, action, req.Note, clientIP(r))
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, "TRANSITION_FAILED", "Perubahan status gagal.")
		return
	}
	writeData(w, http.StatusOK, updated)
}

func (s *Server) handleTransition(w http.ResponseWriter, r *http.Request, role, expected, next, action string, noteRequired bool) {
	sub, ok := s.reviewDetail(w, r, role)
	if !ok {
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if r.Body != nil {
		if err := decodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
			writeErr(w, http.StatusBadRequest, "INVALID_JSON", "Payload keputusan tidak valid.")
			return
		}
	}
	req.Note = strings.TrimSpace(req.Note)
	if noteRequired && req.Note == "" {
		writeErr(w, http.StatusBadRequest, "NOTE_REQUIRED", "Catatan penolakan wajib diisi.")
		return
	}
	updated, err := store.Transition(r.Context(), s.Pool, sub.ID, userFrom(r).ID, expected, next, action, req.Note, clientIP(r))
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, "TRANSITION_FAILED", "Perubahan status gagal.")
		return
	}
	writeData(w, http.StatusOK, updated)
}

func (s *Server) handleSignLetter(w http.ResponseWriter, r *http.Request) {
	submissionID, err := parsePathID(r, "submission_id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ID", "ID pengajuan tidak valid.")
		return
	}
	var req struct {
		Passphrase string `json:"passphrase"`
	}
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Passphrase) == "" {
		writeErr(w, http.StatusBadRequest, "PASSPHRASE_REQUIRED", "Passphrase TTE wajib diisi.")
		return
	}
	if s.Signer == nil {
		writeErr(w, http.StatusServiceUnavailable, "TTE_NOT_CONFIGURED", "Integrasi eSign belum dikonfigurasi.")
		return
	}
	if s.Renderer == nil {
		writeErr(w, http.StatusServiceUnavailable, "PDF_NOT_CONFIGURED", "Renderer PDF belum dikonfigurasi.")
		return
	}
	signer := userFrom(r)
	if signer.NIK == nil || *signer.NIK == "" || signer.SignatureImageBase64 == nil || *signer.SignatureImageBase64 == "" {
		writeErr(w, http.StatusServiceUnavailable, "SIGNER_NOT_CONFIGURED", "NIK dan spesimen TTD pimpinan belum dikonfigurasi.")
		return
	}
	if signer.EmployeeNumber == nil || strings.TrimSpace(*signer.EmployeeNumber) == "" {
		writeErr(w, http.StatusServiceUnavailable, "SIGNER_NOT_CONFIGURED", "NIP pimpinan untuk blok tanda tangan belum dikonfigurasi.")
		return
	}
	issue, err := store.BeginIssue(r.Context(), s.Pool, submissionID)
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, "TTE_PREPARE_FAILED", "Konsep surat gagal disiapkan.")
		return
	}
	defer func() {
		_ = store.ReleaseIssue(context.Background(), s.Pool, submissionID, issue.LockToken)
	}()
	draft := issue.Submission.LetterDraftValues()
	signerJob := ""
	if signer.JobTitle != nil {
		signerJob = *signer.JobTitle
	}
	perpanjangan := "-"
	if draft.Perpanjangan != nil {
		perpanjangan = pdf.FormatTanggalID(*draft.Perpanjangan)
	}
	ld := pdf.LetterData{
		Number:              issue.Number,
		TanggalNaskah:       pdf.TodayID(),
		IssuedAt:            pdf.TodayID(),
		ASNType:             issue.Submission.ASNType,
		TeacherName:         issue.Submission.TeacherName,
		NIP:                 issue.Submission.NIP,
		Karpeg:              draft.Karpeg,
		BirthPlace:          draft.BirthPlace,
		BirthDate:           formatTanggalID(draft.BirthDate),
		Pangkat:             draft.Pangkat,
		PangkatGol:          issue.Submission.PangkatGol,
		Jabatan:             draft.Jabatan,
		UnitName:            issue.Submission.UnitName,
		CurrentSalary:       issue.Submission.CurrentSalary,
		NextSalary:          issue.Submission.NextSalary,
		MasaKerjaLamaTahun:  derefIntPtr(draft.MKGLamaTahun),
		MasaKerjaLamaBulan:  derefIntPtr(draft.MKGLamaBulan),
		MasaKerjaBaruTahun:  derefIntPtr(draft.MKGBaruTahun),
		MasaKerjaBaruBulan:  derefIntPtr(draft.MKGBaruBulan),
		Golongan:            issue.Submission.PangkatGol,
		ProposedTMT:         pdf.FormatTanggalID(issue.Submission.ProposedTMT),
		NextKGBDate:         pdf.FormatTanggalID(issue.Submission.ProposedTMT.AddDate(2, 0, 0)),
		LastSKPejabat:       draft.LastSKPejabat,
		LastSKTanggal:       formatTanggalID(draft.LastSKTanggal),
		LastSKNomor:         draft.LastSKNomor,
		LastSKTMTBerlaku:    formatTanggalID(draft.LastSKTMT),
		MasaPerjanjian:      draft.MasaPerjanjian,
		PerpanjanganKontrak: perpanjangan,
		SignerName:          signer.Name,
		SignerNIP:           strings.TrimSpace(*signer.EmployeeNumber),
		SignerJob:           signerJob,
	}
	letterPDF, err := pdf.RenderLetter(r.Context(), s.Renderer, ld)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "PDF_RENDER_FAILED", "Konsep surat gagal dibuat.")
		return
	}
	sign, err := s.Signer.Sign(r.Context(), esign.SignRequest{NIK: *signer.NIK, Passphrase: req.Passphrase, ImageBase64: *signer.SignatureImageBase64, Page: 1, OriginX: 105, OriginY: 190, Width: 70, Height: 45, PDF: letterPDF})
	if err != nil {
		writeErr(w, http.StatusBadGateway, "TTE_FAILED", "Penandatanganan gagal, surat belum diterbitkan.")
		return
	}
	finalPDF, err := s.Signer.Download(r.Context(), sign.ReceiptID)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "TTE_DOWNLOAD_FAILED", "PDF hasil TTE gagal diunduh.")
		return
	}
	if s.Files == nil {
		writeErr(w, http.StatusInternalServerError, "FILE_STORAGE_NOT_CONFIGURED", "Penyimpanan surat belum dikonfigurasi.")
		return
	}
	finalPath, _, err := s.Files.SavePDF(r.Context(), bytes.NewReader(finalPDF), fmt.Sprintf("surat-kgb-%d.pdf", submissionID), int64(len(finalPDF)))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "PDF_SAVE_FAILED", "Surat final gagal disimpan.")
		return
	}
	if err := store.CommitIssue(r.Context(), s.Pool, issue, signer.ID, sign.ReceiptID, finalPath, clientIP(r)); err != nil {
		_ = s.Files.Remove(finalPath)
		writeErr(w, http.StatusInternalServerError, "ISSUE_COMMIT_FAILED", "Penerbitan surat gagal diselesaikan.")
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"number": issue.Number, "issued_at": time.Now().Format(time.RFC3339), "receipt_id": sign.ReceiptID})
}

// draftLetterData menyiapkan data naskah SK untuk pratinjau (nomor sementara,
// read-only: tidak mencadangkan sequence atau lock TTE). Bila peminta bukan
// pimpinan, blok tanda tangan memakai profil pimpinan aktif.
func (s *Server) draftLetterData(w http.ResponseWriter, r *http.Request, submissionID int64) (pdf.LetterData, bool) {
	var ld pdf.LetterData
	sub, err := store.GetSubmission(r.Context(), s.Pool, submissionID)
	if err != nil {
		if !mapStoreError(w, err) {
			writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil pengajuan.")
		}
		return ld, false
	}
	if sub.Status != "menunggu_tte" {
		writeErr(w, http.StatusConflict, "NOT_PENDING_TTE", "Draft hanya tersedia saat pengajuan menunggu TTE.")
		return ld, false
	}
	if !s.canAccessSubmission(r, sub) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Draft bukan dalam kewenangan Anda.")
		return ld, false
	}
	if err := store.ValidateSubmissionDraft(sub); err != nil {
		if !mapStoreError(w, err) {
			writeErr(w, http.StatusUnprocessableEntity, "DRAFT_INCOMPLETE", "Naskah SK belum lengkap untuk membuat draft.")
		}
		return ld, false
	}
	number, err := store.PreviewLetterNumber(r.Context(), s.Pool)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "TEMPLATE_UNAVAILABLE", "Template nomor surat aktif belum tersedia.")
		return ld, false
	}
	signer := userFrom(r)
	if signer.Role != "pimpinan" {
		// Petugas Dinas meninjau draft: blok tanda tangan memakai profil
		// pimpinan aktif agar naskah sama seperti yang akan diterbitkan.
		if p, err := store.GetActivePimpinan(r.Context(), s.Pool); err == nil {
			signer = p
		}
	}
	signerJob := ""
	if signer.JobTitle != nil {
		signerJob = *signer.JobTitle
	}
	signerNIP := ""
	if signer.EmployeeNumber != nil {
		signerNIP = strings.TrimSpace(*signer.EmployeeNumber)
	}
	draft := sub.LetterDraftValues()
	perpanjangan := "-"
	if draft.Perpanjangan != nil {
		perpanjangan = pdf.FormatTanggalID(*draft.Perpanjangan)
	}
	ld = pdf.LetterData{
		Number:              number,
		TanggalNaskah:       pdf.TodayID(),
		IssuedAt:            pdf.TodayID(),
		ASNType:             sub.ASNType,
		TeacherName:         sub.TeacherName,
		NIP:                 sub.NIP,
		Karpeg:              draft.Karpeg,
		BirthPlace:          draft.BirthPlace,
		BirthDate:           formatTanggalID(draft.BirthDate),
		Pangkat:             draft.Pangkat,
		PangkatGol:          sub.PangkatGol,
		Jabatan:             draft.Jabatan,
		UnitName:            sub.UnitName,
		CurrentSalary:       sub.CurrentSalary,
		NextSalary:          sub.NextSalary,
		MasaKerjaLamaTahun:  derefIntPtr(draft.MKGLamaTahun),
		MasaKerjaLamaBulan:  derefIntPtr(draft.MKGLamaBulan),
		MasaKerjaBaruTahun:  derefIntPtr(draft.MKGBaruTahun),
		MasaKerjaBaruBulan:  derefIntPtr(draft.MKGBaruBulan),
		Golongan:            sub.PangkatGol,
		ProposedTMT:         pdf.FormatTanggalID(sub.ProposedTMT),
		NextKGBDate:         pdf.FormatTanggalID(sub.ProposedTMT.AddDate(2, 0, 0)),
		LastSKPejabat:       draft.LastSKPejabat,
		LastSKTanggal:       formatTanggalID(draft.LastSKTanggal),
		LastSKNomor:         draft.LastSKNomor,
		LastSKTMTBerlaku:    formatTanggalID(draft.LastSKTMT),
		MasaPerjanjian:      draft.MasaPerjanjian,
		PerpanjanganKontrak: perpanjangan,
		SignerName:          signer.Name,
		SignerNIP:           signerNIP,
		SignerJob:           signerJob,
	}
	return ld, true
}

// handleDraftDOCX mengunduh draft naskah SK dalam format DOCX (Word) untuk
// ditinjau sebelum diteruskan ke TTE. Pimpinan mengunduh saat menunggu TTE;
// verifikator/admin Dinas ikut dapat mengunduhnya sebagai bahan pengecekan.
// Bersifat read-only: memakai nomor pratinjau dan tidak mencadangkan sequence
// atau lock TTE. Bila peminta bukan pimpinan, blok tanda tangan memakai
// profil pimpinan aktif (NIK/NIP/spesimen tidak diperlukan untuk DOCX).
func (s *Server) handleDraftDOCX(w http.ResponseWriter, r *http.Request) {
	submissionID, err := parsePathID(r, "submission_id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ID", "ID pengajuan tidak valid.")
		return
	}
	if s.Renderer == nil {
		writeErr(w, http.StatusServiceUnavailable, "PDF_NOT_CONFIGURED", "Renderer naskah belum dikonfigurasi.")
		return
	}
	ld, ok := s.draftLetterData(w, r, submissionID)
	if !ok {
		return
	}
	docx, err := pdf.RenderLetterDOCX(s.Renderer, ld)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DOCX_RENDER_FAILED", "Draft naskah gagal dibuat.")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="draft-SK-KGB-%d.docx"`, submissionID))
	w.Header().Set("Content-Length", strconv.Itoa(len(docx)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(docx)
}

// handleDraftPDF menyajikan pratinjau PDF hasil render template Word asli
// (via LibreOffice di server) untuk layar TTE pimpinan. Read-only seperti
// draft DOCX: nomor sementara, tanpa lock TTE. Disajikan inline agar bisa
// dibuka di iframe pratinjau.
func (s *Server) handleDraftPDF(w http.ResponseWriter, r *http.Request) {
	submissionID, err := parsePathID(r, "submission_id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ID", "ID pengajuan tidak valid.")
		return
	}
	if s.Renderer == nil {
		writeErr(w, http.StatusServiceUnavailable, "PDF_NOT_CONFIGURED", "Renderer naskah belum dikonfigurasi.")
		return
	}
	ld, ok := s.draftLetterData(w, r, submissionID)
	if !ok {
		return
	}
	body, err := pdf.RenderLetter(r.Context(), s.Renderer, ld)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "PDF_RENDER_FAILED", "Pratinjau naskah gagal dibuat.")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="pratinjau-SK-KGB-%d.pdf"`, submissionID))
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) handleLetterDownload(w http.ResponseWriter, r *http.Request) {
	letterID, err := parsePathID(r, "id")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ID", "ID surat tidak valid.")
		return
	}
	letter, path, submissionID, err := store.GetLetter(r.Context(), s.Pool, letterID)
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil surat.")
		return
	}
	sub, err := store.GetSubmission(r.Context(), s.Pool, submissionID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal memeriksa akses surat.")
		return
	}
	if !s.canAccessSubmission(r, sub) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "Surat bukan dalam kewenangan Anda.")
		return
	}
	if s.Files == nil {
		writeErr(w, http.StatusNotFound, "FILE_NOT_FOUND", "PDF surat tidak tersedia.")
		return
	}
	f, err := s.Files.Open(path)
	if err != nil {
		writeErr(w, http.StatusNotFound, "FILE_NOT_FOUND", "PDF surat tidak tersedia.")
		return
	}
	defer f.Close()
	if err := store.InsertAudit(r.Context(), s.Pool, store.AuditEntry{ActorUserID: ptrInt64(userFrom(r).ID), SubmissionID: ptrInt64(submissionID), Action: "unduh_surat", IP: clientIP(r)}); err != nil {
		writeErr(w, http.StatusInternalServerError, "AUDIT_FAILED", "Unduhan gagal dicatat.")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeFilename("surat-kgb-"+strconv.FormatInt(letter.ID, 10)+".pdf", "surat-kgb.pdf")+`"`)
	http.ServeContent(w, r, "surat-kgb.pdf", letter.IssuedAt, f)
}

func (s *Server) handleListLetters(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	items, err := store.ListLetters(r.Context(), s.Pool, year)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil rekap surat.")
		return
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) handlePublicStats(w http.ResponseWriter, r *http.Request) {
	stats, err := store.GetPublicStats(r.Context(), s.Pool)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Statistik belum tersedia.")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeData(w, http.StatusOK, stats)
}

func ptrInt64(v int64) *int64 { return &v }
func safeFilename(name, fallback string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "\"", "")
	if name == "." || name == "" {
		return fallback
	}
	return name
}
