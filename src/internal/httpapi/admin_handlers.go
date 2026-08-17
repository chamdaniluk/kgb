package httpapi

import (
	"encoding/csv"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"sicendikia/internal/auth"
	"sicendikia/internal/store"

	"github.com/xuri/excelize/v2"
)

const maxImportSize int64 = 32 << 20

var institutionCodePattern = regexp.MustCompile(`[^A-Z0-9]+`)

func canonicalInstitutionName(name string) string {
	name = strings.ToUpper(strings.TrimSpace(name))
	if name == "KORWILCAM GUBIG" {
		return "KORWILCAM GUBUG"
	}
	return name
}

func institutionCode(name string) string {
	name = canonicalInstitutionName(name)
	name = institutionCodePattern.ReplaceAllString(name, "-")
	return strings.Trim(name, "-")
}

func mapWorkbookStaffRole(role string) (string, bool, error) {
	switch normalizeHeader(role) {
	case "admin tte":
		return store.StaffRolePimpinan, true, nil
	case "admin dinas":
		return store.StaffRoleAdminDinas, false, nil
	case "admin korwil", "admin smp":
		return store.StaffRoleVerifikatorUnit, false, nil
	default:
		return "", false, fmt.Errorf("role workbook tidak dikenal: %q", role)
	}
}

func korwilNameFromInstitution(institution string) string {
	parts := strings.Fields(canonicalInstitutionName(institution))
	if len(parts) == 0 {
		return ""
	}
	return canonicalInstitutionName("KORWILCAM " + parts[len(parts)-1])
}

func readTabularUpload(file multipart.File, filename string) ([][]string, error) {
	if strings.HasSuffix(strings.ToLower(filename), ".csv") {
		return csv.NewReader(file).ReadAll()
	}
	book, err := excelize.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("baca Excel: %w", err)
	}
	defer book.Close()
	sheets := book.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("sheet Excel kosong")
	}
	return book.GetRows(sheets[0])
}

func normalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")
	return strings.Join(strings.Fields(s), " ")
}

func headerIndex(headers []string, names ...string) int {
	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[normalizeHeader(name)] = true
	}
	for i, header := range headers {
		if wanted[normalizeHeader(header)] {
			return i
		}
	}
	return -1
}

func cell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func parseDateCell(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" || strings.Contains(v, "0001") {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006", "02-01-2006", "2006/01/02"} {
		if t, err := time.Parse(layout, v); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("tanggal %q tidak valid", v)
}

// deriveMasaKerja menerapkan aturan bisnis sumber BKN.
// PNS hanya memakai TMT CPNS; PPPK memakai TMT CPNS dan fallback ke TMT GOL.
func deriveMasaKerja(asnType string, tmtCPNS, tmtGol *time.Time, asOf time.Time) (int, string) {
	candidate := tmtCPNS
	source := "tmt_cpns"
	if asnType == "pppk" && candidate == nil {
		candidate = tmtGol
		source = "tmt_gol"
	}
	if candidate == nil {
		return 0, "belum_tersedia"
	}
	if candidate.After(asOf) {
		return 0, "belum_tersedia"
	}
	years := asOf.Year() - candidate.Year()
	anniversary := candidate.AddDate(years, 0, 0)
	if anniversary.After(asOf) {
		years--
	}
	if years < 0 {
		years = 0
	}
	return years, source
}

type parsedBKNUnit struct {
	UnitCode       string
	UnitName       string
	UnitType       string
	ParentUnitCode string
	ParentUnitName string
}

var bknDistrictPattern = regexp.MustCompile(`(?i)KECAMATAN\s+([A-Z]+)`)

var bknDistricts = map[string]bool{
	"BRATI": true, "GABUS": true, "GEYER": true, "GODONG": true,
	"GROBOGAN": true, "GUBUG": true, "KARANGRAYUNG": true,
	"KEDUNGJATI": true, "KLAMBU": true, "KRADENAN": true,
	"NGARINGAN": true, "PENAWANGAN": true, "PULOKULON": true,
	"PURWODADI": true, "TANGGUNGHARJO": true, "TAWANGHARJO": true,
	"TEGOWANU": true, "TOROH": true, "WIROSARI": true,
}

