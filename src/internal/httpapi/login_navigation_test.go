package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestAuthenticatedUserIsRedirectedAwayFromLoginPage(t *testing.T) {
	fx := newFixture(t)
	client, _ := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Get(fx.srv.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("GET /login status=%d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if got := resp.Header.Get("Location"); got != "/app" {
		t.Fatalf("redirect location=%q, want /app", got)
	}
}

func TestAuthenticatedAppPageIsBareSPAShell(t *testing.T) {
	fx := newFixture(t)
	client, _ := loginClient(t, fx.srv.URL, nipPNS, nipPNS)
	resp, err := client.Get(fx.srv.URL + "/app")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body []byte
	body = make([]byte, 0)
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		body = append(body, buf[:n]...)
		if readErr != nil {
			break
		}
	}
	text := string(body)
	if !strings.Contains(text, `src="/static/app.js`) {
		t.Fatalf("app page does not load app.js: %s", text)
	}
	if !strings.Contains(text, `id="app"`) {
		t.Fatalf("app page does not contain SPA mount point: %s", text)
	}
	if strings.Contains(text, `href="/login">Masuk</a>`) {
		t.Fatalf("authenticated app page still contains login link")
	}
}
