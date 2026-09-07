-- 013_kp_terakhir.sql
-- Kenaikan Pangkat (KP) terakhir sebagai acuan golongan terpisah dari KGB.
-- Aturan (keputusan owner 2026-09-03):
--   bila ada KP terbaru, gaji KGB mengacu golongan KP dengan jangka waktu
--   tetap 2 tahun dari KGB terakhir; bila tidak ada KP, gaji memakai
--   golongan yang sama dengan masa kerja dari KGB terakhir.
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_kp_golongan text;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_kp_tmt date;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_kp_nomor text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_kp_golongan text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_kp_tmt date;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_kp_nomor text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_kp_tanggal date;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_kp_pejabat text;
COMMENT ON COLUMN teachers.last_kp_golongan IS 'Golongan KP terakhir (acuan gaji bila lebih baru dari KGB)';
COMMENT ON COLUMN teachers.last_kp_tmt IS 'TMT KP terakhir';
COMMENT ON COLUMN teachers.last_kp_nomor IS 'Nomor SK KP terakhir';
