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
		writeErr(w, http.StatusConflict, "INVALID_STATUS_TRANSITION", "Status pengajuan sudah berubah atau tidak sesuai.")
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

// resolveKGBInputs memilih data master atau koreksi yang diisi saat pengajuan.
// Masa kerja harus tersedia untuk menghitung skala gaji; TMT KGB terakhir
// boleh kosong pada seeding awal dan hanya dipakai bila diisi.
func resolveKGBInputs(teacher store.Teacher, proposedMasaKerja *int, proposedTMTKGBLast *time.Time, proposedTMT time.Time) (int, *time.Time, error) {
	masaKerja := teacher.MasaKerjaTahun
	if proposedMasaKerja != nil {
		masaKerja = *proposedMasaKerja
	}
	if masaKerja < 0 {
		return 0, nil, errors.New("masa kerja tidak boleh negatif")
	}
	if proposedMasaKerja == nil && teacher.MasaKerjaSource == "belum_tersedia" {
		return 0, nil, errors.New("masa kerja wajib dilengkapi pada pengajuan")
	}
	last := teacher.TMTKGBLast
	if proposedTMTKGBLast != nil {
		last = proposedTMTKGBLast
	}
	if last != nil && last.After(proposedTMT) {
		return 0, nil, errors.New("TMT KGB terakhir tidak boleh setelah TMT usulan")
	}
	return masaKerja, last, nil
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
	// Golongan editable langsung; PPPK dikunci IX. Pangkat diturunkan dari golongan.
	formGol := strings.TrimSpace(r.FormValue("pangkat_gol"))
	if t.ASNType == "pppk" {
		t.PangkatGol = "IX"
	} else if formGol != "" {
		if !validPNSGolongan(formGol) {
			return preparedKGB{}, errors.New("pangkat/golongan tidak valid")
		}
		t.PangkatGol = formGol
		draft.Pangkat = pangkatForGolongan(formGol)
	}
	// Jabatan wajib dari daftar jenjang fungsional guru.
	if j := strings.TrimSpace(draft.Jabatan); j != "" && !validJabatanGuru(j) {
		return preparedKGB{}, errors.New("jabatan harus dipilih dari daftar jenjang fungsional guru")
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
		case "sd", "tk", "smp", "skb", "dinas":
		default:
			return preparedKGB{}, errors.New("unit kerja harus unit layanan, bukan Korwil")
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
	// Masa kerja golongan dihitung dari TMT awal → TMT berlaku; baru = masa ke TMT berlaku.
	masaBaru := letterdata.MasaKerjaFromTMT(*tmtAwal, tmt)
	masaLama := masaBaru - 2
	if masaLama < 0 {
		masaLama = 0
	}
	lama, baru, zero := masaLama, masaBaru, 0
	draft.MKGLamaTahun = &lama
	draft.MKGLamaBulan = &zero
	draft.MKGBaruTahun = &baru
	draft.MKGBaruBulan = &zero
	gol := t.PangkatGol
	if t.ASNType == "pppk" {
		gol = "IX"
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
	lastSKTanggal, err := parseOptionalFormDate(r.FormValue("last_sk_tanggal"))
	if err != nil {
		return store.LetterDraft{}, errors.New("tanggal SK terakhir tidak valid")
	}
	if lastSKTanggal == nil {
		lastSKTanggal = teacher.LastSKTanggal
	}
	lastSKTMT, err := parseOptionalFormDate(r.FormValue("last_sk_tmt"))
	if err != nil {
		return store.LetterDraft{}, errors.New("TMT SK terakhir tidak valid")
	}
	if lastSKTMT == nil {
		lastSKTMT = teacher.LastSKTMTBerlaku
	}
	mkgLamaTahun, err := parseOptionalFormInt(r.FormValue("mkg_lama_tahun"))
	if err != nil {
		return store.LetterDraft{}, errors.New("masa kerja lama tidak valid")
	}
	mkgLamaBulan, err := parseOptionalFormInt(r.FormValue("mkg_lama_bulan"))
	if err != nil {
		return store.LetterDraft{}, errors.New("bulan masa kerja lama tidak valid")
	}
	mkgBaruTahun, err := parseOptionalFormInt(r.FormValue("mkg_baru_tahun"))
	if err != nil {
		return store.LetterDraft{}, errors.New("masa kerja baru tidak valid")
	}
	mkgBaruBulan, err := parseOptionalFormInt(r.FormValue("mkg_baru_bulan"))
	if err != nil {
		return store.LetterDraft{}, errors.New("bulan masa kerja baru tidak valid")
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
	return store.LetterDraft{
		BirthPlace:     orDefault(r.FormValue("birth_place"), teacher.BirthPlace),
		BirthDate:      birthDate,
		Karpeg:         orDefault(r.FormValue("karpeg"), teacher.Karpeg),
		Pangkat:        orDefault(r.FormValue("pangkat"), teacher.Pangkat),
		Jabatan:        orDefault(r.FormValue("jabatan"), teacher.Jabatan),
		LastSKPejabat:  orDefault(r.FormValue("last_sk_pejabat"), teacher.LastSKPejabat),
		LastSKTanggal:  lastSKTanggal,
		LastSKNomor:    orDefault(r.FormValue("last_sk_nomor"), teacher.LastSKNomor),
		LastSKTMT:      lastSKTMT,
		MKGLamaTahun:   mkgLamaTahun,
		MKGLamaBulan:   mkgLamaBulan,
		MKGBaruTahun:   mkgBaruTahun,
		MKGBaruBulan:   mkgBaruBulan,
		MasaPerjanjian: strings.TrimSpace(r.FormValue("masa_perjanjian_kerja")),
		Perpanjangan:   perpanjangan,
	}, nil
}

func validateDraftFor(teacher store.Teacher, draft store.LetterDraft, proposedTMT time.Time, current, next string) error {
	input := letterdata.Draft{
		ASNType:        teacher.ASNType,
		BirthPlace:     draft.BirthPlace,
		Karpeg:         draft.Karpeg,
		Pangkat:        draft.Pangkat,
		PangkatGol:     teacher.PangkatGol,
		Jabatan:        draft.Jabatan,
		BirthDate:      draft.BirthDate,
		LastSKPejabat:  draft.LastSKPejabat,
		LastSKNomor:    draft.LastSKNomor,
		LastSKTanggal:  draft.LastSKTanggal,
		LastSKTMT:      draft.LastSKTMT,
		MKGLamaTahun:   derefIntPtr(draft.MKGLamaTahun),
		MKGLamaBulan:   derefIntPtr(draft.MKGLamaBulan),
		MKGBaruTahun:   derefIntPtr(draft.MKGBaruTahun),
		MKGBaruBulan:   derefIntPtr(draft.MKGBaruBulan),
		ProposedTMT:    proposedTMT,
		CurrentSalary:  current,
		NextSalary:     next,
		UnitName:       teacher.UnitName,
		MasaPerjanjian: draft.MasaPerjanjian,
		Perpanjangan:   draft.Perpanjangan,
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
		gol = "IX"
	} else if gol == "" || !validPNSGolongan(gol) {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_GOLONGAN", "Golongan tidak valid.")
		return
	}
	masaBaru := letterdata.MasaKerjaFromTMT(*tmtAwal, tmtBerlaku)
	masaLama := masaBaru - 2
	if masaLama < 0 {
		masaLama = 0
	}
	current, next, err := store.SalaryCurrentNext(r.Context(), s.Pool, t.ASNType, gol, masaLama)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "SALARY_SCALE_NOT_FOUND", "Kombinasi golongan/masa kerja tidak ada di skala gaji.")
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"current_salary":   current,
		"next_salary":      next,
		"mkg_lama_tahun":   masaLama,
		"mkg_baru_tahun":   masaBaru,
		"tmt_berlaku":      tmtBerlaku.Format("2006-01-02"),
		"pangkat":          pangkatForGolongan(gol),
	})
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
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "FILE_REQUIRED", "Satu berkas PDF wajib diunggah.")
		return
	}
	defer file.Close()
	if header.Size > filesMaxUploadSize() {
		writeErr(w, http.StatusUnprocessableEntity, "FILE_TOO_LARGE", "Berkas melebihi 5MB.")
		return
	}
	if s.Files == nil {
		writeErr(w, http.StatusInternalServerError, "FILE_STORAGE_NOT_CONFIGURED", "Penyimpanan berkas belum dikonfigurasi.")
		return
	}
	path, size, err := s.Files.SavePDF(r.Context(), file, header.Filename, header.Size)
	if err != nil {
		if errors.Is(err, files.ErrNotPDF) {
			writeErr(w, http.StatusUnprocessableEntity, "FILE_NOT_PDF", "Berkas harus PDF.")
			return
		}
		if errors.Is(err, files.ErrTooLarge) {
			writeErr(w, http.StatusUnprocessableEntity, "FILE_TOO_LARGE", "Berkas melebihi 5MB.")
			return
		}
		writeErr(w, http.StatusInternalServerError, "FILE_SAVE_FAILED", "Berkas gagal disimpan.")
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
	sub, err := store.CreateSubmission(r.Context(), s.Pool, t.ID, userFrom(r).ID, prep.ProposedTMT, &effectiveMasaKerja, prep.LastTMT, prep.TMTAwal, cg, draft, prep.Current, prep.Next, files.OriginalName(header.Filename), path, size, clientIP(r))
	if err != nil {
		_ = s.Files.Remove(path)
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, "SUBMISSION_CREATE_FAILED", "Pengajuan gagal dibuat.")
		return
	}
	writeData(w, http.StatusCreated, sub)
}

