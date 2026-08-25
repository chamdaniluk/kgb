-- 010_tmt_awal.sql
-- TMT awal = TMT CPNS (PNS) atau TMT pengangkatan (PPPK). Dipakai sebagai
-- acuan perhitungan masa kerja golongan dan gaji. Tidak tercetak di surat.
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS tmt_awal date;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS tmt_awal date;
COMMENT ON COLUMN teachers.tmt_awal IS 'TMT CPNS/pengangkatan; acuan masa kerja & gaji, bukan cetak surat';
COMMENT ON COLUMN submissions.tmt_awal IS 'Snapshot TMT awal saat usul; dasar hitung masa kerja & gaji';
