-- 021: golongan ruang PADA SAAT SK KGB terakhir — tidak boleh disamakan
-- dengan golongan efektif kini (bila SK KP lebih baru, golongan sudah naik).
-- Dipakai prefill form kirim-ulang/koreksi dan histori terverifikasi berkas
-- (keputusan owner 2026-10-05, kasus #165: KGB 2024 = III/a meski kini III/b).
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS draft_last_kgb_golongan text;
