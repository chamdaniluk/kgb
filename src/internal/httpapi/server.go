// Package httpapi: routing, middleware, dan handler HTTP SI CENDIKIA.
// Kontrak: API v1.2 (base /api/v1, envelope {data}/{error}).
package httpapi

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"sicendikia/internal/auth"
	"sicendikia/internal/esign"
	"sicendikia/internal/files"
	"sicendikia/internal/pdf"
	"sicendikia/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

const cookieName = "sic_session"

type Server struct {
	Pool         *pgxpool.Pool
	Sessions     *auth.SessionManager
	Limiter      *auth.Limiter
	SecureCookie bool
	Files        *files.Storage
	Renderer     *pdf.Renderer
	Signer       *esign.Client
	Templates    *template.Template
	AppTemplate  *template.Template
	SIPPASN      *store.SIPPASNSnapshot
}

type Dependencies struct {
	Files    *files.Storage
	Renderer *pdf.Renderer
	Signer   *esign.Client
	SIPPASN  *store.SIPPASNSnapshot
}

func New(pool *pgxpool.Pool, sessionSecret string, secureCookie bool) *Server {
	root := filepath.Join(os.TempDir(), "si-cendikia-uploads")
	fileStore, _ := files.New(root)
	return NewWithDependencies(pool, sessionSecret, secureCookie, Dependencies{Files: fileStore, Renderer: pdf.NewRenderer()})
}

func NewWithDependencies(pool *pgxpool.Pool, sessionSecret string, secureCookie bool, deps Dependencies) *Server {
	if deps.Renderer == nil {
		deps.Renderer = pdf.NewRenderer()
	}
	return &Server{
		Pool:         pool,
		Sessions:     auth.NewSessionManager(sessionSecret, sessionTTL),
		Limiter:      auth.NewLimiter(5, limiterWindow), // PRD F-4: 5 gagal / 15 menit
		SecureCookie: secureCookie,
		Files:        deps.Files,
		Renderer:     deps.Renderer,
		Signer:       deps.Signer,
		Templates:    loadTemplates(),
		AppTemplate:  loadAppTemplate(),
		SIPPASN:      deps.SIPPASN,
	}
}

