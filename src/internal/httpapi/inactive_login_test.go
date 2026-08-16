package httpapi

import (
	"context"
	"net/http"
	"testing"

	"sicendikia/internal/auth"
)

func TestLoginInactiveStaffIsRejected(t *testing.T) {
	fx := newFixture(t)
	hash, err := auth.HashPassword("sandi-tte")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.pool.Exec(context.Background(), `INSERT INTO users (username,password_hash,role,name,is_active) VALUES ('inactive-tte',$1,'pimpinan','TTE Belum Siap',false)`, hash); err != nil {
		t.Fatal(err)
	}
	resp, out := login(t, fx.srv.URL, "inactive-tte", "sandi-tte")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, body=%v; want 401", resp.StatusCode, out)
	}
	if _, ok := out["data"]; ok {
		t.Fatalf("inactive login returned data: %v", out)
	}
}
