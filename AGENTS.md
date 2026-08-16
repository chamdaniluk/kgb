# AGENTS.md — Proyek KGB

Instruksi wajib untuk SEMUA AI coding agent (Claude Code, Codex, Hermes, dll.) yang bekerja di repositori ini. Baca sampai habis sebelum melakukan apa pun.

## ATURAN #0 — KEAMANAN KREDENSIAL (NON-NEGOTIABLE)

1. **JANGAN PERNAH** membaca, membuka, menyalin, menampilkan, meng-commit, atau mengirim isi file berikut (dan file sejenis):
   - `.env`, `.env.*`, `*.env`, `env`, `env.local`, `env.production`
   - `~/.hermes/.env`, `~/.hermes/auth.json`, `~/.hermes/config.yaml`
   - `~/.ssh/**`, `~/.gnupg/**`, `~/.aws/credentials`, `~/.config/gh/hosts.yml`
   - `~/.claude.json`, `~/.npmrc`, `*.pem`, `*.key`, `*.p12`, `*.pfx`, `*.keystore`
   - `id_rsa*`, `id_ed25519*`, `*token*`, `*secret*`, `*credential*`, `*password*`
   - `secrets/`, `credentials/`, `.credentials/` di mana pun
   - **`/var/www/ekgb/env`, `/var/www/ekgb/deploy/**`** (kredensial sistem live)
2. Jika tugas terasa membutuhkan isi file kredensial: **BERHENTI** dan tanyakan ke owner. Jangan mencari celah (jangan baca via log, history, backup, proses, atau `strings`).
3. Jangan pernah menulis rahasia (API key, password, token, DSN database) ke file yang akan di-commit. Gunakan `.env.example` dengan placeholder kosong.
4. Sebelum commit: selalu jalankan `git status` dan pastikan tidak ada file terlarang yang ikut ter-stage. Jangan pernah `git add -A` / `git add .` buta.
5. Jangan pernah mematikan, melemahkan, atau mengabaikan hook keamanan di `.claude/settings.json`.
6. Sistem live `/var/www/ekgb` hanya boleh **dibaca sebagai referensi struktur** (Controllers, Views, migrations, README, DEPLOYMENT.md). Jangan memodifikasi apa pun di sana dari proyek ini.

## ATURAN #1 — BACA DAN GUNAKAN SKILLS/PLUGINS INI

Toolchain berikut terpasang di mesin ini. WAJIB digunakan sesuai fungsinya — jangan kerja manual kalau tool-nya ada:

### MCP Servers (terpasang global, scope user)
- **Context7** — sumber kebenaran dokumentasi library. Setiap menulis kode dengan library/framework, ambil docs versi terkini via Context7 dulu. Jangan mengandalkan ingatan (hindari API usang/halusinasi).
- **codebase-memory** — knowledge graph codebase. Untuk pertanyaan struktural ("siapa memanggil X?", "di mana Y didefinisikan?") pakai `search_graph` / `trace_call_path`, BUKAN grep file-per-file. Index sekali dengan `index_repository` di awal kerja pada repo.
- **Memory MCP** (@modelcontextprotocol/server-memory) — simpan keputusan penting dan preferensi owner agar bertahan lintas sesi.

### Plugins Claude Code (terpasang, scope user)
- **Superpowers** (v6.x) — workflow inti:
  - `/superpowers:brainstorming` sebelum mendesain apa pun
  - `/superpowers:writing-plans` untuk rencana implementasi
  - `/superpowers:systematic-debugging` untuk bug sulit (jangan nebak-nebak)
  - `/superpowers:test-driven-development` untuk implementasi
  - `/superpowers:verification-before-completion` sebelum menyatakan selesai
