package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sicendikia/internal/auth"
	"sicendikia/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

const nipPNS = "198001012005011001"  // III/b, masa kerja 12
const nipPPPK = "198501012010012002" // guru PPPK (IX), masa kerja 10

type fixture struct {
	pool *pgxpool.Pool
	srv  *httptest.Server
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := testPoolDB(t)
	resetSchema(t, pool)
	ctx := context.Background()

	if _, err := store.Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("migrasi: %v", err)
	}
	seed, err := os.ReadFile(filepath.Join("..", "..", "data", "seeds", "001_salary_scales.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(seed)); err != nil {
		t.Fatalf("seed gaji: %v", err)
	}

	// Fixture: 1 unit, guru PNS + PPPK (akun NIP/NIP), admin dinas.
	hash := func(pw string) string {
		h, err := auth.HashPassword(pw)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, q, args...); err != nil {
			t.Fatalf("fixture: %v\n%s", err, q)
		}
	}
	insertReturningID := func(q string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, q+" RETURNING id", args...).Scan(&id); err != nil {
			t.Fatalf("fixture: %v\n%s", err, q)
		}
		return id
	}

	unitID := insertReturningID(
		`INSERT INTO units (code, name, type) VALUES ('KORWIL-01', 'Korwil Kec. Grobogan', 'korwil')`)
	userPNS := insertReturningID(
		`INSERT INTO users (username, password_hash, role, name, unit_id) VALUES ($1, $2, 'asn', 'Guru PNS Contoh', $3)`,
		nipPNS, hash(nipPNS), unitID)
	insertReturningID(
		`INSERT INTO teachers (user_id, nip, name, asn_type, unit_id, pangkat_gol, masa_kerja_tahun, masa_kerja_source, tmt_kgb_last, tmt_awal)
		 VALUES ($1, $2, 'Guru PNS Contoh', 'pns', $3, 'III/b', 12, 'tmt_cpns', '2024-04-01', '2016-04-01')`,
		userPNS, nipPNS, unitID)
	userPPPK := insertReturningID(
		`INSERT INTO users (username, password_hash, role, name, unit_id) VALUES ($1, $2, 'asn', 'Guru PPPK Contoh', $3)`,
		nipPPPK, hash(nipPPPK), unitID)
	insertReturningID(
		`INSERT INTO teachers (user_id, nip, name, asn_type, unit_id, pangkat_gol, masa_kerja_tahun, masa_kerja_source, tmt_kgb_last, tmt_awal)
		 VALUES ($1, $2, 'Guru PPPK Contoh', 'pppk', $3, 'IX', 10, 'tmt_cpns', '2024-04-01', '2018-04-01')`,
		userPPPK, nipPPPK, unitID)
	mustExec(
		`INSERT INTO users (username, password_hash, role, name) VALUES ('admin-dinas', $1, 'admin', 'Admin Dinas')`,
		hash("sandiadmin"))

	api := New(pool, "secret-test", false)
	ts := httptest.NewServer(api.Routes())
	t.Cleanup(ts.Close)
	return fixture{pool: pool, srv: ts}
}

func testPoolDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("SI_CENDIKIA_TEST_DB")
	if url == "" {
		url = "dbname=si_cendikia_test sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := store.Open(ctx, url)
	if err != nil {
		t.Skipf("database uji tidak tersedia: %v", err)
	}
	lockConn, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		t.Skipf("koneksi lock database uji tidak tersedia: %v", err)
	}
	if _, err := lockConn.Exec(ctx, `SELECT pg_advisory_lock(hashtext('si-cendikia-test-suite'))`); err != nil {
		lockConn.Release()
		pool.Close()
		t.Skipf("lock database uji tidak tersedia: %v", err)
	}
	t.Cleanup(func() {
		_, _ = lockConn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext('si-cendikia-test-suite'))`)
		lockConn.Release()
		pool.Close()
	})
	return pool
}

func resetSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatalf("reset skema: %v", err)
	}
}

// login membantu: kembalikan respons penuh + cookie sesi.
func login(t *testing.T, base, user, pass string) (*http.Response, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	resp, err := http.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func TestLoginSukses(t *testing.T) {
	fx := newFixture(t)
	resp, out := login(t, fx.srv.URL, nipPNS, nipPNS)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body=%v", resp.StatusCode, out)
	}
	data := out["data"].(map[string]any)
	if data["role"] != "asn" || data["unit"] != "Korwil Kec. Grobogan" {
		t.Errorf("data login salah: %v", data)
	}
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == cookieName {
			cookie = c
		}
	}
	if cookie == nil || cookie.Value == "" {
		t.Fatal("cookie sesi tidak terkirim")
	}
	n, _ := store.CountAudit(context.Background(), fx.pool, "login")
	if n != 1 {
		t.Errorf("audit login = %d, ingin 1", n)
	}
}

func TestLoginSalahLaluRateLimit(t *testing.T) {
	fx := newFixture(t)
	for i := 1; i <= 5; i++ {
		resp, out := login(t, fx.srv.URL, nipPNS, "password-salah")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("percobaan %d: status = %d, ingin 401, body=%v", i, resp.StatusCode, out)
		}
	}
	resp, out := login(t, fx.srv.URL, nipPNS, nipPNS) // benar pun ditolak
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, ingin 429, body=%v", resp.StatusCode, out)
	}
	errObj := out["error"].(map[string]any)
	if errObj["code"] != "LOGIN_RATE_LIMITED" {
		t.Errorf("kode error = %v, ingin LOGIN_RATE_LIMITED", errObj["code"])
	}
	n, _ := store.CountAudit(context.Background(), fx.pool, "login_gagal")
	if n != 5 {
		t.Errorf("audit login_gagal = %d, ingin 5", n)
	}
}

func TestMeTanpaSesi(t *testing.T) {
	fx := newFixture(t)
	resp, err := http.Get(fx.srv.URL + "/api/v1/me")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, ingin 401", resp.StatusCode)
	}
}

func TestMeGuruPNS(t *testing.T) {
	fx := newFixture(t)
	resp, _ := login(t, fx.srv.URL, nipPNS, nipPNS)
	cookie := pickCookie(t, resp)

	req, _ := http.NewRequest("GET", fx.srv.URL+"/api/v1/me", nil)
	req.AddCookie(cookie)
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(r2.Body).Decode(&out)
	teacher := out["data"].(map[string]any)["teacher"].(map[string]any)
	// Sumber: PP 5/2024 — gaji diambil dari salary_scales berdasarkan
	// TMT awal → TMT berlaku; lengkap = gaji muncul, tidak boleh kosong.
	if teacher["gaji_sekarang"] == "" || teacher["gaji_berikutnya"] == "" {
		t.Errorf("gaji belum muncul padahal semua isian lengkap: %v", teacher)
	}
	if teacher["tmt_kgb_last"] != "2024-04-01" {
		t.Errorf("tmt_kgb_last = %v", teacher["tmt_kgb_last"])
	}
	lama, _ := teacher["mkg_lama_tahun"].(float64)
	baru, _ := teacher["mkg_baru_tahun"].(float64)
	if lama == 0 || baru != lama+2 {
		t.Errorf("masa kerja tidak konsisten (baru harus lama+2): %v", teacher)
	}
	if teacher["data_perlu_dilengkapi"] == true {
		t.Errorf("data_perlu_dilengkapi tidak boleh true pada data lengkap: %v", teacher)
	}
}

func TestMeGuruPPPK(t *testing.T) {
	fx := newFixture(t)
	resp, _ := login(t, fx.srv.URL, nipPPPK, nipPPPK)
	cookie := pickCookie(t, resp)

	req, _ := http.NewRequest("GET", fx.srv.URL+"/api/v1/me", nil)
	req.AddCookie(cookie)
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(r2.Body).Decode(&out)
	teacher := out["data"].(map[string]any)["teacher"].(map[string]any)
	// PPPK IX — PP 11/2024 — juga lewat jalur (b); isian lengkap = gaji muncul.
	if teacher["gaji_sekarang"] == "" || teacher["gaji_berikutnya"] == "" {
		t.Errorf("pratinjau gaji PPPK belum muncul padahal semua isian lengkap: %v", teacher)
	}
	lama, _ := teacher["mkg_lama_tahun"].(float64)
	baru, _ := teacher["mkg_baru_tahun"].(float64)
	if lama == 0 || baru != lama+2 {
		t.Errorf("masa kerja PPPK tidak konsisten (baru harus lama+2): %v", teacher)
	}
	if teacher["data_perlu_dilengkapi"] == true {
		t.Errorf("data_perlu_dilengkapi tidak boleh true pada PPPK lengkap: %v", teacher)
	}
}

func TestMeSkalaTidakAda(t *testing.T) {
	fx := newFixture(t)
	// Masa kerja di luar cakupan PP 5/2024 harus membuat /me tetap 200
	// (tidak memblokir dasbor) dengan sinyal data_perlu_dilengkapi, tanpa gaji.
	if _, err := fx.pool.Exec(context.Background(),
		`UPDATE teachers SET tmt_awal = '1940-04-01' WHERE nip = $1`, nipPNS); err != nil {
		t.Fatal(err)
	}
	resp, _ := login(t, fx.srv.URL, nipPNS, nipPNS)
	cookie := pickCookie(t, resp)

	req, _ := http.NewRequest("GET", fx.srv.URL+"/api/v1/me", nil)
	req.AddCookie(cookie)
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(r2.Body).Decode(&out)
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, ingin 200, body %v", r2.StatusCode, out)
	}
	teacher := out["data"].(map[string]any)["teacher"].(map[string]any)
	if _, ada := teacher["gaji_sekarang"]; ada {
		t.Fatalf("gaji_sekarang tidak boleh ada ketika skala di luar tabel: %v", teacher)
	}
	if teacher["data_perlu_dilengkapi"] != true {
		t.Errorf("data_perlu_dilengkapi harus true: %v", teacher)
	}
}

func TestCSRFCookieCurangLogout(t *testing.T) {
	fx := newFixture(t)
	resp, out := login(t, fx.srv.URL, nipPNS, nipPNS)
	cookie := pickCookie(t, resp)
	csrf := out["data"].(map[string]any)["csrf"].(string)

	// Tanpa header CSRF → 403
	req, _ := http.NewRequest("POST", fx.srv.URL+"/api/v1/auth/logout", nil)
	req.AddCookie(cookie)
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r2.Body.Close()
	if r2.StatusCode != http.StatusForbidden {
		t.Fatalf("tanpa CSRF: status = %d, ingin 403", r2.StatusCode)
	}

	// Dengan header CSRF → 200, sesi mati
	req, _ = http.NewRequest("POST", fx.srv.URL+"/api/v1/auth/logout", nil)
	req.AddCookie(cookie)
	req.Header.Set("X-CSRF-Token", csrf)
	r3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r3.Body.Close()
	if r3.StatusCode != http.StatusOK {
		t.Fatalf("logout: status = %d, ingin 200", r3.StatusCode)
	}

	// Setelah logout, /me pakai cookie lama → 401
	req, _ = http.NewRequest("GET", fx.srv.URL+"/api/v1/me", nil)
	req.AddCookie(cookie)
	r4, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r4.Body.Close()
	if r4.StatusCode != http.StatusUnauthorized {
		t.Fatalf("setelah logout: status = %d, ingin 401", r4.StatusCode)
	}
}

func TestCookieDipalsukan(t *testing.T) {
	fx := newFixture(t)
	req, _ := http.NewRequest("GET", fx.srv.URL+"/api/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "1|9999999999|AAAA.payloadpalsu"})
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r2.Body.Close()
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, ingin 401", r2.StatusCode)
	}
}

func TestMeAdmin(t *testing.T) {
	fx := newFixture(t)
	resp, out := login(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login admin gagal: %d %v", resp.StatusCode, out)
	}
	cookie := pickCookie(t, resp)
	req, _ := http.NewRequest("GET", fx.srv.URL+"/api/v1/me", nil)
	req.AddCookie(cookie)
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(r2.Body).Decode(&body)
	data := body["data"].(map[string]any)
	if data["role"] != "admin" {
		t.Errorf("role = %v", data["role"])
	}
	if _, ada := data["teacher"]; ada {
		t.Error("admin tidak boleh punya blok teacher")
	}
}

func pickCookie(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == cookieName {
			return c
		}
	}
	t.Fatal("cookie sesi tidak ditemukan")
	return nil
}
