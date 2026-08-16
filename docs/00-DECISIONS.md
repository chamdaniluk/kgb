# Log Keputusan (Decision Log) — SI CENDIKIA

Catatan keputusan penting. Format: tanggal, keputusan, alasan, konsekuensi.

| Tanggal | Keputusan | Alasan | Konsekuensi |
|---------|-----------|--------|-------------|
| 2026-08-16 | Proyek baru bernama **SI CENDIKIA** (Sistem Cepat Efektif Non-stop Digital Informasi Kenaikan Gaji Berkala ASN), terpisah dari sistem live e-KGB | Owner ingin sistem yang lebih sederhana dan lebih ringan; hasil akhir = terbitnya surat/SK KGB | Sistem live `/var/www/ekgb` hanya referensi domain, read-only |
| 2026-08-16 | Cakupan pengguna: ASN (PNS dan PPPK) lingkungan Dinas Pendidikan; **piloting tahap awal khusus guru** | Permintaan owner | Master data & role awal fokus guru; struktur tetap memungkinkan perluasan ke tenaga kependidikan lain |
| 2026-08-16 | Login ASN: **NIP sebagai username, NIP sebagai password** | Agar mudah bagi pengguna | Dicatat sebagai risiko keamanan di PRD; menunggu keputusan mitigasi (lihat open question PRD) |
| 2026-08-16 | Berkas unggahan: **PDF, maksimal 5MB per file** | Permintaan owner | Validasi tipe & ukuran di sisi server dan klien |
| 2026-08-16 | Alur verifikasi: **berjenjang** — unit (Korwil/SMP/SKB sesuai tempat bekerja) → Dinas → **TTE pimpinan** → surat terbit (PDF) | Permintaan owner | Setiap transisi status diaudit |
| 2026-08-16 | Toolchain agent: Context7, codebase-memory MCP, Memory MCP, Spec Kit, Superpowers, Ponytail, Caveman, RTK | Konsistensi dengan proyek CMS Sekolahku | Semua agent wajib memakai tool ini (lihat AGENTS.md) |
| 2026-08-16 | Keamanan kredensial via hook PreToolUse + AGENTS.md + .gitignore, termasuk larangan baca `/var/www/ekgb/env` dan `deploy/` | Perlapisan: deterministik + imbauan + pencegahan commit | Agent tidak bisa membaca file kredensial apa pun |
