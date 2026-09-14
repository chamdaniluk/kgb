-- 019_unit_kd_unker.sql — identitas unit stabil dari kode unit SIPP ASN.
--
-- Masalah (temuan 2026-09-14): units.code lama diturunkan dari NAMA saja
-- ("SDN-1-KARANGANYAR"), padahal beberapa sekolah bernama sama berdiri di
-- kecamatan berbeda (SDN 1 Karanganyar ada di Purwodadi, Geyer, Karangrayung).
-- Karena code UNIQUE, hanya satu yang tersimpan dan 10 unit SD hilang; kecamatan
-- pun ditebak dari nama sehingga SDN 1-4 Tanggungharjo (desa di Kec. Grobogan)
-- salah masuk Kec. Tanggungharjo.
--
-- kd_unker adalah kode unit kerja SIPP ASN yang unik 1:1 (1.503 kode untuk
-- 1.472 nama) dan memuat kode kecamatan (03.NN). Kolom ini menjadi identitas
-- kanonik unit: dipakai sebagai akhiran kode sekolah dan sumber kecamatan.
ALTER TABLE units ADD COLUMN IF NOT EXISTS kd_unker text;

CREATE UNIQUE INDEX IF NOT EXISTS units_kd_unker_key ON units (kd_unker) WHERE kd_unker IS NOT NULL;

COMMENT ON COLUMN units.kd_unker IS 'Kode unit kerja SIPP ASN (kd_unker) bersifat unik; identitas kanonik unit. NULL untuk unit yang tidak berasal dari sinkron SIPP ASN (mis. impor BKN manual).';
