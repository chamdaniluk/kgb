# TECH STACK — SI CENDIKIA v0.2 (DRAF)

> Fase 6 dari 6 (terakhir sebelum implementasi). Sumber: PRD v1.6, ERD v1.0, API v1.2, UI/UX v1.0, ARCHITECTURE v1.1. Menunggu konfirmasi owner.

## 1. Prinsip Pemilihan

1. **Seringan mungkin** (NF-1, NF-2): satu binary, satu proses, tanpa runtime ganda, tanpa toolchain frontend terpisah.
2. **Konsisten dengan pengalaman tim**: proyek CBT eSchool yang sudah berjalan memakai Go; lanjut Go untuk menekan kurva belajar dan biaya perawatan.
3. **Tidak menambah mesin baru**: PostgreSQL sudah dipakai sistem e-KGB live; Nginx + systemd sudah ada di VPS.
4. **Versi library diverifikasi via Context7** saat implementasi (aturan AGENTS.md) — daftar di bawah adalah pilihan, bukan kunci versi.

## 2. Keputusan Stack (ringkasan)

| Lapisan | Pilihan | Alasan |
|---|---|---|
| **Bahasa** | Go 1.22+ | Satu binary statis, memori kecil, sesuai pengalaman CBT eSchool |
| **HTTP server** | Go stdlib `net/http` (routing pola 1.22+) | Tanpa framework; cukup untuk ~25 endpoint; middleware RBAC/rate-limit ditulis manual |
| **Frontend** | Server-rendered `html/template` + CSS minimal + vanilla JS | Tanpa build step Node, tanpa SPA; grafik pakai CSS bar (UI/UX §4.0); ponsel tetap responsif |
| **Basis data** | PostgreSQL 16 | Sesuai referensi e-KGB; ERD ditulis untuk PostgreSQL |
| **Akses DB** | `pgx` (driver) + SQL parameterized langsung | Tanpa ORM berat; 9 tabel sederhana; menghindari masalah yang pernah ditemui di proyek Go sebelumnya (mis. COALESCE lintas tabel) |
| **PDF surat** | `wkhtmltopdf` (binary statis) | Render konsep surat dari template e-KGB; hasil PDF dikirim ke eSign |
| **TTE** | REST API **eSign Kominfo** (Esign Client Service 2.2.2), HTTP client stdlib | Integrasi nyata; dokumentasi Postman di `local/esign-kominfo/`; env dev `esign-dev.layanan.go.id` |
| **Excel impor** | `github.com/xuri/excelize/v2` | Baca file BKN & daftar akun petugas (.xlsx); library Go standar |
| **Auth** | Session cookie HMAC-signed (httpOnly, Secure, SameSite) + token CSRF | Sederhana, sesuai arsitektur monolitik; password bcrypt (`golang.org/x/crypto/bcrypt`) |
| **Rate limit** | In-memory per username+IP (sliding window) | Pola sama dengan e-KGB; tanpa Redis |
| **Deploy** | systemd unit + Nginx (reverse proxy, SSL) + binary di `/opt/si-cendikia` | Pola yang sudah teruji di CBT eSchool |
| **Backup** | Cron `pg_dump` harian + rsync direktori PDF | ERD & ARCHITECTURE §8 |
| **Testing** | Unit test Go + `httptest`; E2E Playwright headless (pola `test_ujian.py`) | Konsisten workflow owner |

## 3. Struktur Repo (usulan)

```
src/
  cmd/server/main.go          # entry point
  internal/
    httpapi/                  # routing, middleware (auth, RBAC, rate limit, CSRF)
    auth/                     # login, sesi, hash password
    submission/               # pengajuan, verifikasi, mesin status
    letter/                   # template surat, nomor, PDF, TTE
    admin/                    # impor BKN, impor akun, unit, pengguna, skala gaji
    public/                   # stats agregat
    store/                    # pgx queries per entitas (9 tabel)
  web/
    templates/                # html/template (semua halaman)
    static/                   # CSS, JS minimal
  data/
    seeds/                    # skala gaji, template nomor (SQL)
    (bkn & akun tidak di-commit — dari local/)
  migrations/                 # SQL migrasi skema
```

