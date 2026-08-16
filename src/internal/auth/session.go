// Package auth: sesi, password, dan rate limit SI CENDIKIA.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SessionManager: sesi cookie HMAC-signed + daftar nonce aktif (agar logout
// benar-benar membatalkan sesi). Satu proses monolitik — map in-memory cukup
// (TECH-STACK: tanpa Redis); restart aplikasi = semua sesi hangus, aman.
type SessionManager struct {
	secret []byte
	ttl    time.Duration

	mu     sync.Mutex
	active map[string]time.Time // nonce -> kedaluwarsa
}

func NewSessionManager(secret string, ttl time.Duration) *SessionManager {
	return &SessionManager{
		secret: []byte(secret),
		ttl:    ttl,
		active: make(map[string]time.Time),
	}
}

// New membuat sesi baru; mengembalikan token cookie dan token CSRF.
func (m *SessionManager) New(userID int64) (token, csrf string, err error) {
	nonceB := make([]byte, 16)
	if _, err = rand.Read(nonceB); err != nil {
		return "", "", err
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceB)
	exp := time.Now().Add(m.ttl).Unix()

	payload := fmt.Sprintf("%d|%d|%s", userID, exp, nonce)
	token = m.sign(payload)

	m.mu.Lock()
	m.active[nonce] = time.Unix(exp, 0)
	m.mu.Unlock()
	return token, m.csrfFor(nonce), nil
}

// Verify memeriksa signature, kedaluwarsa, dan status aktif (belum di-revoke).
func (m *SessionManager) Verify(token string) (userID int64, csrf string, ok bool) {
	i := strings.LastIndexByte(token, '.')
	if i < 0 {
		return 0, "", false
	}
	payload, sig := token[:i], token[i+1:]
	want := m.mac(payload)
	if subtle.ConstantTimeCompare([]byte(sig), []byte(want)) != 1 {
		return 0, "", false
	}
	var exp int64
	var nonce string
	if _, err := fmt.Sscanf(payload, "%d|%d|%s", &userID, &exp, &nonce); err != nil {
		return 0, "", false
	}
	if time.Now().Unix() >= exp {
		return 0, "", false
	}
	m.mu.Lock()
	expActive, ada := m.active[nonce]
	m.mu.Unlock()
	if !ada || time.Unix(exp, 0) != expActive {
		return 0, "", false
	}
	return userID, m.csrfFor(nonce), true
}

// Revoke membatalkan sesi (logout).
func (m *SessionManager) Revoke(token string) {
	i := strings.LastIndexByte(token, '.')
	if i < 0 {
		return
	}
	payload := token[:i]
	var userID, exp int64
	var nonce string
	if _, err := fmt.Sscanf(payload, "%d|%d|%s", &userID, &exp, &nonce); err != nil {
		return
	}
	m.mu.Lock()
	delete(m.active, nonce)
	m.mu.Unlock()
}

func (m *SessionManager) sign(payload string) string {
	return payload + "." + m.mac(payload)
}

func (m *SessionManager) mac(payload string) string {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

func (m *SessionManager) csrfFor(nonce string) string {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte("csrf:" + nonce))
	return hex.EncodeToString(h.Sum(nil))
}