- **Ponytail** (v4.9) — anti over-engineering. Sebelum menulis kode, panjat tangga ini: (1) apakah ini perlu ada? (2) sudah ada di codebase? (3) stdlib bisa? (4) bisa satu baris? Baru tulis kode MINIMAL yang bekerja. `/ponytail-review` sebelum commit. JANGAN pernah menambah fitur yang tidak diminta.
- **Caveman** — mode output ringkas. Aktifkan saat iterasi cepat. Jawaban: langsung ke inti, tanpa filler. Kode, perintah, dan pesan error tetap ditulis persis apa adanya.

### CLI Tools (terpasang global)
- **Spec Kit** (`specify`, ~/.local/bin) — perencanaan spec-driven:
  1. `/speckit.constitution` → prinsip proyek
  2. `/speckit.specify` → APA & MENGAPA (requirements)
  3. `/speckit.plan` → BAGAIMANA (tech stack & arsitektur)
  4. `/speckit.tasks` → daftar tugas
  5. `/speckit.implement` → eksekusi
- **RTK** (`rtk`, ~/.local/bin) — hook PreToolUse aktif; output perintah Bash yang berisik otomatis dikompresi. Untuk eksplorasi file, prefer `rtk read` / `rtk grep` / `rtk find` kalau tersedia.

## ATURAN #2 — KONTRAK URUTAN ARTEFAK

Proyek ini bergerak ketat mengikuti rantai:

```
PRD  >  DATABASE ERD  >  API DOCUMENTATION  >  UI/UX  >  ARCHITECTURE  >  TECH STACK
```

- Satu fase hanya dikerjakan jika artefak fase sebelumnya **sudah ada dan disetujui owner**.
- Artefak ditulis di `docs/01-PRD/` … `docs/06-TECH-STACK/` (lihat README).
- Perubahan pada fase di atas memaksa review ulang semua fase di bawahnya.
- Fase IMPLEMENTASI hanya dibuka setelah keenam artefak selesai dan disetujui.

## ATURAN #3 — KONTEKS DOMAIN KGB

- KGB = **Kenaikan Gaji Berkala** untuk ASN/guru di lingkungan Dinas Pendidikan Kabupaten Grobogan.
- Sistem live: `kgb.grobogankab.web.id`, source `/var/www/ekgb` (CI 4.7 + Shield + PostgreSQL 16 + Dompdf). Alur: draf guru + 2 dokumen wajib → verifikasi unit (Korwil/SMP/SKB) → pemeriksaan Dinas → persetujuan konsep → penerbitan surat final. Semua transisi diaudit.
- Data guru berasal dari sumber resmi: **Dapodik/PTK** (Kemendikdasmen, API publik api.data.belajar.id). Verifikasi kemutakhiran data sebelum mengutip. Jangan pernah mengarang data guru.
- Keputusan tentang hubungan proyek baru ini dengan sistem live (rewrite / pengganti / pendamping) ada di `docs/00-DECISIONS.md` — baca dulu sebelum mengusulkan arsitektur.

## ATURAN #4 — CARA KERJA

1. **Diskusi dulu, eksekusi kemudian.** Untuk perubahan besar (struktur, dependency baru, arsitektur), jelaskan pro/kontra dan tunggu persetujuan. Jangan pernah install/ubah sistem di luar yang diminta.
2. **Bahasa Indonesia** untuk semua dokumen dan komunikasi, humanized academic, tanpa excessive em dash. Istilah teknis boleh Inggris.
3. **Hasil akhir harus teruji**: jalankan build/test/E2E dan tunjukkan output nyata. Jangan laporkan "selesai" tanpa bukti eksekusi. Jangan pernah mengarang output.
4. **Conventional Commits**, commit kecil dan fokus.
5. Jika tidak yakin: **tanya**, jangan menebak.

## Referensi

- `README.md` — peta proyek & status artefak
- `.gitignore` — daftar lengkap file terlarang (FORBIDDEN)
- `.claude/settings.json` — hook keamanan PreToolUse
- `/var/www/ekgb/README.md` + `DEPLOYMENT.md` — referensi sistem live (read-only)
