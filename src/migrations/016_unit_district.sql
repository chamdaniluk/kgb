-- 016_unit_district.sql — kolom kecamatan + jenis verifikasi unit.
-- Jenjang verifikasi: TK/SD lewat Korwil kecamatan masing-masing,
-- SMP/SKB diverifikasi admin unitnya sendiri, Dinas langsung ke Dinas.
-- Kecamatan: 19 nama resmi Grobogan (huruf besar) atau 'DINAS'.
ALTER TABLE units ADD COLUMN IF NOT EXISTS district text;
ALTER TABLE units DROP CONSTRAINT IF EXISTS units_district_check;
ALTER TABLE units ADD CONSTRAINT units_district_check CHECK (
    district IS NULL OR district IN (
        'BRATI','GABUS','GEYER','GODONG','GROBOGAN','GUBUG','KARANGRAYUNG',
        'KEDUNGJATI','KLAMBU','KRADENAN','NGARINGAN','PENAWANGAN','PULOKULON',
        'PURWODADI','TANGGUNGHARJO','TAWANGHARJO','TEGOWANU','TOROH','WIROSARI','DINAS'
    )
);
CREATE INDEX IF NOT EXISTS idx_units_district ON units (district);
COMMENT ON COLUMN units.district IS 'Kecamatan unit (19 nama resmi) atau DINAS untuk unit internal Dinas';

-- Backfill kecamatan Korwil dari namanya (KORWILCAM X / ... Kecamatan X).
UPDATE units SET district = sub.d, updated_at = now() FROM (
    SELECT id,
        CASE
            WHEN upper(name) LIKE '%BRATI%' THEN 'BRATI'
            WHEN upper(name) LIKE '%GABUS%' THEN 'GABUS'
            WHEN upper(name) LIKE '%GEYER%' THEN 'GEYER'
            WHEN upper(name) LIKE '%GODONG%' THEN 'GODONG'
            WHEN upper(name) LIKE '%KARANGRAYUNG%' THEN 'KARANGRAYUNG'
            WHEN upper(name) LIKE '%KEDUNGJATI%' THEN 'KEDUNGJATI'
            WHEN upper(name) LIKE '%KLAMBU%' THEN 'KLAMBU'
            WHEN upper(name) LIKE '%KRADENAN%' THEN 'KRADENAN'
            WHEN upper(name) LIKE '%NGARINGAN%' THEN 'NGARINGAN'
            WHEN upper(name) LIKE '%PENAWANGAN%' THEN 'PENAWANGAN'
            WHEN upper(name) LIKE '%PULOKULON%' THEN 'PULOKULON'
            WHEN upper(name) LIKE '%PURWODADI%' THEN 'PURWODADI'
            WHEN upper(name) LIKE '%TANGGUNGHARJO%' THEN 'TANGGUNGHARJO'
            WHEN upper(name) LIKE '%TAWANGHARJO%' THEN 'TAWANGHARJO'
            WHEN upper(name) LIKE '%TEGOWANU%' THEN 'TEGOWANU'
            WHEN upper(name) LIKE '%TOROH%' THEN 'TOROH'
            WHEN upper(name) LIKE '%WIROSARI%' THEN 'WIROSARI'
            WHEN upper(name) LIKE '%GROBOGAN%' THEN 'GROBOGAN'
            WHEN upper(name) LIKE '%GUBUG%' THEN 'GUBUG'
            ELSE NULL
        END AS d
    FROM units WHERE type = 'korwil'
) AS sub WHERE units.id = sub.id AND sub.d IS NOT NULL AND units.district IS NULL;
