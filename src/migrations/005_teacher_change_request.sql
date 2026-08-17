-- 005_teacher_change_request.sql
-- Data identitas BKN immutable secara manual; perubahan kepegawaian diusulkan
-- pada submission KGB dan baru diterapkan ketika surat terbit.
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS birth_date date;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS pangkat text;
ALTER TABLE teachers ADD COLUMN IF NOT EXISTS jabatan text;
COMMENT ON COLUMN teachers.birth_date IS 'Tanggal lahir dari sumber BKN; tidak dapat diubah melalui usul KGB';
COMMENT ON COLUMN teachers.pangkat IS 'Pangkat tekstual dari sumber BKN';
COMMENT ON COLUMN teachers.jabatan IS 'Jabatan dari sumber BKN';

ALTER TABLE submissions ADD COLUMN IF NOT EXISTS proposed_pangkat_gol text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS proposed_pangkat text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS proposed_jabatan text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS proposed_unit_id int REFERENCES units(id);
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS proposed_effective_date date;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS proposed_change_note text;
COMMENT ON COLUMN submissions.proposed_pangkat_gol IS 'Usulan pangkat/golongan berdasarkan SK pendukung';
COMMENT ON COLUMN submissions.proposed_pangkat IS 'Usulan pangkat berdasarkan SK pendukung';
COMMENT ON COLUMN submissions.proposed_jabatan IS 'Usulan jabatan/promosi berdasarkan SK pendukung';
COMMENT ON COLUMN submissions.proposed_unit_id IS 'Unit tujuan mutasi; dipakai untuk antrean verifikasi';
COMMENT ON COLUMN submissions.proposed_effective_date IS 'Tanggal mulai berlaku perubahan pada SK';
COMMENT ON COLUMN submissions.proposed_change_note IS 'Catatan perubahan kepegawaian dari ASN';
CREATE INDEX IF NOT EXISTS idx_submissions_proposed_unit ON submissions (proposed_unit_id);
CREATE INDEX IF NOT EXISTS idx_teachers_birth_date ON teachers (birth_date);

-- Snapshot data BKN pada saat submit agar histori tidak berubah ketika master
-- ASN berubah setelah surat diterbitkan.
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_name text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_nip varchar(18);
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_birth_date date;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_asn_type text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_pangkat_gol text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_pangkat text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_jabatan text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_masa_kerja_tahun int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_unit_id int;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS snapshot_unit_name text;

-- Jenis unit tujuan divalidasi oleh store sebelum submission disimpan. PostgreSQL
-- CHECK constraint tidak boleh memakai subquery ke tabel units.
