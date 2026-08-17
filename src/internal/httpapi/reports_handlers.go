package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"sicendikia/internal/store"
)

func staffReportRoles() []string {
	return []string{"admin", "admin_dinas", "verifikator_dinas", "verifikator_unit", "pimpinan"}
}

func (s *Server) handleIssuedHistory(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	unitID, _ := strconv.ParseInt(r.URL.Query().Get("unit_id"), 10, 64)
	items, err := store.ListIssuedSubmissions(r.Context(), s.Pool, u.Role, u.UnitID, store.IssuedFilter{
		Query:  r.URL.Query().Get("q"),
		Year:   year,
		UnitID: unitID,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil riwayat pengajuan.")
		return
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) handleNominations(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	items, err := store.ListNominations(r.Context(), s.Pool, u.Role, u.UnitID, store.NominationFilter{
		Query:  r.URL.Query().Get("q"),
		Bucket: r.URL.Query().Get("bucket"),
	}, time.Now())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil nominasi KGB.")
		return
	}
	writeData(w, http.StatusOK, items)
}