func canonicalBKNUnitName(name string) string {
	name = canonicalInstitutionName(name)
	name = strings.TrimSpace(name)
	name = strings.Replace(name, "SMPN ", "SMP NEGERI ", 1)
	name = strings.Replace(name, " KELOMPOK SEKOLAH MENENGAH PERTAMA NEGERI", "", 1)
	return strings.Join(strings.Fields(name), " ")
}

func bknDistrictFromInstitution(raw string) string {
	upper := strings.ToUpper(raw)
	matches := bknDistrictPattern.FindAllStringSubmatch(upper, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		if len(matches[i]) > 1 && bknDistricts[matches[i][1]] {
			return matches[i][1]
		}
	}
	for district := range bknDistricts {
		if strings.Contains(upper, district) {
			return district
		}
	}
	return ""
}

func bknDistrictFromSchoolName(school string) string {
	upper := strings.ToUpper(strings.TrimSpace(school))
	for district := range bknDistricts {
		if strings.HasSuffix(upper, " "+district) || strings.Contains(upper, " "+district+" ") {
			return district
		}
	}
	return ""
}

func parseBKNUnit(raw string) (parsedBKNUnit, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return parsedBKNUnit{}, errors.New("instansi sub unit kosong")
	}
	if normalizeHeader(raw) == "dinas pendidikan" {
		return parsedBKNUnit{UnitCode: "DINAS-PENDIDIKAN", UnitName: "DINAS PENDIDIKAN", UnitType: "dinas"}, nil
	}
	parts := strings.SplitN(canonicalInstitutionName(raw), " - ", 2)
	school := canonicalBKNUnitName(parts[0])
	upper := strings.ToUpper(school)
	unitType := ""
	switch {
	case strings.HasPrefix(upper, "SDN ") || strings.HasPrefix(upper, "SD "):
		unitType = "sd"
	case strings.HasPrefix(upper, "TK "):
		unitType = "tk"
	case strings.HasPrefix(upper, "SMPN ") || strings.HasPrefix(upper, "SMP NEGERI ") || strings.HasPrefix(upper, "SMP "):
		unitType = "smp"
	case strings.Contains(upper, "SKB") || strings.HasPrefix(upper, "SPNF "):
		unitType = "skb"
	default:
		return parsedBKNUnit{}, fmt.Errorf("format unit BKN tidak dikenali: %q", school)
	}
	district := bknDistrictFromInstitution(raw)
	if district == "" {
		district = bknDistrictFromSchoolName(school)
	}
	if district == "" {
		return parsedBKNUnit{}, fmt.Errorf("kecamatan unit BKN tidak dapat dipetakan: %q", school)
	}
	parentName := "KORWILCAM " + district
	return parsedBKNUnit{
		UnitCode:       institutionCode(school),
		UnitName:       school,
		UnitType:       unitType,
		ParentUnitCode: institutionCode(parentName),
		ParentUnitName: parentName,
	}, nil
}

