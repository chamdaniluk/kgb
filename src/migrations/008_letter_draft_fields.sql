-- 008_letter_draft_fields.sql
-- Field naskah SK yang diisi ASN pada usulan, plus NIP/jabatan pimpinan
-- (terpisah dari NIK eSign).

ALTER TABLE teachers ADD COLUMN IF NOT EXISTS contract_start date;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS contract_end date;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS contract_extension date;

ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_birth_place text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_birth_date date;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_karpeg text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_pangkat text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_jabatan text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_sk_pejabat text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_sk_tanggal date;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_sk_nomor text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_sk_tmt date;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_mkg_lama_tahun int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_mkg_lama_bulan int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_mkg_baru_tahun int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_mkg_baru_bulan int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_masa_perjanjian text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_perpanjangan_kontrak date;

ALTER TABLE users ADD COLUMN IF NOT EXISTS employee_number varchar(18);
ALTER TABLE users ADD COLUMN IF NOT EXISTS job_title text;

COMMENT ON COLUMN users.employee_number IS 'NIP pimpinan yang tercetak di blok TTD; bukan NIK eSign';
COMMENT ON COLUMN users.job_title IS 'Jabatan pimpinan yang tercetak di blok TTD PPPK';
COMMENT ON COLUMN submissions.draft_karpeg IS 'Nilai naskah yang dikirim ASN; sumber cetak SK';