func filesMaxUploadSize() int64 { return files.MaxUploadSize }

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
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "FILE_REQUIRED", "Satu berkas PDF wajib diunggah.")
		return
	}
	defer file.Close()
	if s.Files == nil {
		writeErr(w, http.StatusInternalServerError, "FILE_STORAGE_NOT_CONFIGURED", "Penyimpanan berkas belum dikonfigurasi.")
		return
	}
	path, size, err := s.Files.SavePDF(r.Context(), file, header.Filename, header.Size)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "FILE_INVALID", "Berkas harus PDF dan maksimal 5MB.")
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
	sub, err := store.Resubmit(r.Context(), s.Pool, id, userFrom(r).ID, prep.ProposedTMT, &effectiveMasaKerja, prep.LastTMT, prep.TMTAwal, cg2, draft, prep.Current, prep.Next, filepath.Base(header.Filename), path, size, clientIP(r))
	if err != nil {
		_ = s.Files.Remove(path)
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, http.StatusInternalServerError, "RESUBMIT_FAILED", "Pengajuan ulang gagal.")
		return
	}
	writeData(w, http.StatusOK, sub)
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
	if sub.FilePath == "" || s.Files == nil {
		writeErr(w, http.StatusNotFound, "FILE_NOT_FOUND", "Berkas tidak tersedia.")
		return
	}
	f, err := s.Files.Open(sub.FilePath)
	if err != nil {
		writeErr(w, http.StatusNotFound, "FILE_NOT_FOUND", "Berkas tidak tersedia.")
		return
	}
	defer f.Close()
	if err := store.InsertAudit(r.Context(), s.Pool, store.AuditEntry{ActorUserID: ptrInt64(userFrom(r).ID), SubmissionID: ptrInt64(id), Action: "unduh_berkas", IP: clientIP(r)}); err != nil {
		writeErr(w, http.StatusInternalServerError, "AUDIT_FAILED", "Unduhan gagal dicatat.")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+safeFilename(sub.FileName, "berkas.pdf")+`"`)
	http.ServeContent(w, r, sub.FileName, time.Time{}, f)
}