func parseImportTeachers(rows [][]string) ([]store.ImportedTeacher, error) {
	if len(rows) < 2 {
		return nil, errors.New("file impor tidak memiliki data")
	}
	h := rows[0]
	idxNIP := headerIndex(h, "NIP", "NIP Baru", "Nomor Induk Pegawai")
	idxName := headerIndex(h, "Nama", "Nama ASN", "Nama Pegawai")
	idxASN := headerIndex(h, "Jenis ASN", "Status ASN", "ASN Type", "Status Kepegawaian", "Status Pegawai")
	idxUnitCode := headerIndex(h, "Kode Unit", "Kode Sekolah", "Kode")
	idxUnit := headerIndex(h, "Unit", "Unit Kerja", "Nama Unit", "Sekolah", "Instansi Sub Unit")
	idxGol := headerIndex(h, "Pangkat Golongan", "Pangkat/Gol", "Golongan", "Pangkat", "Gol.")
	idxMKG := headerIndex(h, "Masa Kerja Tahun", "Masa Kerja", "MKG")
	idxTMT := headerIndex(h, "TMT KGB Terakhir", "TMT KGB")
	idxTMTCPNS := headerIndex(h, "TMT CPNS")
	idxTMTGOL := headerIndex(h, "TMT GOL", "TMT Golongan")
	idxBirthDate := headerIndex(h, "Tanggal Lahir", "Tgl Lahir")
	idxPangkat := headerIndex(h, "Pangkat")
	idxJabatan := headerIndex(h, "Jabatan")
	if idxNIP < 0 || idxName < 0 || idxASN < 0 || idxUnit < 0 || idxGol < 0 || (idxMKG < 0 && idxTMTCPNS < 0 && idxTMTGOL < 0) {
		return nil, errors.New("kolom wajib BKN: NIP, nama, status pegawai, instansi sub unit, golongan/pangkat, atau TMT CPNS/TMT GOL")
	}
	result := make([]store.ImportedTeacher, 0, len(rows)-1)
	asOf := time.Now()
	for _, row := range rows[1:] {
		if strings.TrimSpace(cell(row, idxNIP)) == "" {
			continue
		}
		asn := strings.ToLower(cell(row, idxASN))
		if strings.Contains(asn, "pppk") || strings.Contains(asn, "p3k") {
			asn = "pppk"
		} else if strings.Contains(asn, "pns") {
			asn = "pns"
		}
		unitName := cell(row, idxUnit)
		parsedUnit, err := parseBKNUnit(unitName)
		if err != nil {
			return nil, err
		}
		if idxUnitCode >= 0 && cell(row, idxUnitCode) != "" {
			parsedUnit.UnitCode = cell(row, idxUnitCode)
		}
		var tmtCPNS, tmtGOL, tmtKGB, birthDate *time.Time
		if idxTMTCPNS >= 0 {
			tmtCPNS, err = parseDateCell(cell(row, idxTMTCPNS))
			if err != nil {
				return nil, err
			}
		}
		if idxTMTGOL >= 0 {
			tmtGOL, err = parseDateCell(cell(row, idxTMTGOL))
			if err != nil {
				return nil, err
			}
		}
		if idxBirthDate >= 0 {
			birthDate, err = parseDateCell(cell(row, idxBirthDate))
			if err != nil {
				return nil, err
			}
		}
		if idxTMT >= 0 {
			tmtKGB, err = parseDateCell(cell(row, idxTMT))
			if err != nil {
				return nil, err
			}
		}
		mkg, source := 0, "belum_tersedia"
		if idxMKG >= 0 && strings.TrimSpace(cell(row, idxMKG)) != "" {
			mkg, err = strconv.Atoi(strings.TrimSpace(cell(row, idxMKG)))
			if err != nil || mkg < 0 {
				return nil, fmt.Errorf("masa kerja BKN tidak valid untuk NIP %s", cell(row, idxNIP))
			}
			source = "masa_kerja_bkn"
		} else {
			mkg, source = deriveMasaKerja(asn, tmtCPNS, tmtGOL, asOf)
		}
		result = append(result, store.ImportedTeacher{NIP: cell(row, idxNIP), Name: cell(row, idxName), ASNType: asn, UnitCode: parsedUnit.UnitCode, UnitName: parsedUnit.UnitName, UnitType: parsedUnit.UnitType, ParentUnitCode: parsedUnit.ParentUnitCode, ParentUnitName: parsedUnit.ParentUnitName, PangkatGol: cell(row, idxGol), Pangkat: cell(row, idxPangkat), Jabatan: cell(row, idxJabatan), BirthDate: birthDate, MasaKerjaTahun: mkg, MasaKerjaSource: source, TMTKGBLast: tmtKGB})
	}
	return result, nil
}

