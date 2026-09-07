package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func getHome(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestHomePageShowsTrialNoticeWithCountdown(t *testing.T) {
	fx := newFixture(t)
	text := getHome(t, fx.srv.URL)
	for _, want := range []string{
		"Pemberitahuan Pelaksanaan Uji Coba Internal",
		"masa uji coba internal",
		"tidak memiliki kekuatan administrasi maupun hukum",
		"trial-countdown",
		"data-trial-until",
		"Sisa Waktu Masa Uji Coba",
		"trial-days",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("beranda tidak memuat %q", want)
		}
	}
}

func TestHomePageTrialNoticeRespectsTrialUntilEnv(t *testing.T) {
	t.Setenv("TRIAL_UNTIL", "2026-09-11")
	fx := newFixture(t)
	text := getHome(t, fx.srv.URL)
	if !strings.Contains(text, "2026-09-11") {
		t.Errorf("beranda tidak memakai TRIAL_UNTIL dari env")
	}
	if !strings.Contains(text, "11 September 2026") {
		t.Errorf("beranda tidak menampilkan tanggal akhir masa uji coba")
	}
}

func putTrialNotice(t *testing.T, client *http.Client, base, csrf string, body any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPut, base+"/api/v1/admin/trial-notice", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func getTrialNotice(t *testing.T, client *http.Client, base string) map[string]any {
	t.Helper()
	resp, err := client.Get(base + "/api/v1/admin/trial-notice")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	data, _ := out["data"].(map[string]any)
	if data == nil {
		t.Fatalf("GET trial-notice tanpa data: %v", out)
	}
	return data
}

func TestAdminCanDisableTrialNotice(t *testing.T) {
	fx := newFixture(t)
	client, csrf := loginClient(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	if code, _ := putTrialNotice(t, client, fx.srv.URL, csrf, map[string]any{"enabled": false}); code != http.StatusOK {
		t.Fatalf("PUT disable status=%d", code)
	}
	if text := getHome(t, fx.srv.URL); strings.Contains(text, "Pemberitahuan Pelaksanaan Uji Coba Internal") {
		t.Errorf("banner masih tampil setelah dimatikan")
	}
	data := getTrialNotice(t, client, fx.srv.URL)
	if data["enabled"] != false {
		t.Errorf("enabled = %v, want false", data["enabled"])
	}
	if code, _ := putTrialNotice(t, client, fx.srv.URL, csrf, map[string]any{"enabled": true}); code != http.StatusOK {
		t.Fatalf("PUT enable status=%d", code)
	}
	if text := getHome(t, fx.srv.URL); !strings.Contains(text, "Pemberitahuan Pelaksanaan Uji Coba Internal") {
		t.Errorf("banner tidak tampil setelah dihidupkan lagi")
	}
}

func TestAdminCanSetTrialUntilDate(t *testing.T) {
	fx := newFixture(t)
	client, csrf := loginClient(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	if code, _ := putTrialNotice(t, client, fx.srv.URL, csrf, map[string]any{"until": "2026-09-20"}); code != http.StatusOK {
		t.Fatalf("PUT until status=%d", code)
	}
	text := getHome(t, fx.srv.URL)
	if !strings.Contains(text, "20 September 2026") {
		t.Errorf("beranda tidak menampilkan tanggal dari dasbor admin")
	}
	if !strings.Contains(text, "2026-09-20") {
		t.Errorf("beranda tidak memakai data-trial-until dari dasbor admin")
	}
}

func TestAdminTrialUntilRejectsBadDate(t *testing.T) {
	fx := newFixture(t)
	client, csrf := loginClient(t, fx.srv.URL, "admin-dinas", "sandiadmin")
	if code, _ := putTrialNotice(t, client, fx.srv.URL, csrf, map[string]any{"until": "20-09-2026"}); code != http.StatusBadRequest {
		t.Fatalf("PUT tanggal salah status=%d, want 400", code)
	}
}

func TestTrialNoticeRequiresLogin(t *testing.T) {
	fx := newFixture(t)
	resp, err := http.Get(fx.srv.URL + "/api/v1/admin/trial-notice")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET tanpa login status=%d, want 401", resp.StatusCode)
	}
}
