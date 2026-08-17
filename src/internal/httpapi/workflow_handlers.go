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
	default:
		return false
	}
	return true
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

// ProposedTeacherChangeInput adalah field perubahan kepegawaian yang boleh
// diusulkan ASN berdasarkan SK. Identitas BKN tidak termasuk di sini.
type ProposedTeacherChangeInput struct {
	PangkatGol    string
	Pangkat       string
	Jabatan       string
	UnitID        *int64
	EffectiveDate *time.Time
	Note          string
}

// resolveTeacherChange memvalidasi perubahan kepegawaian dan membuat detail
// audit sebelum submission disimpan. Unit Korwil tidak boleh menjadi tujuan
// mutasi; tujuan harus unit layanan yang dapat menerima antrean verifikasi.
func resolveTeacherChange(teacher store.Teacher, input ProposedTeacherChangeInput, targetUnit *store.Unit, proposedTMT time.Time) (store.TeacherChange, error) {
	input.PangkatGol = strings.TrimSpace(input.PangkatGol)
	input.Pangkat = strings.TrimSpace(input.Pangkat)
	input.Jabatan = strings.TrimSpace(input.Jabatan)
	input.Note = strings.TrimSpace(input.Note)
	hasChange := input.PangkatGol != "" || input.Pangkat != "" || input.Jabatan != "" || input.UnitID != nil
	if !hasChange {
		if input.EffectiveDate != nil || input.Note != "" {
			return store.TeacherChange{}, errors.New("tanggal dan catatan hanya boleh diisi bersama perubahan data")
		}
		return store.TeacherChange{}, nil
	}
	if input.EffectiveDate == nil {
		return store.TeacherChange{}, errors.New("tanggal mulai berlaku pada SK wajib diisi")
	}
	if input.EffectiveDate.After(proposedTMT) {
		return store.TeacherChange{}, errors.New("tanggal mulai berlaku perubahan tidak boleh setelah TMT usulan KGB")
	}
	if input.Note == "" {
		return store.TeacherChange{}, errors.New("catatan perubahan wajib diisi")
	}
	if len(input.Note) > 1000 || len(input.PangkatGol) > 100 || len(input.Pangkat) > 200 || len(input.Jabatan) > 200 {
		return store.TeacherChange{}, errors.New("data perubahan terlalu panjang")
	}
	if input.UnitID != nil {
		if targetUnit == nil || targetUnit.ID != *input.UnitID {
			return store.TeacherChange{}, errors.New("unit tujuan tidak ditemukan")
		}
		switch targetUnit.Type {
		case "sd", "tk", "smp", "skb", "dinas":
		default:
			return store.TeacherChange{}, errors.New("unit tujuan harus unit layanan, bukan Korwil")
		}
	}
	if input.PangkatGol != "" && teacher.ASNType == "pppk" && input.PangkatGol != "IX" {
		return store.TeacherChange{}, errors.New("golongan PPPK harus IX")
	}
	change := store.TeacherChange{
		EffectiveDate: input.EffectiveDate,
		Note:          input.Note,
		AuditDetails:  make(map[string]any),
	}
	if input.PangkatGol != "" && input.PangkatGol != teacher.PangkatGol {
		value := input.PangkatGol
		change.PangkatGol = &value
		change.AuditDetails["pangkat_gol_lama"] = teacher.PangkatGol
		change.AuditDetails["pangkat_gol_baru"] = value
	}
	if input.Pangkat != "" && input.Pangkat != teacher.Pangkat {
		value := input.Pangkat
		change.Pangkat = &value
		change.AuditDetails["pangkat_lama"] = teacher.Pangkat
		change.AuditDetails["pangkat_baru"] = value
	}
	if input.Jabatan != "" && input.Jabatan != teacher.Jabatan {
		value := input.Jabatan
		change.Jabatan = &value
		change.AuditDetails["jabatan_lama"] = teacher.Jabatan
		change.AuditDetails["jabatan_baru"] = value
	}
	if input.UnitID != nil && *input.UnitID != teacher.UnitID {
		value := *input.UnitID
		change.UnitID = &value
		change.AuditDetails["unit_lama"] = teacher.UnitName
		change.AuditDetails["unit_baru"] = targetUnit.Name
	}
	if len(change.AuditDetails) == 0 {
		return store.TeacherChange{}, errors.New("tidak ada perubahan data yang berbeda dari master BKN")
	}
	change.AuditDetails["tanggal_berlaku"] = input.EffectiveDate.Format("2006-01-02")
	change.AuditDetails["catatan"] = input.Note
	return change, nil
}