func parseImportStaff(rows [][]string) ([]store.ImportedStaffUser, error) {
	if len(rows) < 2 {
		return nil, errors.New("file akun petugas tidak memiliki data")
	}
	h := rows[0]
	idxUser := headerIndex(h, "Username", "User", "Nama Pengguna")
	idxPass := headerIndex(h, "Password", "Kata Sandi")
	idxRole := headerIndex(h, "Role", "Peran")
	idxName := headerIndex(h, "Nama", "Nama Petugas")
	idxInstitution := headerIndex(h, "Nama Institusi", "Institusi")
	if idxUser < 0 || idxPass < 0 || idxRole < 0 {
		return nil, errors.New("kolom wajib akun: username, password, role")
	}
	if idxName < 0 && idxInstitution < 0 {
		return nil, errors.New("kolom nama institusi atau nama petugas wajib diisi")
	}
	result := make([]store.ImportedStaffUser, 0, len(rows)-1)
	idxUnitCode := headerIndex(h, "Kode Unit", "Kode Sekolah")
	idxUnitName := headerIndex(h, "Unit", "Unit Kerja", "Nama Unit")
	idxUnitType := headerIndex(h, "Jenis Unit", "Tipe Unit")
	idxNIK := headerIndex(h, "NIK")
	idxSignature := headerIndex(h, "Signature Base64", "TTD Base64")
	for _, row := range rows[1:] {
		if cell(row, idxUser) == "" {
			continue
		}
		institution := canonicalInstitutionName(cell(row, idxInstitution))
		role, needsSigner, err := mapWorkbookStaffRole(cell(row, idxRole))
		if err != nil {
			return nil, err
		}
		unitName := cell(row, idxUnitName)
		unitCode := cell(row, idxUnitCode)
		unitType := strings.ToLower(cell(row, idxUnitType))
		parentName, parentCode := "", ""
		if institution != "" && normalizeHeader(institution) != "dinas pendidikan" {
			unitName = institution
			unitCode = institutionCode(institution)
			if normalizeHeader(cell(row, idxRole)) == "admin korwil" {
				unitType = "korwil"
			} else if strings.Contains(normalizeHeader(institution), "skb") {
				unitType = "skb"
			} else {
				unitType = "smp"
			}
			if unitType == "smp" || unitType == "skb" {
				parentName = korwilNameFromInstitution(institution)
				parentCode = institutionCode(parentName)
			}
		}
		if unitType == "" && role == store.StaffRoleVerifikatorUnit {
			return nil, fmt.Errorf("institusi/unit wajib untuk role %q", cell(row, idxRole))
		}
		name := cell(row, idxName)
		if name == "" {
			name = institution
		}
		result = append(result, store.ImportedStaffUser{
			Username: cell(row, idxUser), Password: cell(row, idxPass), Role: role, Name: name,
			UnitCode: unitCode, UnitName: unitName, UnitType: unitType, NIK: cell(row, idxNIK),
			SignatureImageBase64: cell(row, idxSignature), NeedsSignerProfile: needsSigner,
			ParentUnitCode: parentCode, ParentUnitName: parentName,
		})
	}
	return result, nil
}

func (s *Server) handleImportBKN(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportSize)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "FILE_REQUIRED", "File BKN wajib diunggah.")
		return
	}
	defer file.Close()
	rows, err := readTabularUpload(file, header.Filename)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "IMPORT_INVALID", err.Error())
		return
	}
	teachers, err := parseImportTeachers(rows)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "IMPORT_INVALID", err.Error())
		return
	}
	result, err := store.ImportTeachers(r.Context(), s.Pool, userFrom(r).ID, header.Filename, teachers, auth.HashPassword)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "IMPORT_FAILED", "Impor BKN gagal.")
		return
	}
	writeData(w, http.StatusOK, result)
}

func (s *Server) handleImportUsers(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportSize)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "FILE_REQUIRED", "File akun petugas wajib diunggah.")
		return
	}
	defer file.Close()
	rows, err := readTabularUpload(file, header.Filename)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "IMPORT_INVALID", err.Error())
		return
	}
	users, err := parseImportStaff(rows)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "IMPORT_INVALID", err.Error())
		return
	}
	result, err := store.ImportStaffUsers(r.Context(), s.Pool, userFrom(r).ID, users, auth.HashPassword)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "IMPORT_FAILED", "Impor akun petugas gagal.")
		return
	}
	writeData(w, http.StatusOK, result)
}

