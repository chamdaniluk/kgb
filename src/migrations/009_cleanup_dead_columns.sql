-- 009_cleanup_dead_columns.sql
-- Buang kolom mati pada teachers: contract_start/end/extension tidak pernah
-- dibaca atau ditulis kode aplikasi (Temuan #2 audit). Masa perjanjian PPPK
-- disimpan di submissions.draft_masa_perjanjian & draft_perpanjangan_kontrak.
ALTER TABLE teachers DROP COLUMN IF EXISTS contract_start;
ALTER TABLE teachers DROP COLUMN IF EXISTS contract_end;
ALTER TABLE teachers DROP COLUMN IF EXISTS contract_extension;
