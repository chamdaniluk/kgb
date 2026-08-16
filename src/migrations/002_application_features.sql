-- 002_application_features.sql — kolom runtime yang diperlukan workflow TTE.
ALTER TABLE users ADD COLUMN IF NOT EXISTS nik varchar(16);
ALTER TABLE users ADD COLUMN IF NOT EXISTS signature_image_base64 text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS tte_lock_token text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS tte_lock_expires_at timestamptz;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS tte_number text;

CREATE INDEX IF NOT EXISTS idx_users_role_unit ON users (role, unit_id);
CREATE INDEX IF NOT EXISTS idx_submissions_proposed_tmt ON submissions (proposed_tmt);
CREATE UNIQUE INDEX IF NOT EXISTS uq_submission_tte_number
    ON submissions (tte_number) WHERE tte_number IS NOT NULL;

-- Instalasi lama yang sudah menerapkan 001 tetap mendapat template default.
INSERT INTO letter_number_templates (pattern, is_active)
SELECT '800/{SEQ}/4.2/{YEAR}', true
WHERE NOT EXISTS (SELECT 1 FROM letter_number_templates WHERE is_active);

-- Instalasi lama juga mendapat batas satu pengajuan aktif per guru.
CREATE UNIQUE INDEX IF NOT EXISTS uq_submission_one_active_per_teacher
    ON submissions (teacher_id)
    WHERE status <> 'terbit';

-- Constraint ini hanya dokumentasi semantik; validasi PPPK dilakukan pada impor
-- dan service karena golongan lama dapat perlu dikoreksi oleh admin secara audit.
COMMENT ON COLUMN users.nik IS 'NIK pimpinan untuk eSign Kominfo, tidak ditampilkan publik';
COMMENT ON COLUMN users.signature_image_base64 IS 'Spesimen TTD pimpinan untuk mode VISIBLE, bukan credential';
