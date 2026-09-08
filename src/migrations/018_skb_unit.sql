-- 018_skb_unit.sql — SKB memiliki unit verifikasi sendiri (bukan lewat Korwil
-- maupun antrean Dinas). Lokasi SKB: Kecamatan Purwodadi (keputusan owner
-- 2026-09-08: "SKB tidak menggunakan korwil sebagai unit tapi memiliki unit
-- sendiri, cuma ikut lokasi di Kec Purwodadi").
--
-- Sebelumnya unit SPNF/SKB terpetakan bertipe 'dinas' karena namanya
-- mengandung "Dinas Pendidikan" (sippASNUnitType), sehingga usulan pegawai
-- SKB langsung masuk antrean Dinas tanpa verifikasi unit sendiri.
UPDATE units
SET type = 'skb',
    district = 'PURWODADI',
    parent_id = NULL,
    updated_at = now()
WHERE type <> 'skb'
  AND (
    name ILIKE '%sanggar kegiatan belajar%'
    OR name ILIKE '%SKB%'
    OR name ILIKE '%SPNF%'
  );

COMMENT ON COLUMN units.type IS 'korwil, sd, tk, smp, skb, dinas; skb = SPNF Sanggar Kegiatan Belajar dengan verifikasi unit sendiri (lokasi Kec. Purwodadi)';