// Routes membangun mux API v1.2, halaman web, healthz, dan readiness.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	mux.HandleFunc("GET /static/", s.handleStatic)
	mux.HandleFunc("GET /favicon.ico", s.handleFavicon)
	mux.HandleFunc("GET /", s.handleHome)
	mux.HandleFunc("GET /login", s.handleLoginPage)
	mux.HandleFunc("GET /panduan", s.handleGuidePage)
	mux.HandleFunc("GET /alur", s.handleFlowPage)
	mux.HandleFunc("GET /app", s.handleAppPage)
	mux.HandleFunc("GET /guru", s.handleAppPage)
	mux.HandleFunc("GET /unit", s.handleAppPage)
	mux.HandleFunc("GET /dinas", s.handleAppPage)
	mux.HandleFunc("GET /pimpinan", s.handleAppPage)
	mux.HandleFunc("GET /admin", s.handleAppPage)

	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.withAuth()(s.handleLogout))
	mux.HandleFunc("GET /api/v1/me", s.withAuth()(s.handleMe))

	mux.HandleFunc("GET /api/v1/submissions", s.withAuth()(s.handleListSubmissions))
	mux.HandleFunc("GET /api/v1/salary-preview", s.withAuth("asn")(s.handleSalaryPreview))
	mux.HandleFunc("POST /api/v1/submissions", s.withAuth("asn")(s.handleCreateSubmission))
	mux.HandleFunc("GET /api/v1/submissions/{id}", s.withAuth()(s.handleGetSubmission))
	mux.HandleFunc("POST /api/v1/submissions/{id}/resubmit", s.withAuth("asn")(s.handleResubmit))
	mux.HandleFunc("GET /api/v1/submissions/{id}/file", s.withAuth()(s.handleSubmissionFile))

	mux.HandleFunc("GET /api/v1/verifications/unit", s.withAuth("verifikator_unit")(s.handleUnitQueue))
	mux.HandleFunc("GET /api/v1/verifications/unit/monitor", s.withAuth("verifikator_unit")(s.handleUnitMonitor))
	mux.HandleFunc("GET /api/v1/verifications/unit/{id}", s.withAuth("verifikator_unit")(s.handleUnitDetail))
	mux.HandleFunc("POST /api/v1/verifications/unit/{id}/approve", s.withAuth("verifikator_unit")(s.handleUnitApprove))
	mux.HandleFunc("POST /api/v1/verifications/unit/{id}/reject", s.withAuth("verifikator_unit")(s.handleUnitReject))
	mux.HandleFunc("GET /api/v1/verifications/dinas", s.withAuth("verifikator_dinas", "admin_dinas")(s.handleDinasQueue))
	mux.HandleFunc("GET /api/v1/verifications/dinas/{id}", s.withAuth("verifikator_dinas", "admin_dinas")(s.handleDinasDetail))
	mux.HandleFunc("POST /api/v1/verifications/dinas/{id}/approve", s.withAuth("verifikator_dinas", "admin_dinas")(s.handleDinasApprove))
	mux.HandleFunc("POST /api/v1/verifications/dinas/{id}/reject", s.withAuth("verifikator_dinas", "admin_dinas")(s.handleDinasReject))

	mux.HandleFunc("GET /api/v1/letters/pending-tte", s.withAuth("pimpinan", "admin_dinas", "verifikator_dinas")(s.handlePendingTTE))
	mux.HandleFunc("POST /api/v1/letters/{submission_id}/sign", s.withAuth("pimpinan")(s.handleSignLetter))
	mux.HandleFunc("GET /api/v1/letters/{submission_id}/draft-docx", s.withAuth("pimpinan", "admin_dinas", "verifikator_dinas")(s.handleDraftDOCX))
	mux.HandleFunc("GET /api/v1/letters/{id}/download", s.withAuth()(s.handleLetterDownload))
	mux.HandleFunc("GET /api/v1/letters", s.withAuth("verifikator_dinas", "admin_dinas", "pimpinan", "admin")(s.handleListLetters))
	mux.HandleFunc("GET /api/v1/reports/issued", s.withAuth(staffReportRoles()...)(s.handleIssuedHistory))
	mux.HandleFunc("GET /api/v1/reports/nominations", s.withAuth(staffReportRoles()...)(s.handleNominations))

	mux.HandleFunc("GET /api/v1/public/stats", s.handlePublicStats)
	mux.HandleFunc("GET /api/v1/units", s.withAuth()(s.handleAdminUnits))
	mux.HandleFunc("POST /api/v1/admin/import-bkn", s.withAuth("admin", "admin_dinas")(s.handleImportBKN))
	mux.HandleFunc("POST /api/v1/admin/import-users", s.withAuth("admin", "admin_dinas")(s.handleImportUsers))
	mux.HandleFunc("POST /api/v1/admin/sync-sippasn", s.withAuth("admin", "admin_dinas")(s.handleSyncSIPPASN))
	mux.HandleFunc("POST /api/v1/admin/sync-sippasn/preview", s.withAuth("admin", "admin_dinas")(s.handleSyncSIPPASNPreview))
	mux.HandleFunc("GET /api/v1/admin/sync-sippasn/history", s.withAuth("admin", "admin_dinas")(s.handleSyncSIPPASNHistory))
	mux.HandleFunc("GET /api/v1/admin/trial-notice", s.withAuth("admin", "admin_dinas")(s.handleTrialNoticeGet))
	mux.HandleFunc("PUT /api/v1/admin/trial-notice", s.withAuth("admin", "admin_dinas")(s.handleTrialNoticePut))
	mux.HandleFunc("GET /api/v1/admin/teachers", s.withAuth("admin", "admin_dinas")(s.handleAdminTeachers))
	mux.HandleFunc("GET /api/v1/admin/teachers/{id}", s.withAuth("admin", "admin_dinas")(s.handleAdminTeacherDetail))
	mux.HandleFunc("GET /api/v1/admin/units", s.withAuth("admin", "admin_dinas")(s.handleAdminUnits))
	mux.HandleFunc("POST /api/v1/admin/units", s.withAuth("admin", "admin_dinas")(s.handleAdminCreateUnit))
	mux.HandleFunc("PATCH /api/v1/admin/units/{id}", s.withAuth("admin", "admin_dinas")(s.handleAdminUpdateUnit))
	mux.HandleFunc("GET /api/v1/admin/users", s.withAuth("admin", "admin_dinas")(s.handleAdminUsers))
	mux.HandleFunc("POST /api/v1/admin/users", s.withAuth("admin", "admin_dinas")(s.handleAdminCreateUser))
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", s.withAuth("admin", "admin_dinas")(s.handleAdminUpdateUser))
	mux.HandleFunc("GET /api/v1/admin/salary-scales", s.withAuth("admin", "admin_dinas")(s.handleAdminSalaryScales))
	mux.HandleFunc("POST /api/v1/admin/salary-scales/import", s.withAuth("admin", "admin_dinas")(s.handleAdminImportSalary))
	mux.HandleFunc("GET /api/v1/admin/letter-number-templates", s.withAuth("admin", "admin_dinas")(s.handleAdminTemplates))
	mux.HandleFunc("POST /api/v1/admin/letter-number-templates", s.withAuth("admin", "admin_dinas")(s.handleAdminCreateTemplate))
	mux.HandleFunc("GET /api/v1/admin/audit-logs", s.withAuth("admin", "admin_dinas")(s.handleAdminAudit))
	mux.HandleFunc("GET /api/v1/admin/audit-logs/actions", s.withAuth("admin", "admin_dinas")(s.handleAdminAuditActions))
	mux.HandleFunc("GET /api/v1/admin/filter-options", s.withAuth("admin", "admin_dinas")(s.handleAdminFilterOptions))
	return securityHeaders(mux, s.SecureCookie)
}

func securityHeaders(next http.Handler, secure bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; base-uri 'self'; form-action 'self'")
		if secure {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeData(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	_, _ = w.Write(logoGroboganPNG)
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.Pool == nil || s.Pool.Ping(r.Context()) != nil {
		writeErr(w, http.StatusServiceUnavailable, "NOT_READY", "Basis data belum siap.")
		return
	}
	writeData(w, http.StatusOK, map[string]string{"status": "ready"})
}

// ---- respons JSON (format API v1.2) ----

func writeData(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": v})
}

func writeDataMeta(w http.ResponseWriter, status int, data any, meta any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "meta": meta})
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

// csrfFrom mengambil token CSRF sesi aktif dari context (diisi oleh withAuth).
func csrfFrom(r *http.Request) string {
	c, _ := r.Context().Value(ctxCSRF).(string)
	return c
}

// pageFrom membaca limit/offset dari query dan menormalkannya lewat store.NewPage.
// limit di luar {0,20,50,100} dipaksa ke default 20; 0 berarti semua baris.
func pageFrom(r *http.Request) store.Page {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	return store.NewPage(limit, offset)
}
