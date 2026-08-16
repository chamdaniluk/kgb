package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		pctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(pctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "db-down"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("si-cendikia mendengarkan di %s", addr)
	log.Fatal(srv.ListenAndServe())
}
