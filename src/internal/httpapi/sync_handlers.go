package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"sicendikia/internal/auth"
	"sicendikia/internal/store"
)

// Konfigurasi konektor SIPP ASN via environment (kredensial tidak pernah di-hardcode).
func sippASNBaseURL() string {
	if v := os.Getenv("SIPPASN_BASE_URL"); v != "" {
		return v
	}
	return "https://sippasn.grobogan.go.id"
}

// SIPPASNSnapshotDefault membangun snapshot dari environment untuk server produksi.
func SIPPASNSnapshotDefault() *store.SIPPASNSnapshot {
	timeout := sippASNTimeout()
	client := store.SIPPASNClient{BaseURL: sippASNBaseURL(), HTTP: &http.Client{Timeout: timeout}}
	ttl := 6 * time.Hour
	if v := os.Getenv("SIPPASN_SNAPSHOT_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= time.Minute {
			ttl = d
		}
	}
	return store.NewSIPPASNSnapshot(client, ttl)
}

func sippASNTimeout() time.Duration {
	if v := os.Getenv("SIPPASN_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 300 {
			return time.Duration(n) * time.Second
		}
	}
	return 60 * time.Second
}

// syncConfigDefault: sinkron hanya guru aktif di unit Dinas Pendidikan,
// sesuai cakupan piloting SI CENDIKIA.
func syncConfigDefault() store.SIPPASNSyncConfig {
	return store.SIPPASNSyncConfig{HanyaUnitPendidikan: true, HanyaStatusAktif: true}
}

// POST /api/v1/admin/sync-sippasn — tarik daftar pejabat dari SIPP ASN lalu
// upsert ke master teachers. Body JSON opsional: {"limit": 100} untuk uji
// coba bertahap. Hak: admin, admin_dinas. Audit: sinkron_sippasn.
func (s *Server) handleSyncSIPPASN(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Limit int `json:"limit"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Body JSON tidak valid.")
			return
		}
	}
	if req.Limit < 0 || req.Limit > 20000 {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Batas limit 0 sampai 20000.")
		return
	}
	cfg := syncConfigDefault()
	cfg.BatasiJumlah = req.Limit
	client := store.SIPPASNClient{BaseURL: sippASNBaseURL(), HTTP: &http.Client{Timeout: sippASNTimeout()}}
	ctx, cancel := context.WithTimeout(r.Context(), sippASNTimeout()+30*time.Second)
	defer cancel()
	officers, err := client.FetchOfficers(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "SYNC_UPSTREAM", "Gagal mengambil data SIPP ASN: "+err.Error())
		return
	}
	result, err := store.ImportSIPPASNOfficers(r.Context(), s.Pool, userFrom(r).ID, officers, cfg, auth.HashPassword)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "SYNC_FAILED", "Sinkronisasi SIPP ASN gagal.")
		return
	}
	_ = store.TouchAudit(r.Context(), s.Pool, userFrom(r).ID, "sinkron_sippasn_selesai", nil, clientIP(r))
	writeData(w, http.StatusOK, result)
}

// POST /api/v1/admin/sync-sippasn/preview — ambil data SIPP ASN tanpa menulis
// ke database; mengembalikan 5 contoh baris + hitungan kelayakan.
// Hak: admin, admin_dinas. Tanpa audit (read-only).
func (s *Server) handleSyncSIPPASNPreview(w http.ResponseWriter, r *http.Request) {
	client := store.SIPPASNClient{BaseURL: sippASNBaseURL(), HTTP: &http.Client{Timeout: sippASNTimeout()}}
	ctx, cancel := context.WithTimeout(r.Context(), sippASNTimeout()+30*time.Second)
	defer cancel()
	officers, err := client.FetchOfficers(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "SYNC_UPSTREAM", "Gagal mengambil data SIPP ASN: "+err.Error())
		return
	}
	cfg := syncConfigDefault()
	layak, terlewat := 0, 0
	contoh := make([]store.ImportedTeacher, 0, 5)
	for _, o := range officers {
		t, ok := store.MapSIPPASNOfficer(o, cfg)
		if !ok {
			terlewat++
			continue
		}
		layak++
		if len(contoh) < 5 {
			contoh = append(contoh, t)
		}
	}
	writeData(w, http.StatusOK, map[string]any{
		"sumber":        sippASNBaseURL(),
		"rows_total":    len(officers),
		"rows_layak":    layak,
		"rows_terlewat": terlewat,
		"contoh":        contoh,
		"filter":        "guru aktif, unit Dinas Pendidikan",
		"catatan_tulis": "preview tidak menulis ke database",
	})
}

// GET /api/v1/admin/sync-sippasn/history?limit=10 — riwayat sinkronisasi.
// Hak: admin, admin_dinas.
func (s *Server) handleSyncSIPPASNHistory(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	items, err := store.ListSyncHistory(r.Context(), s.Pool, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal mengambil riwayat sinkron.")
		return
	}
	writeData(w, http.StatusOK, items)
}
