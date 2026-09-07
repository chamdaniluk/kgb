-- 015_app_settings.sql
-- Pengaturan aplikasi umum (kunci-nilai) agar admin dapat mengubah perilaku
-- tanpa menyentuh env di server. Keputusan owner 2026-09-04: saklar banner
-- uji coba internal di beranda (DB menang, env TRIAL_UNTIL sebagai cadangan).
CREATE TABLE IF NOT EXISTS app_settings (
  key text PRIMARY KEY,
  value text NOT NULL,
  updated_by integer REFERENCES users(id),
  updated_at timestamp with time zone NOT NULL DEFAULT now()
);
COMMENT ON TABLE app_settings IS 'Pengaturan aplikasi (kunci-nilai) yang dapat diubah admin lewat dasbor';
-- Default: banner uji coba tampil. Tanggal tidak di-seed agar fallback ke
-- env TRIAL_UNTIL lalu default 7 hari tetap berjalan seperti sebelumnya.
INSERT INTO app_settings (key, value) VALUES ('trial_notice_enabled', 'true')
ON CONFLICT (key) DO NOTHING;
