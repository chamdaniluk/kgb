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
| 2026-08-16 | **PRD v1.0 FINAL** — 5 open question diputuskan owner | Keputusan langsung owner | Fase DATABASE ERD dibuka |
| 2026-08-16 | Login ASN: NIP sebagai username DAN NIP sebagai password, berlaku seterusnya | Keputusan owner (risiko diterima, mitigasi rate-limit + hashing) | Tidak ada fitur ganti password ASN di cakupan awal |
| 2026-08-16 | Berkas pengajuan: **1 file PDF saja**, maks 5MB | Keputusan owner (penyederhanaan dari e-KGB yang memakai 2–3 berkas) | Skema cukup satu entitas lampiran per pengajuan |
| 2026-08-16 | TTE memakai sertifikat elektronik **BSrE/BSSN** (kedepannya) | Keputusan owner | Arsitektur harus mengakomodasi integrasi layanan TTE BSrE |
| 2026-08-16 | Master data ASN dari **file BKN milik Dinas Pendidikan** (impor) | Keputusan owner | Perlu fitur impor file BKN oleh admin; tanpa integrasi live ke Dapodik/BKN |
| 2026-08-16 | Format surat mengikuti **template e-KGB** | Keputusan owner (acuan resmi) | Template divariabelkan: nomor, nama, NIP, unit, TMT, gaji lama→baru, tanggal terbit |
| 2026-08-16 | Impor BKN **hanya sekali di awal** (seeding); pemutakhiran data lewat pengajuan perubahan guru + verifikasi Dinas dengan bukti berkas | Keputusan owner | PRD v1.1 menambah F-22..F-27; ERD v0.2 menambah tabel `teacher_changes` |
| 2026-08-16 | **Penyederhanaan**: perubahan data guru HANYA lewat pengajuan KGB, sekalian bukti dukung & kelengkapan berkas; mekanisme perubahan terpisah dihapus | Keputusan owner | PRD v1.2 (F-22..F-24); ERD v0.3 kembali 8 tabel (`teacher_changes` dihapus); master data diperbarui otomatis saat surat terbit |
| 2026-08-16 | Submit ulang setelah ditolak: **ditolak Korwil/unit → ulang dari unit; ditolak Dinas → langsung dari Dinas** | Keputusan owner | Mesin status ERD v0.4 diperbaiki; cukup 1 kolom `status` + audit_logs |
| 2026-08-16 | Nomor surat **otomatis dari template yang dapat diisi Dinas** | Keputusan owner | ERD v0.4 menambah tabel `letter_number_templates` (token {SEQ}/{YEAR}) |
| 2026-08-16 | Data gaji **tidak ada di file BKN**; dihitung dari **tabel skala gaji PNS & PPPK terbaru** (PP 5/2024 & Perpres 11/2024) sesuai masa kerja + pangkat/golongan; PPPK guru golongan **tetap IX** | Keputusan owner | ERD v0.4: kolom `gaji_pokok` dihapus dari `teachers`, diganti `masa_kerja_tahun`; `salary_scales` wajib (asn_type + golongan + masa kerja); PRD v1.3 (F-5, F-9a) |
