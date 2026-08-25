package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"sicendikia/internal/auth"
	"sicendikia/internal/letterdata"
	"sicendikia/internal/store"
)

const (
	sessionTTL    = 12 * time.Hour
	limiterWindow = 15 * time.Minute
)

func contextWith(ctx context.Context, u store.User, csrf string) context.Context {
	ctx = context.WithValue(ctx, ctxUser, u)
	return context.WithValue(ctx, ctxCSRF, csrf)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// POST /api/v1/auth/login — API v1.2 §3.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "Username dan password wajib diisi.")
		return
	}

	key := req.Username + "|" + clientIP(r)
	if s.Limiter.Blocked(key) {
		writeErr(w, http.StatusTooManyRequests, "LOGIN_RATE_LIMITED",
			"Terlalu banyak percobaan gagal. Coba lagi dalam 15 menit.")
		return
	}

	user, err := store.GetUserByUsername(r.Context(), s.Pool, req.Username)
	if err != nil || !checkPassword(user, req.Password) || !user.IsActive {
		s.Limiter.RecordFail(key)
		details, _ := json.Marshal(map[string]string{"username": req.Username})
		_ = store.InsertAudit(r.Context(), s.Pool, store.AuditEntry{
			Action: "login_gagal", Details: details, IP: clientIP(r),
		})
		writeErr(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Username atau password salah.")
		return
	}

	if err := store.TouchLastLogin(r.Context(), s.Pool, user.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal memperbarui sesi login.")
		return
	}
	s.Limiter.Reset(key)

	token, csrf, err := s.Sessions.New(user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "Gagal membuat sesi.")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.SecureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})

	unit := ""
	if user.UnitName != nil {
		unit = *user.UnitName
	}
	details, _ := json.Marshal(map[string]string{"username": user.Username})
	_ = store.InsertAudit(r.Context(), s.Pool, store.AuditEntry{
		ActorUserID: &user.ID, Action: "login", Details: details, IP: clientIP(r),
	})

	writeData(w, http.StatusOK, map[string]any{
		"token": token,
		"csrf":  csrf,
		"role":  user.Role,
		"name":  user.Name,
		"unit":  unit,
	})
}

// Hash tetap dibanding walau user tidak ditemukan (anti timing-attack).
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func checkPassword(user store.User, password string) bool {
	if user.ID == 0 {
		auth.CheckPassword(dummyHash, password) // samakan durasi, hasil diabaikan
		return false
	}
	return auth.CheckPassword(user.PasswordHash, password)
}

// POST /api/v1/auth/logout — revoke sesi + audit.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if c, err := r.Cookie(cookieName); err == nil {
		s.Sessions.Revoke(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", HttpOnly: true,
		Secure: s.SecureCookie, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	_ = store.InsertAudit(r.Context(), s.Pool, store.AuditEntry{
		ActorUserID: &u.ID, Action: "logout", IP: clientIP(r),
	})
	writeData(w, http.StatusOK, map[string]string{"status": "logout"})
}

// GET /api/v1/me — profil; ASN mendapat data kepegawaian + pratinjau gaji.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	out := map[string]any{
		"username": u.Username,
		"role":     u.Role,
		"name":     u.Name,
		"csrf":     csrfFrom(r),
	}
	if u.UnitName != nil {
		out["unit"] = *u.UnitName
	}

	if u.Role == "asn" {
		t, err := store.GetTeacherByUserID(r.Context(), s.Pool, u.ID)
		if err != nil {
			writeErr(w, http.StatusNotFound, "TEACHER_NOT_FOUND", "Data kepegawaian tidak ditemukan.")
			return
		}
		me := map[string]any{
			"nip": t.NIP, "asn_type": t.ASNType, "unit": t.UnitName, "unit_id": t.UnitID,
			"pangkat_gol": t.PangkatGol, "pangkat": t.Pangkat, "jabatan": t.Jabatan,
			"masa_kerja_tahun": t.MasaKerjaTahun, "masa_kerja_source": t.MasaKerjaSource,
			"birth_place": t.BirthPlace, "karpeg": t.Karpeg,
			"last_sk_pejabat": t.LastSKPejabat, "last_sk_nomor": t.LastSKNomor,
		}
		if t.BirthDate != nil {
			me["birth_date"] = t.BirthDate.Format("2006-01-02")
		}
		if t.LastSKTanggal != nil {
			me["last_sk_tanggal"] = t.LastSKTanggal.Format("2006-01-02")
		}
		if t.LastSKTMTBerlaku != nil {
			me["last_sk_tmt"] = t.LastSKTMTBerlaku.Format("2006-01-02")
		}
		if t.LastSKMasaKerjaTahun != nil {
			me["mkg_lama_tahun"] = *t.LastSKMasaKerjaTahun
		}
		if t.LastSKMasaKerjaBulan != nil {
			me["mkg_lama_bulan"] = *t.LastSKMasaKerjaBulan
		}
		gol := t.PangkatGol
		if t.ASNType == "pppk" {
			gol = "IX" // guru PPPK selalu IX (ERD §7a)
		}
		if t.TMTAwal != nil {
			me["tmt_awal"] = t.TMTAwal.Format("2006-01-02")
		}
		// Rumus tunggal jadwal KGB (letterdata.NextDueTMT): anniversary
		// setelah SK terakhir di grid TMT awal; telat tidak menggeser siklus.
		var tmtBerlaku time.Time
		if t.TMTAwal != nil {
			prior := *t.TMTAwal
			if t.LastSKTMTBerlaku != nil && t.LastSKTMTBerlaku.After(prior) {
				prior = *t.LastSKTMTBerlaku
			}
			if t.TMTKGBLast != nil && t.TMTKGBLast.After(prior) {
				prior = *t.TMTKGBLast
			}
			tmtBerlaku = letterdata.NextDueTMT(*t.TMTAwal, prior, time.Now())
			me["proposed_tmt"] = tmtBerlaku.Format("2006-01-02")
		}
		if t.TMTKGBLast != nil {
			me["tmt_kgb_last"] = t.TMTKGBLast.Format("2006-01-02")
		}
		// Pra-isi gaji & masa kerja dari TMT awal → TMT berlaku (jalur B).
		// Tanpa TMT awal jangan menebak — tandai perlu dilengkapi.
		if t.TMTAwal != nil && t.MasaKerjaSource != "belum_tersedia" {
			masaBaru := letterdata.MasaKerjaFromTMT(*t.TMTAwal, tmtBerlaku)
			masaLama := masaBaru - 2
			if masaLama < 0 {
				masaLama = 0
			}
			cur, next, err := store.SalaryCurrentNext(r.Context(), s.Pool, t.ASNType, gol, masaLama)
			if err != nil {
				// Skala tak ketemu bukan alasan menggagalkan /me: dasbor tetap
				// dimuat, gaji dikosongkan, dan salary-preview menampilkan pesan
				// yang tepat saat guru mengisi form.
				me["data_perlu_dilengkapi"] = true
			} else {
				me["gaji_sekarang"] = cur
				me["gaji_berikutnya"] = next
				me["mkg_lama_tahun"] = masaLama
				me["mkg_lama_bulan"] = 0
				me["mkg_baru_tahun"] = masaLama + 2
				me["mkg_baru_bulan"] = 0
			}
		} else {
			me["data_perlu_dilengkapi"] = true
		}
		out["teacher"] = me
	}
	writeData(w, http.StatusOK, out)
}