func (s *Server) teacherChangeFromForm(r *http.Request, teacher store.Teacher, proposedTMT time.Time) (store.TeacherChange, error) {
	unitID, err := parseOptionalFormInt64(r.FormValue("change_unit_id"))
	if err != nil {
		return store.TeacherChange{}, err
	}
	var targetUnit *store.Unit
	if unitID != nil {
		unit, err := store.GetUnitByID(r.Context(), s.Pool, *unitID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return store.TeacherChange{}, errors.New("unit tujuan tidak ditemukan")
			}
			return store.TeacherChange{}, err
		}
		targetUnit = &unit
	}
	effectiveDate, err := parseOptionalEffectiveDate(r.FormValue("change_effective_date"))
	if err != nil {
		return store.TeacherChange{}, err
	}
	return resolveTeacherChange(teacher, ProposedTeacherChangeInput{
		PangkatGol:    r.FormValue("change_pangkat_gol"),
		Pangkat:       r.FormValue("change_pangkat"),
		Jabatan:       r.FormValue("change_jabatan"),
		UnitID:        unitID,
		EffectiveDate: effectiveDate,
		Note:          r.FormValue("change_note"),
	}, targetUnit, proposedTMT)
}

func salaryGolongan(teacher store.Teacher, change store.TeacherChange) string {
	gol := teacher.PangkatGol
	if change.PangkatGol != nil {
		gol = *change.PangkatGol
	}
	if teacher.ASNType == "pppk" {
		return "IX"
	}
	return gol
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
	proposedTMT, err := time.Parse("2006-01-02", strings.TrimSpace(r.FormValue("proposed_tmt")))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Tanggal TMT tidak valid.")
		return
	}
	proposedMasaKerja, err := parseOptionalFormInt(r.FormValue("masa_kerja_tahun"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	proposedTMTKGBLast, err := parseOptionalFormDate(r.FormValue("tmt_kgb_last"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "TMT KGB terakhir tidak valid.")
		return
	}
	change, err := s.teacherChangeFromForm(r, t, proposedTMT)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "TEACHER_CHANGE_INVALID", err.Error())
		return
	}
	masaKerja, tmtKGBLast, err := resolveKGBInputs(t, proposedMasaKerja, proposedTMTKGBLast, proposedTMT)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "KGB_DATA_REQUIRED", err.Error())
		return
	}
	if tmtKGBLast != nil && proposedTMT.Before(tmtKGBLast.AddDate(2, 0, 0)) {
		writeErr(w, http.StatusUnprocessableEntity, "NOT_YET_ELIGIBLE", "Pengajuan belum mencapai dua tahun dari TMT KGB terakhir.")
		return
	}
	gol := salaryGolongan(t, change)
	current, next, err := store.SalaryCurrentNext(r.Context(), s.Pool, t.ASNType, gol, masaKerja)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "SALARY_SCALE_NOT_FOUND", "Kombinasi golongan/masa kerja tidak ada di skala gaji.")
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
	effectiveMasaKerja := masaKerja
	sub, err := store.CreateSubmission(r.Context(), s.Pool, t.ID, userFrom(r).ID, proposedTMT, &effectiveMasaKerja, tmtKGBLast, change, current, next, files.OriginalName(header.Filename), path, size, clientIP(r))
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
	proposedTMT, err := time.Parse("2006-01-02", strings.TrimSpace(r.FormValue("proposed_tmt")))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Tanggal TMT tidak valid.")
		return
	}
	proposedMasaKerja, err := parseOptionalFormInt(r.FormValue("masa_kerja_tahun"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	proposedTMTKGBLast, err := parseOptionalFormDate(r.FormValue("tmt_kgb_last"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "TMT KGB terakhir tidak valid.")
		return
	}
	change, err := s.teacherChangeFromForm(r, t, proposedTMT)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "TEACHER_CHANGE_INVALID", err.Error())
		return
	}
	masaKerja, tmtKGBLast, err := resolveKGBInputs(t, proposedMasaKerja, proposedTMTKGBLast, proposedTMT)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "KGB_DATA_REQUIRED", err.Error())
		return
	}
	if tmtKGBLast != nil && proposedTMT.Before(tmtKGBLast.AddDate(2, 0, 0)) {
		writeErr(w, http.StatusUnprocessableEntity, "NOT_YET_ELIGIBLE", "Pengajuan belum mencapai dua tahun dari TMT KGB terakhir.")
		return
	}
	gol := salaryGolongan(t, change)
	current, next, err := store.SalaryCurrentNext(r.Context(), s.Pool, t.ASNType, gol, masaKerja)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "SALARY_SCALE_NOT_FOUND", "Skala gaji belum tersedia.")
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
	effectiveMasaKerja := masaKerja
	sub, err := store.Resubmit(r.Context(), s.Pool, id, userFrom(r).ID, proposedTMT, &effectiveMasaKerja, tmtKGBLast, change, current, next, filepath.Base(header.Filename), path, size, clientIP(r))
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
	items, err := store.ListQueue(r.Context(), s.Pool, role, userFrom(r).UnitID, status)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil antrean.")
		return
	}
	writeData(w, http.StatusOK, items)
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
	letterPDF, err := s.Renderer.RenderLetter(r.Context(), pdf.LetterData{Number: issue.Number, TeacherName: issue.Submission.TeacherName, NIP: issue.Submission.NIP, UnitName: issue.Submission.UnitName, ProposedTMT: issue.Submission.ProposedTMT.Format("02-01-2006"), CurrentSalary: issue.Submission.CurrentSalary, NextSalary: issue.Submission.NextSalary, IssuedAt: pdf.TodayID(), SignerName: signer.Name})
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
