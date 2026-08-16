package auth

import (
	"sync"
	"time"
)

// Limiter: sliding window percobaan login gagal per username+IP.
// PRD F-4: 5 gagal / 15 menit -> 429. In-memory (pola e-KGB, tanpa Redis).
type Limiter struct {
	max    int
	window time.Duration

	mu    sync.Mutex
	fails map[string][]time.Time
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, fails: make(map[string][]time.Time)}
}

func (l *Limiter) prune(key string, now time.Time) {
	fs := l.fails[key]
	keep := fs[:0]
	for _, t := range fs {
		if now.Sub(t) < l.window {
			keep = append(keep, t)
		}
	}
	l.fails[key] = keep
}

// Blocked melaporkan apakah key sudah melewati batas percobaan gagal.
func (l *Limiter) Blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.prune(key, now)
	return len(l.fails[key]) >= l.max
}

// RecordFail mencatat satu kegagalan login.
func (l *Limiter) RecordFail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(key, time.Now())
	l.fails[key] = append(l.fails[key], time.Now())
}

// Reset menghapus catatan gagal (dipanggil saat login berhasil).
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}
