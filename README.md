# KGB — Proyek Baru

Repositori induk (umbrella repo) proyek baru tentang **KGB (Kenaikan Gaji Berkala)** untuk lingkungan Dinas Pendidikan Kabupaten Grobogan.

Repositori ini berisi **dokumen desain + konfigurasi agent**. Kode sumber aplikasi dibuat bertahap mengikuti urutan artefak di bawah.

## Konteks Domain

Sistem e-KGB sudah berjalan (live) di `kgb.grobogankab.web.id` dengan source di `/var/www/ekgb` (CodeIgniter 4.7 + Shield + PostgreSQL 16). Alur utamanya: guru membuat draf + unggah dua dokumen wajib → verifikasi unit (Korwil/SMP/SKB) → pemeriksaan Dinas → persetujuan konsep → penerbitan surat final (PDF, Dompdf). Semua perubahan status diaudit.

Source live itu adalah **bahan referensi domain**, bukan bagian repo ini (jangan dimodifikasi dari sini; ia bukan git repo).

## Kontrak Utama: Urutan Artefak

Semua pekerjaan mengikuti rantai ini, tanpa lompat fase:

```
PRD  >  DATABASE ERD  >  API DOCUMENTATION  >  UI/UX  >  ARCHITECTURE  >  TECH STACK
```

| Fase | Artefak | Status |
|------|---------|--------|
| 1. PRD | `docs/01-PRD/` | ⬜ belum mulai |
| 2. DATABASE ERD | `docs/02-DATABASE-ERD/` | ⬜ belum mulai |
| 3. API DOCUMENTATION | `docs/03-API-DOCUMENTATION/` | ⬜ belum mulai |
| 4. UI/UX | `docs/04-UIUX/` | ⬜ belum mulai |
| 5. ARCHITECTURE | `docs/05-ARCHITECTURE/` | ⬜ belum mulai |
| 6. TECH STACK | `docs/06-TECH-STACK/` | ⬜ belum mulai |
| 7. IMPLEMENTASI | `src/` | ⬜ hanya setelah 1–6 selesai dan disetujui owner |

**Aturan keras:**
- Satu fase hanya boleh dikerjakan kalau artefak fase sebelumnya sudah ada dan disetujui.
- Perubahan PRD memaksa review ulang semua artefak di bawahnya (efek berantai).
- Keputusan penting dicatat di `docs/00-DECISIONS.md`.

## Toolchain Agent (WAJIB dipakai)

Sama seperti proyek CMS Sekolahku — detail prosedur lengkap di `AGENTS.md`:

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
