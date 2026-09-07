-- 011_sync_sippasn.sql
-- Riwayat sinkronisasi ASN dari SIPP ASN. Tabel bkn_imports dipakai ulang
-- (file_name 'sippasn:...'); kolom asal_data menandai sumber sinkron secara
-- eksplisit agar mudah difilter dari impor file BKN manual.
ALTER TABLE bkn_imports ADD COLUMN IF NOT EXISTS asal_data text NOT NULL DEFAULT 'bkn_file';
CREATE INDEX IF NOT EXISTS idx_bkn_imports_asal ON bkn_imports (asal_data);
COMMENT ON COLUMN bkn_imports.asal_data IS 'Sumber impor: bkn_file (unggah manual) atau sippasn (sinkron API)';
