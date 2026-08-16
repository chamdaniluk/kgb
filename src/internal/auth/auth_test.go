package auth

import (
	"testing"
	"time"
)

func TestLimiterSlidingWindow(t *testing.T) {
	l := NewLimiter(3, 30*time.Millisecond)
	for i := 0; i < 3; i++ {
		if l.Blocked("k") {
			t.Fatalf("terblokir terlalu cepat pada percobaan %d", i+1)
		}
		l.RecordFail("k")
	}
	if !l.Blocked("k") {
		t.Fatal("harus terblokir setelah 3 gagal")
	}
	l.Reset("k")
	if l.Blocked("k") {
		t.Fatal("Reset harus membuka blokir")
	}
}

func TestLimiterWindowBergeser(t *testing.T) {
	l := NewLimiter(2, 20*time.Millisecond)
	l.RecordFail("k")
	l.RecordFail("k")
	if !l.Blocked("k") {
		t.Fatal("harus terblokir")
	}
	time.Sleep(25 * time.Millisecond)
	if l.Blocked("k") {
		t.Fatal("setelah window lewat harus terbuka lagi (sliding)")
	}
}

func TestSessionVerifyRevoke(t *testing.T) {
	m := NewSessionManager("secret-uji", time.Hour)
	tok, csrf, err := m.New(42)
	if err != nil {
		t.Fatal(err)
	}
	uid, gotCSRF, ok := m.Verify(tok)
	if !ok || uid != 42 || gotCSRF != csrf {
		t.Fatalf("Verify gagal: ok=%v uid=%d", ok, uid)
	}
	m.Revoke(tok)
	if _, _, ok := m.Verify(tok); ok {
		t.Fatal("sesi ter-revoke masih valid")
	}
}

func TestSessionTampered(t *testing.T) {
	m := NewSessionManager("secret-uji", time.Hour)
	tok, _, _ := m.New(7)
	if _, _, ok := m.Verify(tok + "x"); ok {
		t.Fatal("token yang diubah signature harus ditolak")
	}
	if _, _, ok := m.Verify("bogus"); ok {
		t.Fatal("token sampah harus ditolak")
	}
}

func TestSessionKedaluwarsa(t *testing.T) {
	m := NewSessionManager("secret-uji", 30*time.Millisecond)
	tok, _, _ := m.New(7)
	time.Sleep(40 * time.Millisecond)
	if _, _, ok := m.Verify(tok); ok {
		t.Fatal("sesi kedaluwarsa harus ditolak")
	}
}