func (s *Server) handleAdminTeachers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	items, total, err := store.ListTeachers(r.Context(), s.Pool, r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		writeErr(w, 500, "INTERNAL", "Gagal mengambil master guru.")
		return
	}
	writeDataMeta(w, 200, items, map[string]any{"limit": limit, "offset": offset, "total": total})
}
func (s *Server) handleAdminTeacherDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeErr(w, 400, "INVALID_ID", "ID guru tidak valid.")
		return
	}
	item, err := store.GetTeacherByID(r.Context(), s.Pool, id)
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, 500, "INTERNAL", "Gagal mengambil guru.")
		return
	}
	writeData(w, 200, item)
}
func (s *Server) handleAdminUnits(w http.ResponseWriter, r *http.Request) {
	items, err := store.ListUnits(r.Context(), s.Pool)
	if err != nil {
		writeErr(w, 500, "INTERNAL", "Gagal mengambil unit.")
		return
	}
	writeData(w, 200, items)
}
func (s *Server) handleAdminCreateUnit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if decodeJSON(r, &req) != nil || req.Code == "" || req.Name == "" {
		writeErr(w, 400, "VALIDATION_ERROR", "Kode, nama, dan tipe unit wajib diisi.")
		return
	}
	if req.Type != "korwil" && req.Type != "sd" && req.Type != "tk" && req.Type != "smp" && req.Type != "skb" && req.Type != "dinas" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Tipe unit harus korwil, sd, tk, smp, skb, atau dinas.")
		return
	}
	item, err := store.CreateUnit(r.Context(), s.Pool, req.Code, req.Name, req.Type)
	if err != nil {
		if store.IsUniqueViolation(err) {
			writeErr(w, 409, "CONFLICT", "Kode unit sudah ada.")
			return
		}
		writeErr(w, 500, "INTERNAL", "Gagal membuat unit.")
		return
	}
	_ = store.TouchAudit(r.Context(), s.Pool, userFrom(r).ID, "buat_unit", nil, clientIP(r))
	writeData(w, 201, item)
}
func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	items, err := store.ListUsers(r.Context(), s.Pool, r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, 500, "INTERNAL", "Gagal mengambil pengguna.")
		return
	}
	writeData(w, 200, items)
}
func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		Role      string `json:"role"`
		Name      string `json:"name"`
		UnitID    *int64 `json:"unit_id"`
		NIK       string `json:"nik"`
		Signature string `json:"signature_image_base64"`
	}
	if decodeJSON(r, &req) != nil || req.Username == "" || req.Password == "" || req.Role == "" || req.Name == "" {
		writeErr(w, 400, "VALIDATION_ERROR", "Username, password, role, dan nama wajib diisi.")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, 500, "INTERNAL", "Password gagal diproses.")
		return
	}
	item, err := store.CreateStaffUser(r.Context(), s.Pool, req.Username, hash, req.Role, req.Name, req.UnitID, req.NIK, req.Signature)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeErr(w, 409, "CONFLICT", "Username sudah dipakai.")
			return
		}
		writeErr(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	_ = store.TouchAudit(r.Context(), s.Pool, userFrom(r).ID, "buat_pengguna", nil, clientIP(r))
	writeData(w, 201, item)
}
func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeErr(w, 400, "INVALID_ID", "ID pengguna tidak valid.")
		return
	}
	var req struct {
		Name      *string `json:"name"`
		Role      *string `json:"role"`
		UnitID    **int64 `json:"unit_id"`
		Active    *bool   `json:"is_active"`
		Password  *string `json:"password"`
		NIK       *string `json:"nik"`
		Signature *string `json:"signature_image_base64"`
	}
	if decodeJSON(r, &req) != nil {
		writeErr(w, 400, "VALIDATION_ERROR", "Data pengguna tidak valid.")
		return
	}
	var hash *string
	if req.Password != nil {
		h, e := auth.HashPassword(*req.Password)
		if e != nil {
			writeErr(w, 500, "INTERNAL", "Password gagal diproses.")
			return
		}
		hash = &h
	}
	item, err := store.UpdateStaffUser(r.Context(), s.Pool, id, req.Name, req.Role, req.UnitID, req.Active, hash, req.NIK, req.Signature)
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeErr(w, 500, "INTERNAL", "Gagal memperbarui pengguna.")
		return
	}
	writeData(w, 200, item)
}
func (s *Server) handleAdminSalaryScales(w http.ResponseWriter, r *http.Request) {
	items, err := store.ListSalaryScales(r.Context(), s.Pool, r.URL.Query().Get("asn_type"), r.URL.Query().Get("golongan"))
	if err != nil {
		writeErr(w, 500, "INTERNAL", "Gagal mengambil skala gaji.")
		return
	}
	writeData(w, 200, items)
}
func (s *Server) handleAdminImportSalary(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportSize)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, 400, "FILE_REQUIRED", "File skala gaji wajib diunggah.")
		return
	}
	defer file.Close()
	rows, err := readTabularUpload(file, header.Filename)
	if err != nil {
		writeErr(w, 422, "IMPORT_INVALID", err.Error())
		return
	}
	if len(rows) < 2 {
		writeErr(w, 422, "IMPORT_INVALID", "File skala kosong.")
		return
	}
	h := rows[0]
	ia, ig, im, ii := headerIndex(h, "Jenis ASN", "ASN Type"), headerIndex(h, "Golongan", "Pangkat/Gol"), headerIndex(h, "Masa Kerja Tahun", "Masa Kerja", "MKG"), headerIndex(h, "Gaji", "Gaji Pokok")
	if ia < 0 || ig < 0 || im < 0 || ii < 0 {
		writeErr(w, 422, "IMPORT_INVALID", "Kolom skala: jenis ASN, golongan, masa kerja, gaji.")
		return
	}
	scales := make([]store.SalaryScale, 0)
	for _, row := range rows[1:] {
		m, e := strconv.Atoi(cell(row, im))
		if e != nil {
			continue
		}
		scales = append(scales, store.SalaryScale{ASNType: strings.ToLower(cell(row, ia)), Golongan: cell(row, ig), MasaKerjaTahun: m, Gaji: cell(row, ii)})
	}
	if err := store.UpsertSalaryScales(r.Context(), s.Pool, scales); err != nil {
		writeErr(w, 500, "IMPORT_FAILED", "Impor skala gaji gagal.")
		return
	}
	_ = store.TouchAudit(r.Context(), s.Pool, userFrom(r).ID, "impor_skala_gaji", nil, clientIP(r))
	writeData(w, 200, map[string]any{"rows_imported": len(scales)})
}
func (s *Server) handleAdminTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := store.ListTemplates(r.Context(), s.Pool)
	if err != nil {
		writeErr(w, 500, "INTERNAL", "Gagal mengambil template nomor.")
		return
	}
	writeData(w, 200, items)
}
func (s *Server) handleAdminCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pattern string `json:"pattern"`
	}
	if decodeJSON(r, &req) != nil {
		writeErr(w, 400, "VALIDATION_ERROR", "Template tidak valid.")
		return
	}
	item, err := store.CreateTemplate(r.Context(), s.Pool, strings.TrimSpace(req.Pattern))
	if err != nil {
		writeErr(w, 400, "VALIDATION_ERROR", err.Error())
		return
	}
	_ = store.TouchAudit(r.Context(), s.Pool, userFrom(r).ID, "ubah_template_nomor", nil, clientIP(r))
	writeData(w, 201, item)
}
func (s *Server) handleAdminAudit(w http.ResponseWriter, r *http.Request) {
	var sid *int64
	if raw := r.URL.Query().Get("submission_id"); raw != "" {
		v, e := strconv.ParseInt(raw, 10, 64)
		if e != nil {
			writeErr(w, 400, "INVALID_ID", "ID pengajuan tidak valid.")
			return
		}
		sid = &v
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := store.ListAuditLogs(r.Context(), s.Pool, sid, r.URL.Query().Get("action"), limit)
	if err != nil {
		writeErr(w, 500, "INTERNAL", "Gagal mengambil audit.")
		return
	}
	writeData(w, 200, items)
}
