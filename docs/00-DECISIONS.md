# Log Keputusan (Decision Log) — Proyek KGB

Catatan keputusan penting. Format: tanggal, keputusan, alasan, konsekuensi.

| Tanggal | Keputusan | Alasan | Konsekuensi |
|---------|-----------|--------|-------------|
| 2026-08-16 | Proyek baru KGB dibuat terpisah dari sistem live `/var/www/ekgb` | Sistem live berjalan dan bukan git repo; proyek baru mulai dari kontrak artefak (PRD > ERD > API > UI/UX > ARCH > STACK) | Source live hanya referensi read-only |
| 2026-08-16 | Toolchain agent: Context7, codebase-memory MCP, Memory MCP, Spec Kit, Superpowers, Ponytail, Caveman, RTK | Konsistensi dengan proyek CMS Sekolahku; efisiensi token + disiplin planning + anti over-engineering | Semua agent wajib memakai tool ini (lihat AGENTS.md) |
| 2026-08-16 | Keamanan kredensial via hook PreToolUse + AGENTS.md + .gitignore | Berlapis: deterministik (hook) + imbauan (AGENTS.md) + pencegahan commit (.gitignore); termasuk larangan baca `/var/www/ekgb/env` dan `deploy/` | Agent tidak bisa membaca file kredensial apa pun |
| 2026-08-16 | ⬜ BELUM DIPUTUSKAN: hubungan proyek baru dengan sistem live — rewrite penuh, pengganti bertahap, atau pendamping/modul tambahan? | Menunggu keputusan owner | PRD tidak boleh ditulis sebelum ini jelas |
