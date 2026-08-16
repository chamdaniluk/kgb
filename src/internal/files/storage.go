// Package files menyimpan berkas unggahan di luar document root.
package files

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const MaxUploadSize int64 = 5 * 1024 * 1024

var ErrNotPDF = errors.New("berkas bukan PDF")
var ErrTooLarge = errors.New("berkas melebihi 5MB")

// Storage adalah penyimpanan lokal sederhana untuk file privat aplikasi.
type Storage struct {
	Root string
}

// New membuat storage dan folder privatnya.
func New(root string) (*Storage, error) {
	if root == "" {
		return nil, errors.New("folder penyimpanan wajib diisi")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("buat folder penyimpanan: %w", err)
	}
	return &Storage{Root: filepath.Clean(root)}, nil
}

// SavePDF menyalin PDF ke nama acak dan mengembalikan path privat serta ukuran.
func (s *Storage) SavePDF(ctx context.Context, r io.Reader, originalName string, sizeHint int64) (path string, size int64, err error) {
	if sizeHint > MaxUploadSize {
		return "", 0, ErrTooLarge
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", 0, fmt.Errorf("buat nama file acak: %w", err)
	}
	path = filepath.Join(s.Root, fmt.Sprintf("%x.pdf", id[:]))
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", 0, fmt.Errorf("buat file unggahan: %w", err)
	}
	removeOnError := true
	defer func() {
		_ = f.Close()
		if removeOnError {
			_ = os.Remove(path)
		}
	}()
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	probe := make([]byte, 512)
	n, err := io.ReadFull(r, probe)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", 0, fmt.Errorf("baca header PDF: %w", err)
	}
	if n < 5 || string(probe[:5]) != "%PDF-" {
		return "", 0, ErrNotPDF
	}
	if _, err := f.Write(probe[:n]); err != nil {
		return "", 0, fmt.Errorf("simpan header PDF: %w", err)
	}
	size = int64(n)
	if n == len(probe) {
		written, err := io.Copy(io.MultiWriter(f, countWriter{n: &size}), io.LimitReader(r, MaxUploadSize-size+1))
		if err != nil {
			return "", 0, fmt.Errorf("simpan PDF: %w", err)
		}
		_ = written
	} else if size > MaxUploadSize {
		return "", 0, ErrTooLarge
	}
	if size > MaxUploadSize {
		return "", 0, ErrTooLarge
	}
	if err := f.Sync(); err != nil {
		return "", 0, fmt.Errorf("flush PDF: %w", err)
	}
	removeOnError = false
	return path, size, nil
}

type countWriter struct{ n *int64 }

func (w countWriter) Write(p []byte) (int, error) {
	*w.n += int64(len(p))
	return len(p), nil
}

// Open hanya membuka file yang berada di bawah root storage.
func (s *Storage) Open(path string) (*os.File, error) {
	clean := filepath.Clean(path)
	root := filepath.Clean(s.Root)
	if clean == root || len(clean) <= len(root) || clean[:len(root)+1] != root+string(os.PathSeparator) {
		return nil, os.ErrPermission
	}
	return os.Open(clean)
}

// Remove menghapus file privat setelah transaksi gagal.
func (s *Storage) Remove(path string) error {
	if path == "" {
		return nil
	}
	f, err := s.Open(path)
	if err != nil {
		return err
	}
	_ = f.Close()
	return os.Remove(path)
}

// OriginalName dipertahankan hanya untuk Content-Disposition; path tidak pernah
// berasal dari nama asli pengguna.
func OriginalName(name string) string { return filepath.Base(name) }
