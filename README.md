# SI CENDIKIA

**Sistem Cepat Efektif Non-stop Digital Informasi Kenaikan Gaji Berkala ASN**

Repositori induk proyek **SI CENDIKIA**: penerbitan surat/SK Kenaikan Gaji Berkala (KGB) bagi ASN (PNS dan PPPK) di lingkungan Dinas Pendidikan Kabupaten Grobogan. Piloting tahap awal: khusus guru.

Sistem ini dibangun **lebih sederhana dan lebih ringan** daripada sistem e-KGB live (`kgb.grobogankab.web.id`, source `/var/www/ekgb`), yang kini hanya menjadi referensi domain.

Repositori ini berisi **dokumen desain + konfigurasi agent**. Kode sumber aplikasi dibuat bertahap mengikuti urutan artefak di bawah.

## Kontrak Utama: Urutan Artefak

Semua pekerjaan mengikuti rantai ini, tanpa lompat fase:

```
PRD  >  DATABASE ERD  >  API DOCUMENTATION  >  UI/UX  >  ARCHITECTURE  >  TECH STACK
```

| Fase | Artefak | Status |
|------|---------|--------|
| 1. PRD | `docs/01-PRD/` | ✅ v1.6 FINAL (2026-08-16) |
| 2. DATABASE ERD | `docs/02-DATABASE-ERD/` | ✅ v1.0 FINAL (2026-08-16) |
| 3. API DOCUMENTATION | `docs/03-API-DOCUMENTATION/` | ✅ v1.2 FINAL (2026-08-16) |
| 4. UI/UX | `docs/04-UIUX/` | ✅ v1.0 FINAL (2026-08-16) |
| 5. ARCHITECTURE | `docs/05-ARCHITECTURE/` | ✅ v1.1 FINAL (2026-08-16) |
| 6. TECH STACK | `docs/06-TECH-STACK/` | 🟡 draf v0.2, menunggu konfirmasi owner |
| 7. IMPLEMENTASI | `src/` | ⬜ hanya setelah 1–6 selesai dan disetujui owner |

**Aturan keras:**
- Satu fase hanya boleh dikerjakan kalau artefak fase sebelumnya sudah ada dan disetujui.
- Perubahan PRD memaksa review ulang semua artefak di bawahnya (efek berantai).
- Keputusan penting dicatat di `docs/00-DECISIONS.md`.

## Ringkasan Alur Inti (dari brief owner)

1. ASN (guru PNS/PPPK) login dengan **NIP + password (juga NIP)**.
2. Submit pengajuan KGB + unggah berkas **PDF, maksimal 5MB per file**.
3. Verifikasi berjenjang: **Korwil/SMP/SKB** (sesuai tempat bekerja) → **Dinas**.
4. **TTE oleh pimpinan**.
5. Surat/SK KGB **terbit sebagai PDF**.

## Toolchain Agent (WAJIB dipakai)

Detail prosedur lengkap di `AGENTS.md`:

| Tool | Fungsi |
|------|--------|
| Context7 (MCP) | Dokumentasi library terkini, anti-halusinasi API |
| codebase-memory (MCP) | Knowledge graph codebase (index sekali, lalu query) |
| Memory MCP | Ingatan persisten lintas sesi |
| Spec Kit (CLI) | Planning spec-driven: constitution → specify → plan → tasks → implement |
| Superpowers (plugin) | Brainstorming, TDD, subagent, systematic debugging |
| Ponytail (plugin) | Anti over-engineering; kode minimal yang bekerja |
| Caveman (plugin) | Output ringkas saat iterasi |
| RTK | Kompresi output terminal |

## Keamanan — NON-NEGOTIABLE

1. **Dilarang keras membaca, menyalin, meng-commit, atau mengirim file kredensial.** Daftar lengkap di `.gitignore` bagian FORBIDDEN.
2. Hook `PreToolUse` di `.claude/settings.json` memblokir akses ke path kredensial secara deterministik.
3. Semua nilai rahasia hidup di `.env` (lokal, tidak pernah di-commit) atau credential store sistem.
4. Jangan pernah `git add -A` tanpa memeriksa `git status` dulu.
5. **`/var/www/ekgb/env` dan folder `deploy/` sistem live memuat kredensial produksi — jangan pernah dibaca.**

## Konvensi

- Bahasa dokumen: Bahasa Indonesia (kecuali istilah teknis).
- Commit message: Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`).
- Satu perubahan = satu commit kecil dan fokus.
- Owner: Chamdani (Dinas Pendidikan Grobogan). Preferensi: hasil final teruji, tolak fitur tak perlu, diskusi dulu sebelum eksekusi berat.
