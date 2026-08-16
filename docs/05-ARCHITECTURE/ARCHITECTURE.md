# ARCHITECTURE — SI CENDIKIA v0.1 (DRAF)

> Fase 5 dari 6. Sumber: PRD v1.4, ERD v1.0 FINAL, API v1.1, UI/UX v1.0. Menunggu review owner.

## 1. Tujuan dan Prinsip Arsitektur

Arsitektur SI CENDIKIA dirancang untuk satu tujuan: **sederhana, ringan, dan mudah dirawat** (PRD NF-1, NF-2). Konsekuensinya:

1. **Monolitik** — satu aplikasi, satu proses, satu basis data. Tidak ada microservice, message queue, atau cache terpisah.
2. **Satu server** — seluruh aplikasi hidup di satu VPS kecil (1–2 core, 2GB RAM).
3. **Pemisahan penyimpanan** — basis data PostgreSQL + file sistem untuk berkas PDF (di luar document root).
4. **API internal** — frontend dan backend satu kesatuan; endpoint API (API v1.1) adalah kontrak antara keduanya dan tetap terbuka untuk integrasi lain.
5. **Audit append-only** — semua perubahan status terekam, tidak ada yang bisa mengubah atau menghapus jejak.

## 2. Diagram Arsitektur

```mermaid
flowchart LR
    subgraph User["Pengguna"]
        GURU["Guru (HP/PC)"]
        PET["Verifikator Unit & Dinas (PC)"]
        PIM["Pimpinan (PC)"]
        ADM["Admin (PC)"]
        PUB["Publik (HP/PC)"]
    end

    subgraph VPS["VPS — 1 node"]
        NGINX["Nginx\n(HTTPS, static, reverse proxy)"]
        APP["Aplikasi Monolitik\n- HTTP/Routing\n- Auth & RBAC\n- Service (pengajuan, verifikasi, surat)\n- Generator PDF\n- Impor BKN\n- Statistik publik"]
        DB[("PostgreSQL\n9 tabel")]
        FS["File system\n/var/lib/si-cendikia/files\n(berkas PDF, di luar docroot)"]

        NGINX --> APP
        APP --> DB
        APP --> FS
    end

    subgraph EXTERNAL["Layanan Eksternal (kedepannya)"]
        TTESVC["TTE BSrE/BSSN\n(sertifikat elektronik)"]
    end

    GURU & PET & PIM & ADM --> NGINX
    PUB --> NGINX
    APP -.->|"POST /letters/{id}/sign"| TTESVC
```

