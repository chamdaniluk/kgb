package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"sicendikia/internal/httpapi"
	"sicendikia/internal/store"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	addr := envOr("ADDR", ":8080")
	// Dev lokal: unix socket + peer auth (tanpa password). Produksi: set DATABASE_URL penuh.
	dbURL := envOr("DATABASE_URL", "dbname=si_cendikia sslmode=disable")
	migrationsDir := envOr("MIGRATIONS_DIR", "migrations")

	ctx := context.Background()
	pool, err := store.Open(ctx, dbURL)
	if err != nil {
		log.Fatalf("koneksi database gagal: %v", err)
	}
	defer pool.Close()

	n, err := store.Migrate(ctx, pool, migrationsDir)
	if err != nil {
		log.Fatalf("migrasi gagal: %v", err)
	}
	if n > 0 {
		log.Printf("migrasi: %d berkas diterapkan", n)
	}

	sessionSecret := envOr("SESSION_SECRET", "")
	if sessionSecret == "" {
		log.Fatal("SESSION_SECRET wajib diisi (set via environment, jangan pernah di-commit)")
	}
	api := httpapi.New(pool, sessionSecret, envOr("SECURE_COOKIE", "true") == "true")

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("si-cendikia mendengarkan di %s", addr)
	log.Fatal(srv.ListenAndServe())
}
