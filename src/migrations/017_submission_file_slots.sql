-- 017_submission_file_slots.sql — slot berkas pendukung per jenis ASN.
-- PNS: SK KP terakhir (wajib) + KGB terakhir (wajib).
-- PPPK: SK terakhir (wajib) + KGB terakhir (opsional) + SKP 2 tahun (wajib).
-- Kolom file_name/path/size lama dipertahankan sebagai berkas utama
-- (kompatibilitas data lama); setiap slot maks 5MB (validasi aplikasi).
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS file_kp_name text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS file_kp_path text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS file_kp_size int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS file_kgb_name text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS file_kgb_path text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS file_kgb_size int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS file_skp_name text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS file_skp_path text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS file_skp_size int;
COMMENT ON COLUMN submissions.file_kp_name IS 'Berkas SK KP terakhir (PNS wajib, PPPK = SK terakhir)';
COMMENT ON COLUMN submissions.file_kgb_name IS 'Berkas KGB terakhir (PNS wajib, PPPK opsional)';
COMMENT ON COLUMN submissions.file_skp_name IS 'Berkas SKP 2 tahun (PPPK wajib)';
