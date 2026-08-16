// Package httpapi: routing, middleware, dan handler HTTP SI CENDIKIA.
// Kontrak: API v1.2 (base /api/v1, envelope {data}/{error}).
package httpapi

import (
	"encoding/json"
	"net/http"

	"sicendikia/internal/auth"
	"sicendikia/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

const cookieName = "sic_session"

type Server struct {
	Pool         *pgxpool.Pool
	Sessions     *auth.SessionManager
	Limiter      *auth.Limiter
	SecureCookie bool
}

func New(pool *pgxpool.Pool, sessionSecret string, secureCookie bool) *Server {
	return &Server{
		Pool:         pool,
		Sessions:     auth.NewSessionManager(sessionSecret, sessionTTL),
		Limiter:      auth.NewLimiter(5, limiterWindow), // PRD F-4: 5 gagal / 15 menit
		SecureCookie: secureCookie,
	}
}

// Routes membangun mux API v1.2 + healthz.
func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.withAuth()(s.handleLogout))
	mux.HandleFunc("GET /api/v1/me", s.withAuth()(s.handleMe))
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeData(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- respons JSON (format API v1.2) ----

func writeData(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": v})
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}

// ---- middleware ----

type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxCSRF
)

// withAuth: wajib sesi valid; user diambil ulang dari DB (perubahan
// is_active/role langsung berlaku), lalu dicek bila ada batasan peran.
func (s *Server) withAuth(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(cookieName)
			if err != nil || c.Value == "" {
				writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Belum login.")
				return
			}
			uid, csrf, ok := s.Sessions.Verify(c.Value)
			if !ok {
				writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi tidak valid atau kedaluwarsa.")
				return
			}
			user, err := store.GetUserByID(r.Context(), s.Pool, uid)
			if err != nil || !user.IsActive {
				writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Akun tidak ditemukan atau nonaktif.")
				return
			}
			if len(roles) > 0 && !roleAllowed(user.Role, roles) {
				writeErr(w, http.StatusForbidden, "FORBIDDEN", "Peran tidak berwenang.")
				return
			}
			// CSRF: semua mutasi (non-GET/HEAD) wajib membawa header yang cocok.
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				if r.Header.Get("X-CSRF-Token") != csrf {
					writeErr(w, http.StatusForbidden, "CSRF_INVALID", "Token CSRF tidak valid.")
					return
				}
			}
			ctx := contextWith(r.Context(), user, csrf)
			next(w, r.WithContext(ctx))
		}
	}
}

func roleAllowed(role string, allowed []string) bool {
	for _, a := range allowed {
		if role == a {
			return true
		}
	}
	return false
}

func userFrom(r *http.Request) store.User {
	u, _ := r.Context().Value(ctxUser).(store.User)
	return u
}
