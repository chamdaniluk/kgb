package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"sicendikia/internal/store"
)

// trialNoticeState menghitung status banner uji coba: DB menang, env cadangan,
// default 7 hari. enabled=false berarti banner disembunyikan.
type trialNoticeState struct {
	Enabled     bool
	Until       time.Time
	HasUntil    bool
	Source      string
	UntilTampil string
}

func (s *Server) trialNotice(ctx context.Context, r *http.Request) trialNoticeState {
	return s.trialNoticeFrom(ctx, trialNoticeTarget)
}

func (s *Server) trialNoticeFrom(ctx context.Context, fallback func() time.Time) trialNoticeState {
	st := trialNoticeState{Enabled: true, Source: "default"}
	if s.Pool != nil {
		if v, ok, err := store.GetSetting(ctx, s.Pool, store.SettingTrialNoticeEnabled); err == nil && ok {
			st.Enabled = strings.TrimSpace(strings.ToLower(v)) == "true"
			st.Source = "db"
		}
		if v, ok, err := store.GetSetting(ctx, s.Pool, store.SettingTrialNoticeUntil); err == nil && ok {
			if t, has, perr := store.ParseTrialUntil(v); perr == nil && has {
				st.Until, st.HasUntil = t, true
				st.Source = "db"
			}
		}
	}
	if !st.HasUntil {
		st.Until = fallback()
		if st.Source != "db" {
			if _, ok := lookupTrialUntilEnv(); ok {
				st.Source = "env"
			}
		}
	}
	st.UntilTampil = st.Until.Format("2 January 2006")
	return st
}

// GET /api/v1/admin/trial-notice — baca status banner uji coba.
// Hak: admin, admin_dinas. Tanpa audit (read-only).
func (s *Server) handleTrialNoticeGet(w http.ResponseWriter, r *http.Request) {
	st := s.trialNotice(r.Context(), r)
	var until any
	if st.HasUntil {
		until = st.Until.Format(time.RFC3339)
	}
	writeData(w, http.StatusOK, map[string]any{
		"enabled":       st.Enabled,
		"until":         until,
		"until_tampil":  st.UntilTampil,
		"until_efektif": st.Until.Format(time.RFC3339),
		"sumber":        st.Source,
	})
}

// PUT /api/v1/admin/trial-notice — ubah banner uji coba.
// Body: {"enabled": bool, "until": "YYYY-MM-DD"|null}. Hak: admin, admin_dinas.
func (s *Server) handleTrialNoticePut(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled *bool   `json:"enabled"`
		Until   *string `json:"until"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Body JSON tidak valid.")
		return
	}
	if req.Enabled == nil && req.Until == nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Tidak ada perubahan yang dikirim.")
		return
	}
	actor := userFrom(r).ID
	if req.Enabled != nil {
		val := "false"
		if *req.Enabled {
			val = "true"
		}
		if err := store.SetSetting(r.Context(), s.Pool, store.SettingTrialNoticeEnabled, val, actor); err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal menyimpan pengaturan.")
			return
		}
	}
	if req.Until != nil {
		raw := strings.TrimSpace(*req.Until)
		if raw != "" {
			if _, has, err := store.ParseTrialUntil(raw); err != nil || !has {
				writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Format tanggal tidak valid (gunakan YYYY-MM-DD).")
				return
			}
			if len(raw) == len("2006-01-02") {
				if _, err := time.Parse("2006-01-02", raw); err != nil {
					writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Format tanggal tidak valid (gunakan YYYY-MM-DD).")
					return
				}
			}
		}
		if err := store.SetSetting(r.Context(), s.Pool, store.SettingTrialNoticeUntil, raw, actor); err != nil {
			writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal menyimpan tanggal.")
			return
		}
	}
	_ = store.TouchAudit(r.Context(), s.Pool, actor, "ubah_pengumuman_uji_coba", nil, clientIP(r))
	s.handleTrialNoticeGet(w, r)
}
