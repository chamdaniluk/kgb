-- 020_pppk_perjanjian_slot.sql — slot berkas "Perjanjian Kerja/Perpanjangan
-- Kontrak" untuk PPPK (ralat tim Dinas, 2026-09-22). Kolom unggah PPPK
-- dipisah: SK Pertama (file_kp) dan Perjanjian Kerja (file_pk).
-- Slot ini pendukung: wajib diunggah pada pengajuan BARU PPPK dan pada
-- kirim-ulang bila belum pernah ada, tetapi tidak diwajibkan di jalur
-- transisi (ACC unit/Dinas, draft, TTE) agar pengajuan berjalan tidak macet.
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS file_pk_name text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS file_pk_path text;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS file_pk_size int;
COMMENT ON COLUMN submissions.file_kp_name IS 'Berkas SK KP terakhir (PNS) / SK Pertama (PPPK)';
COMMENT ON COLUMN submissions.file_pk_name IS 'Berkas Perjanjian Kerja/Perpanjangan Kontrak (PPPK, pendukung)';
