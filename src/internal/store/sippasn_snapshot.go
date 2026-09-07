package store

import (
	"context"
	"sync"
	"time"
)

// SIPPASNSnapshot adalah salinan daftar pejabat SIPP ASN di memori dengan
// masa berlaku (TTL). Satu unduhan ±10 MB melayani seluruh request selama
// TTL sehingga login dan lookup tidak mengunduh ulang tiap request.
// SIPPASN adalah sumber utama data induk pegawai (keputusan owner 2026-09-03).
type SIPPASNSnapshot struct {
	mu        sync.RWMutex
	client    SIPPASNClient
	ttl       time.Duration
	officers  []SIPPASNOfficer
	byNIP     map[string]SIPPASNOfficer
	fetchedAt time.Time
	fetchErr  error
}

// NewSIPPASNSnapshot membuat snapshot dengan TTL; default 6 jam bila ttl <= 0.
func NewSIPPASNSnapshot(client SIPPASNClient, ttl time.Duration) *SIPPASNSnapshot {
	if ttl <= 0 {
		ttl = 6 * time.Hour
	}
	return &SIPPASNSnapshot{client: client, ttl: ttl}
}

// Ensure memuat ulang snapshot bila kosong atau kedaluwarsa. Gagal muat tidak
// menghapus data lama: hasil terakhir tetap dipakai sampai refresh berikutnya
// berhasil (stale-while-revalidate), sehingga SIPPASN mati tidak melumpuhkan login.
func (s *SIPPASNSnapshot) Ensure(ctx context.Context) error {
	s.mu.RLock()
	fresh := !s.fetchedAt.IsZero() && time.Since(s.fetchedAt) < s.ttl
	s.mu.RUnlock()
	if fresh {
		return nil
	}
	officers, err := s.client.FetchOfficers(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.fetchErr = err
		if s.byNIP != nil {
			return nil
		}
		return err
	}
	byNIP := make(map[string]SIPPASNOfficer, len(officers))
	for _, o := range officers {
		if o.NIP != "" {
			byNIP[o.NIP] = o
		}
	}
	s.officers = officers
	s.byNIP = byNIP
	s.fetchedAt = time.Now()
	s.fetchErr = nil
	return nil
}

// Lookup mengembalikan baris SIPPASN untuk NIP (ok=false bila tidak ada).
// Snapshot harus di-Ensure terlebih dahulu oleh pemanggil.
func (s *SIPPASNSnapshot) Lookup(nip string) (officer SIPPASNOfficer, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	officer, ok = s.byNIP[nip]
	return officer, ok
}

// Count mengembalikan jumlah baris pada snapshot terakhir.
func (s *SIPPASNSnapshot) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.officers)
}

// FetchedAt mengembalikan waktu unduh terakhir (nol bila belum pernah).
func (s *SIPPASNSnapshot) FetchedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fetchedAt
}

// Officers mengembalikan salinan seluruh baris snapshot untuk sinkron malam.
func (s *SIPPASNSnapshot) Officers() []SIPPASNOfficer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SIPPASNOfficer, len(s.officers))
	copy(out, s.officers)
	return out
}
