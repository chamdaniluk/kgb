-- 012_kategori_pegawai.sql
-- Perluasan cakupan ke seluruh pegawai Dinas Pendidikan (guru + non-guru).
-- SIPPASN tidak membedakan jenis ASN secara eksplisit; kategori diturunkan
-- saat sinkron dari pola NIP (segmen bulan 01-12 = PNS, 21-22 = PPPK).
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS kategori text NOT NULL DEFAULT 'guru'
    CHECK (kategori IN ('guru', 'non_guru'));
CREATE INDEX IF NOT EXISTS idx_teachers_kategori ON teachers (kategori);
COMMENT ON COLUMN teachers.kategori IS 'guru (fungsional guru) atau non_guru (pelaksana, pengawas, penilik, pamong, struktural Disdik)';
