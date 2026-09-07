-- 014_mkg_sk.sql
-- Masa kerja sesuai SK pada seksi KP Terakhir dan KGB Terakhir, agar masa
-- kerja terakhir terlihat dari SK yang diterbitkan (keputusan owner 2026-09-03).
-- Blok bawah naskah SK (pejabat/tanggal/nomor SK terakhir + MKG lama/baru)
-- dihapus dari form karena sudah tercakup seksi atas.
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_kp_masa_tahun int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_kp_masa_bulan int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_sk_masa_tahun int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_sk_masa_bulan int;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_kp_masa_tahun int;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_kp_masa_bulan int;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_sk_masa_tahun int;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_sk_masa_bulan int;
COMMENT ON COLUMN submissions.draft_last_kp_masa_tahun IS 'Masa kerja tahun menurut SK KP terakhir';
COMMENT ON COLUMN submissions.draft_last_sk_masa_tahun IS 'Masa kerja tahun menurut SK KGB terakhir';
