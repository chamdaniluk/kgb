-- 004_bkn_derived_fields.sql — asal perhitungan masa kerja dari file BKN.
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS masa_kerja_source text;
COMMENT ON COLUMN teachers.masa_kerja_source IS 'Asal masa kerja: masa_kerja_bkn, tmt_cpns, tmt_gol, atau belum_tersedia';

-- File BKN berisi SD/TK dan satu unit administratif Dinas.
ALTER TABLE units DROP CONSTRAINT IF EXISTS units_type_check;
ALTER TABLE units ADD CONSTRAINT units_type_check CHECK (type IN ('korwil', 'sd', 'tk', 'smp', 'skb', 'dinas'));
CREATE INDEX IF NOT EXISTS idx_teachers_masa_source ON teachers (masa_kerja_source);

ALTER TABLE submissions ADD COLUMN IF NOT EXISTS proposed_masa_kerja_tahun int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS proposed_tmt_kgb_last date;
COMMENT ON COLUMN submissions.proposed_masa_kerja_tahun IS 'Snapshot masa kerja yang dipakai menghitung KGB; dapat dilengkapi saat submit';
COMMENT ON COLUMN submissions.proposed_tmt_kgb_last IS 'Snapshot TMT KGB terakhir; dapat dilengkapi saat submit';
