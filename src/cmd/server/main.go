package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"sicendikia/internal/auth"
	"sicendikia/internal/esign"
	"sicendikia/internal/files"
	"sicendikia/internal/httpapi"
	"sicendikia/internal/pdf"
	"sicendikia/internal/store"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	logger := slog.Default()
	addr := envOr("ADDR", ":8080")
	dbURL := envOr("DATABASE_URL", "dbname=si_cendikia sslmode=disable")
	migrationsDir := envOr("MIGRATIONS_DIR", "migrations")
	fileRoot := envOr("FILE_ROOT", "/var/lib/si-cendikia/files")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := store.Open(ctx, dbURL)
	if err != nil {
		logger.Error("koneksi database gagal", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if n, err := store.Migrate(ctx, pool, migrationsDir); err != nil {
		logger.Error("migrasi gagal", "err", err)
		os.Exit(1)
	} else if n > 0 {
		logger.Info("migrasi diterapkan", "count", n)
	}
	if err := os.MkdirAll(filepath.Dir(fileRoot), 0o750); err != nil {
		logger.Error("folder file gagal disiapkan", "err", err)
		os.Exit(1)
	}
	fileStore, err := files.New(fileRoot)
	if err != nil {
		logger.Error("storage file gagal disiapkan", "err", err)
		os.Exit(1)
	}
	secret := envOr("SESSION_SECRET", "")
	if len(secret) < 32 {
		logger.Error("SESSION_SECRET wajib diisi minimal 32 karakter")
		os.Exit(1)
	}
	bootstrapUsername := os.Getenv("BOOTSTRAP_ADMIN_USERNAME")
	bootstrapPassword := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if bootstrapUsername != "" || bootstrapPassword != "" {
		if bootstrapUsername == "" || bootstrapPassword == "" {
			logger.Error("BOOTSTRAP_ADMIN_USERNAME dan BOOTSTRAP_ADMIN_PASSWORD harus diisi berpasangan")
			os.Exit(1)
		}
		hash, err := auth.HashPassword(bootstrapPassword)
		if err != nil {
			logger.Error("password bootstrap gagal diproses", "err", err)
			os.Exit(1)
		}
		if err := store.EnsureBootstrapAdmin(ctx, pool, bootstrapUsername, hash, os.Getenv("BOOTSTRAP_ADMIN_NAME")); err != nil {
			logger.Error("bootstrap admin gagal", "err", err)
			os.Exit(1)
		}
		logger.Info("bootstrap admin siap", "username", bootstrapUsername)
	}
	var signer *esign.Client
	if baseURL := envOr("ESIGN_BASE_URL", ""); baseURL != "" {
		signer = &esign.Client{BaseURL: baseURL, Username: os.Getenv("ESIGN_USERNAME"), Password: os.Getenv("ESIGN_PASSWORD")}
	}
	api := httpapi.NewWithDependencies(pool, secret, envOr("SECURE_COOKIE", "true") == "true", httpapi.Dependencies{
		Files: fileStore, Renderer: pdf.NewRenderer(), Signer: signer, SIPPASN: httpapi.SIPPASNSnapshotDefault(),
	})
	// Sinkron SIPPASN tiap malam (VPS): SIPPASN adalah sumber utama data induk.
	if envOr("SIPPASN_NIGHTLY_SYNC", "true") == "true" {
		interval := 24 * time.Hour
		if v := os.Getenv("SIPPASN_NIGHTLY_INTERVAL"); v != "" {
			if d, err := time.ParseDuration(v); err == nil && d >= time.Minute {
				interval = d
			}
		}
		var actorID int64
		if v := os.Getenv("SIPPASN_NIGHTLY_ACTOR_ID"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				actorID = n
			}
		}
		if actorID == 0 {
			actorID = store.SIPPASNSyncSystemActor(pool)
		}
		go func() {
			_ = httpapi.RunNightlySIPPASNSync(context.Background(), pool, api.SIPPASN, interval, actorID)
		}()
		logger.Info("sinkron SIPPASN malam aktif", "interval", interval.String())
	}
	srv := &http.Server{Addr: addr, Handler: api.Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 120 * time.Second}
	logger.Info("si-cendikia mendengarkan", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server berhenti", "err", err)
		os.Exit(1)
	}
}
