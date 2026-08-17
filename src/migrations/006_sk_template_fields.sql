-- 006_sk_template_fields.sql -- field SK sesuai contoh PNS/PPPK
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS birth_place text;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS karpeg text;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_sk_pejabat text;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_sk_tanggal date;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_sk_nomor text;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_sk_tmt_berlaku date;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_sk_masa_kerja_tahun int;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS last_sk_masa_kerja_bulan int;

ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_birth_place text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_karpeg text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_last_sk_pejabat text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_last_sk_tanggal date;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_last_sk_nomor text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_last_sk_tmt_berlaku date;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_last_sk_masa_kerja_tahun int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_last_sk_masa_kerja_bulan int;