Catatan: integrasi TTE BSrE/BSSN ditandai garis putus-putus karena **"kedepannya"** (PRD keputusan #3). Sebelum tersedia, alur TTE berjalan mode simulasi (lihat §6).

## 3. Komponen Aplikasi

| Komponen | Tanggung jawab | Sesuai |
|---|---|---|
| **HTTP & routing** | Terima request, jalankan middleware (auth, RBAC, rate limit), arahkan ke handler | API v1.1 |
| **Auth & RBAC** | Login/logout, sesi/token, pemisahan hak 5 peran, rate limit 5×/15 menit | PRD F-1..F-4 |
| **Service pengajuan** | Buat/submit/resubmit, hitung gaji dari `salary_scales` (ERD §7a), validasi PDF ≤5MB, snapshot | PRD F-5..F-9a |
| **Service verifikasi** | Antrean unit/dinas, approve/reject, mesin status | PRD F-10..F-13, ERD §3 |
| **Service surat** | Konsep dari template e-KGB, nomor otomatis dari template aktif, render PDF, TTE, terbit + update master | PRD F-14..F-17 |
| **Service admin** | Impor BKN + provisi akun, master unit/pengguna, skala gaji, template nomor, audit | PRD F-18..F-21 |
| **Service publik** | Statistik agregat `GET /public/stats` | PRD F-28 |
| **Generator PDF** | Render template surat (variabel: nomor, nama, NIP, unit, TMT, gaji lama→baru, tanggal) | API v1.1, referensi e-KGB |
| **Impor BKN** | Parse file BKN, cocokkan per NIP (tambah/update), buat akun ASN (username=NIP, password=hash NIP), catat riwayat | PRD F-18 |

## 4. Alur Data (End-to-End)

```mermaid
sequenceDiagram
    participant G as Guru
    participant A as Aplikasi
    participant DB as PostgreSQL
    participant FS as File system
    participant T as TTE BSrE (kedepannya)

    G->>A: POST /auth/login (NIP, NIP)
    A->>DB: cek akun + role
    A-->>G: token sesi

    G->>A: POST /submissions (TMT + file PDF ≤5MB)
    A->>DB: hitung gaji dari salary_scales (PNS/PPPK)
    A->>FS: simpan file (nama acak, di luar docroot)
    A->>DB: INSERT submissions (status=menunggu_unit) + audit
    A-->>G: 201

    Note over A: verifikator unit approve → menunggu_dinas
    Note over A: verifikator dinas approve → menunggu_tte
    Note over A: (reject → dikembalikan_unit/dikembalikan_dinas, resubmit sesuai aturan jenjang)

    G->>A: POST /letters/{id}/sign
    A->>DB: generate nomor (template aktif) + INSERT letters
    A->>A: render PDF surat
    A->>T: TTE BSrE (kedepannya) / simulasi (sekarang)
    A->>FS: simpan PDF final (immutable)
    A->>DB: UPDATE submissions (status=terbit) + UPDATE teachers (tmt_kgb_last) + audit
    A-->>G: 201 + nomor surat
```

## 5. Keputusan Desain Kunci

| # | Keputusan | Alasan | Konsekuensi |
|---|---|---|---|
| A-1 | Satu proses monolitik | NF-1/NF-2; mudah deploy & debug di VPS 2GB | Tidak ada scaling horizontal terpisah |
| A-2 | Berkas PDF disimpan di luar document root (`/var/lib/si-cendikia/files`), nama file acak (UUID), path disimpan di DB | PRD NF-4; file tidak pernah bisa diakses langsung via URL | Akses hanya lewat endpoint terotorisasi (`GET /submissions/{id}/file`, `GET /letters/{id}/download`) |
| A-3 | Satu tabel `audit_logs` append-only untuk semua aksi | Sederhana, satu tempat telusur | Aplikasi tidak punya operasi UPDATE/DELETE untuk tabel ini |
| A-4 | Gaji dihitung saat submit dan di-snapshot | ERD §2 prinsip #2 | Perubahan skala gaji tidak mengubah surat yang sudah terbit |
| A-5 | Status cukup satu kolom + riwayat di audit | Keputusan owner (submit ulang unit vs dinas) | Mesin status sederhana, tidak ada kolom versi berulang |
| A-6 | `GET /public/stats` di-cache 5 menit | Data agregat jarang berubah | Beban rendah; tanpa cache perlu hitung ulang tiap request |
| A-7 | TTE BSrE terisolasi di belakang satu fungsi/interface | Integrasi "kedepannya"; mudah ganti simulasi → asli | Aplikasi lain tidak tahu detail mekanisme TTE |

## 6. Integrasi TTE BSrE/BSSN (Kedepannya)

- Saat ini: **mode simulasi** — konsep surat ditandatangani dengan placeholder (mis. teks "Dokumen ditandatangani secara elektronik" + nama pejabat + waktu), sudah menghasilkan PDF yang valid untuk uji alur.
- Kedepannya: panggil layanan TTE BSrE/BSSN dengan sertifikat elektronik pimpinan. Integrasi diisolasi dalam satu modul; kontrak API-nya tidak mengubah alur aplikasi (API v1.1 §7 tetap berlaku).
- Desain kolom `letters.tte_receipt_id` sudah disiapkan untuk menyimpan resi/ID dari layanan TTE.
- Jika TTE gagal: pengajuan tetap `menunggu_tte`, pesan error jelas, tidak ada data setengah terbit (transaksi atomik di service surat).

## 7. Keamanan

| Aspek | Desain |
|---|---|
| Autentikasi | Login NIP/password; password di-hash (bcrypt/argon2); sesi/token dibatasi waktu |
| Rate limit login | 5 gagal / 15 menit per username/IP → `429` |
| RBAC | 5 peran terpisah; setiap endpoint divalidasi matriks akses (API v1.1 §2) |
| Upload | Validasi ekstensi + magic bytes PDF + ukuran ≤5MB; simpan di luar docroot; nama acak |
| Download | Hanya lewat endpoint ber-otorisasi; audit `unduh_berkas` / `unduh_surat` |
| Secret | Hanya di `.env` (tidak pernah di-commit); hook PreToolUse memblokir akses agent ke file kredensial |
| CSRF | Token CSRF untuk semua mutasi (jika sesi cookie) / per-token signature (jika header token) |
| Header HTTP | HTTPS wajib (Nginx), HSTS, X-Content-Type-Options, frame/clickjacking protection |
| PII | Beranda publik hanya agregat; endpoint `/public/stats` tanpa data pribadi |

## 8. Operasional

| Aspek | Desain |
|---|---|
| Deploy | Build artefak → salin ke server → systemd unit (satu service) → Nginx reverse proxy + SSL |
| Basis data | PostgreSQL 16; backup terjadwal (pg_dump harian + retensi) |
| Log | Aplikasi → log terstruktur (journald); audit → DB |
| Pemulihan | Restore dari backup DB + sinkronisasi file PDF (keduanya dibackup) |
| Pembaruan skala gaji | Impor baru saat regulasi terbit; surat lama tidak berubah (snapshot) |

## 9. Batasan yang Disengaja

- Tidak ada cache terdistribusi, queue, atau worker terpisah (satu proses cukup untuk skala kabupaten piloting).
- Tidak ada integrasi live ke Dapodik/BKN (master via impor file + pengajuan, sesuai keputusan owner).
- Tidak ada notifikasi push/email (guru memantau lewat aplikasi).
- Tidak ada fitur komentar antar verifikator (catatan penolakan cukup).

## 10. Risiko & Mitigasi

| Risiko | Mitigasi |
|---|---|
| Gaji tidak ditemukan di skala (masa kerja di luar rentang) | Tolak submit dengan pesan jelas + hubungi Dinas (ERD §7a) |
| File BKN rusak / format berubah | Validasi impor, laporan baris gagal, rollback parsial dengan catatan |
| TTE BSrE belum tersedia saat go-live | Mode simulasi jalan; kolom resi siap; kontrak API tetap |
| Lonjakan masa pengajuan massal | Satu VPS 2GB cukup untuk ribuan guru (beban rendah, cache stats); monitor di fase uji |
| Password = NIP (keputusan owner) | Rate limit + audit `login_gagal` + hashing kuat sebagai mitigasi (PRD F-4, NF-5) |