## 4. Keputusan yang Perlu Konfirmasi Owner

### 4.1 Mesin PDF surat (perlu konfirmasi)

Template surat e-KGB adalah HTML+CSS (kop, @page margin, DejaVu Serif/Sans). Tiga opsi:

| Opsi | Kelebihan | Kekurangan |
|---|---|---|
| **A. `wkhtmltopdf` binary statis** (rekomendasi) | Satu file tanpa runtime; hasil mirip template e-KGB; ~100MB RAM saat dipanggil | Proyek sudah jarang di-upstream (WebKit lama); CSS modern terbatas — template kita sederhana, aman |
| **B. WeasyPrint (Python)** | CSS paged-media paling lengkap; aktif dipelihara | Butuh runtime Python ~150-250MB; dua runtime di server |
| **C. Headless Chromium (chromedp)** | Fidelitas tertinggi | Paling berat (~300MB+); VPS 1.9GB berisiko saat Postgres ikut jalan |

Rekomendasi: **A untuk pilot** (paling ringan, template kompatibel). Cadangan: B jika format surat berkembang.

### 4.2 Frontend (perlu konfirmasi)

- **Rekomendasi: server-rendered + vanilla JS** — tidak ada build step, satu binary, sesuai UI/UX (tanpa framework besar). Grafik statistik pakai CSS bar sederhana.
- Alternatif: React SPA (konsisten CBT eSchool) — biayanya: toolchain Node, build/deploy terpisah, memori lebih besar. Tidak disarankan untuk aplikasi sesederhana ini (Ponytail: tulis yang minimal).

### 4.3 Hal lain yang saya putuskan (bisa diubah)

- Auth sesi cookie (bukan JWT) — sesederhana mungkin untuk monolit; JWT tidak memberi keuntungan di sini.
- `pgx` tanpa ORM — 9 tabel tidak butuh ORM.
- Tidak ada migrasi otomatis versi berat; pakai folder `migrations/` SQL polos yang dijalankan saat deploy.

## 5. Dependensi Minimal (perkiraan)

| Kebutuhan | Library |
|---|---|
| HTTP/routing | stdlib |
| DB | `github.com/jackc/pgx/v5` |
| Hash password | `golang.org/x/crypto/bcrypt` |
| Excel | `github.com/xuri/excelize/v2` |
| PDF render | binary eksternal `wkhtmltopdf` |
| TTE eSign | REST API eSign Kominfo — HTTP client stdlib + Basic Auth; `id_dokumen` disimpan di `letters.tte_receipt_id` |

Total: **1 bahasa, 1 database, 1 web server, 4 library Go, 1 binary PDF**. Ini sesuai semangat "cepat, efektif, ringan".

## 6. Risiko Stack & Mitigasi

| Risiko | Mitigasi |
|---|---|
| `wkhtmltopdf` tidak lagi di-maintain | Template surat sederhana & stabil; cadangan WeasyPrint terdokumentasi; uji regresi visual di E2E |
| eSign Kominfo dev environment berubah / mati | Kredensial & koleksi ada di `local/`; verifikasi `GET /api/user/status/{nik}` sebelum sign; fallback: mode simulasi lokal yang jelas-jelas ditandai (hanya untuk uji alur, bukan produksi) |
| Go stdlib routing kurang familier | Pola routing 1.22+ sederhana; dokumentasi via Context7 saat implementasi |
| Tanpa React, fitur interaktif terbatas | Semua interaksi yang dibutuhkan (form, konfirmasi, fetch stats, pratinjau PDF) cukup dengan vanilla JS |
| Password plain petugas (keputusan owner) | Tetap di-hash saat simpan; rate limit + audit login |

## 7. Versi Lingkungan

- Go 1.22+ (VPS: perlu `apt install golang` atau download tarball — diputuskan saat implementasi, tanpa mengubah sistem tanpa izin)
- PostgreSQL 16 (sudah ada di VPS dari e-KGB)
- Nginx + SSL (sudah ada)
- Node (sudah ada, hanya untuk Playwright E2E, bukan runtime aplikasi)