func (s *Server) handleUnitQueue(w http.ResponseWriter, r *http.Request) {
	s.handleQueue(w, r, "verifikator_unit", "menunggu_unit")
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
	s.handleTransition(w, r, "verifikator_dinas", "menunggu_dinas", "menunggu_tte", "setuju_dinas", false)
}
func (s *Server) handleDinasReject(w http.ResponseWriter, r *http.Request) {
	s.handleTransition(w, r, "verifikator_dinas", "menunggu_dinas", "dikembalikan_dinas", "tolak_dinas", true)
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

// handleDraftDOCX mengunduh draft naskah SK dalam format DOCX untuk ditinjau
// atau ditandatangani manual. Bersifat read-only: memakai nomor pratinjau dan
// tidak mencadangkan sequence atau lock TTE.
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
	sub, err := store.GetSubmission(r.Context(), s.Pool, submissionID)
	if err != nil {
		if !mapStoreError(w, err) {
			writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil pengajuan.")
		}
		return
	}
	if sub.Status != "menunggu_tte" {
		writeErr(w, http.StatusConflict, "NOT_PENDING_TTE", "Draft hanya tersedia saat pengajuan menunggu TTE.")
		return
	}
	if err := store.ValidateSubmissionDraft(sub); err != nil {
		if !mapStoreError(w, err) {
			writeErr(w, http.StatusUnprocessableEntity, "DRAFT_INCOMPLETE", "Naskah SK belum lengkap untuk membuat draft.")
		}
		return
	}
	number, err := store.PreviewLetterNumber(r.Context(), s.Pool)
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "TEMPLATE_UNAVAILABLE", "Template nomor surat aktif belum tersedia.")
		return
	}
	signer := userFrom(r)
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
	ld := pdf.LetterData{
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
