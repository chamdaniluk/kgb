# API DOCUMENTATION — SI CENDIKIA v1.0 (FINAL)

> Fase 3 dari 6 — **FINAL, disetujui owner 2026-08-16**. Sumber: PRD v1.3 + ERD v1.0 FINAL.
> Konvensi teknis final (framework, bentuk token, dsb.) diputuskan di fase TECH STACK; dokumen ini mendefinisikan **kontrak** yang harus dipenuhi implementasi apa pun.

## 1. Konvensi Umum

- **Base URL**: `/api/v1`
- **Format**: JSON (kecuali unduhan file → `application/pdf`)
- **Autentikasi**: token sesi yang diperoleh dari `POST /auth/login` (mekanisme token — JWT/session cookie — diputuskan di fase TECH STACK). Header: `Authorization: Bearer <token>`.
- **Peran (role)**: `asn`, `verifikator_unit`, `verifikator_dinas`, `pimpinan`, `admin`.
- **Rate limiting login**: maksimal 5 percobaan gagal per 15 menit per username/IP (PRD F-4), respons `429`.
- **Semua aksi status & unduhan** dicatat ke `audit_logs` secara transparan.

### Format Respons

```json
// sukses
{ "data": { ... } }
{ "data": [ ... ], "meta": { "page": 1, "per_page": 20, "total": 137 } }

// error
{ "error": { "code": "SUBMISSION_LOCKED", "message": "Pengajuan sudah terkunci." } }
```

### Kode Status Utama

| HTTP | Makna |
|---|---|
| 200 | Sukses |
| 201 | Dibuat |
| 400 | Validasi gagal (detail di `error.message`) |
| 401 | Belum login / token tidak valid |
| 403 | Login tapi tidak berwenang (mis. ASN akses data guru lain) |
| 404 | Tidak ditemukan |
| 409 | Konflik status (mis. approve pengajuan yang sudah ditolak) |
| 422 | Berkas tidak valid (bukan PDF / >5MB) |
| 429 | Rate limit login |

### Kode Error Domain (contoh)

`LOGIN_RATE_LIMITED`, `FILE_NOT_PDF`, `FILE_TOO_LARGE`, `SUBMISSION_LOCKED`, `SUBMISSION_NOT_RETURNED`, `INVALID_STATUS_TRANSITION`, `SALARY_SCALE_NOT_FOUND`, `ALREADY_ACTIVE_SUBMISSION`, `TTE_FAILED`.

## 2. Matriks Akses Endpoint

| Endpoint group | asn | verifikator_unit | verifikator_dinas | pimpinan | admin |
|---|:-:|:-:|:-:|:-:|:-:|
| Auth & profil sendiri | ✅ | ✅ | ✅ | ✅ | ✅ |
| Pengajuan ASN (milik sendiri) | ✅ | — | — | — | — |
| Verifikasi unit | — | ✅ (unitnya) | — | — | — |
| Verifikasi dinas | — | — | ✅ | — | — |
| TTE | — | — | — | ✅ | — |
| Administrasi (impor BKN, master, template, audit) | — | — | — | — | ✅ |
| Lihat semua pengajuan (read-only) | — | — | ✅ | ✅ | ✅ |

## 3. Auth & Profil

### `POST /auth/login`
```json
{ "username": "196803121992031004", "password": "196803121992031004" }
```
→ `200`: `{ "data": { "token": "...", "role": "asn", "name": "...", "unit": "SDN 3 Ngraji" } }`
→ `401` kredensial salah (dicatat `login_gagal`), `429` rate limit.

### `POST /auth/logout`
Membatalkan token. Audit: `logout`.

### `GET /me`
Profil pengguna login. Untuk `asn` menyertakan data kepegawaian (dari `teachers`): NIP, nama, status PNS/PPPK, unit, pangkat/golongan, masa kerja, TMT KGB terakhir, dan **pratinjau gaji** (gaji sekarang & berikutnya, dihitung dari `salary_scales` — lihat ERD §7a).

## 4. Pengajuan KGB (ASN)

### `GET /submissions`
Daftar pengajuan milik ASN login (historis + aktif).

### `GET /submissions/{id}`
Detail pengajuan + riwayat status (dari `audit_logs`) + catatan penolakan terakhir. ASN hanya boleh akses miliknya (`403` jika bukan).

### `POST /submissions` — buat/submit pengajuan
```
multipart/form-data:
  proposed_tmt : "2026-09-01"          (tanggal, wajib)
  file         : <PDF, maks 5MB>        (wajib)
```
Aturan (semua `400/422` dengan pesan jelas):
- Hanya boleh satu pengajuan aktif per ASN (`ALREADY_ACTIVE_SUBMISSION`).
- Gaji dihitung otomatis saat submit dari `salary_scales` (PNS: golongan + masa kerja; PPPK: IX + masa kerja). Kombinasi tidak ditemukan → `SALARY_SCALE_NOT_FOUND`, pengajuan tidak dibuat.
- PPPK: `pangkat_gol` harus `IX`.
- Status awal: `menunggu_unit`. Audit: `submit`.

### `POST /submissions/{id}/resubmit` — submit ulang setelah ditolak
Hanya valid bila status `dikembalikan_unit` atau `dikembalikan_dinas` (`409` jika tidak). File & TMT boleh diganti.
- Dari `dikembalikan_unit` → status menjadi `menunggu_unit`.
- Dari `dikembalikan_dinas` → status **langsung** `menunggu_dinas` (keputusan owner).

### `GET /submissions/{id}/file` — unduh berkas ASN
PDF yang diunggah (hanya milik sendiri + verifikator terkait + admin). Audit: `unduh_berkas`.

## 5. Verifikasi Unit (Korwil/SMP/SKB)

### `GET /verifications/unit`
Pengajuan berstatus `menunggu_unit` milik unit verifikator. Filter: `?status=`.

### `GET /verifications/unit/{id}`
Detail + berkas (untuk diperiksa).

### `POST /verifications/unit/{id}/approve`
```json
{ "note": "opsional" }
```
Status `menunggu_unit` → `menunggu_dinas`. Audit: `setuju_unit`.

### `POST /verifications/unit/{id}/reject`
```json
{ "note": "wajib, alasan penolakan" }
```
Status → `dikembalikan_unit`. Audit: `tolak_unit`.

## 6. Verifikasi Dinas

### `GET /verifications/dinas`
Pengajuan berstatus `menunggu_dinas`. Filter: `?status=`.

### `GET /verifications/dinas/{id}`
Detail + berkas + riwayat verifikasi unit.

### `POST /verifications/dinas/{id}/approve`
```json
{ "note": "opsional" }
```
Status → `menunggu_tte`; sistem menyiapkan konsep surat (data snapshot pengajuan). Audit: `setuju_dinas`.

### `POST /verifications/dinas/{id}/reject`
```json
{ "note": "wajib" }
```
Status → `dikembalikan_dinas`. Audit: `tolak_dinas`.

## 7. TTE & Penerbitan (Pimpinan)

### `GET /letters/pending-tte`
Pengajuan berstatus `menunggu_tte` beserta pratinjau konsep surat.

### `POST /letters/{submission_id}/sign` — TTE + terbitkan
```json
{ "credential_ref": "..." }   // referensi kredensial BSrE/BSSN pimpinan (mekanisme detail: fase ARCHITECTURE)
```
Proses (transaksi atomik):
1. Generate nomor dari `letter_number_templates` aktif (`{SEQ}` per tahun, `{YEAR}`).
2. Render PDF dari template e-KGB dengan data snapshot.
3. TTE BSrE/BSSN (kedepannya; sebelum integrasi tersedia, mode simulasi dengan penanda).
4. Insert `letters`, status pengajuan → `terbit`, master `teachers` diperbarui (`tmt_kgb_last ← proposed_tmt`).

→ `201` `{ "data": { "number": "800/001/4.2/2026", "issued_at": "..." } }`
→ `502/422` `TTE_FAILED` jika layanan TTE gagal (pengajuan tetap `menunggu_tte`).
Audit: `tte`, `terbit`.

## 8. Unduhan Surat

### `GET /letters/{id}/download`
PDF final. Hak: ASN pemilik, verifikator terkait, pimpinan, admin. Audit: `unduh_surat`.

### `GET /letters?year=2026` (dinas/admin/pimpinan)
Rekap surat terbit.

## 9. Administrasi

### `POST /admin/import-bkn`
```
multipart/form-data: file=<Excel/CSV BKN>
```
Seeding master + provisi akun (username=NIP, password=hash(NIP)). Respons: ringkasan `{ rows_total, rows_created, rows_updated, rows_skipped, notes }`. Audit: `impor_bkn`. (Catatan: dijalankan sekali di awal; tetap tersedia untuk koreksi massal oleh admin dengan audit.)

### `GET /admin/teachers` · `GET /admin/teachers/{id}`
Master ASN (read-only; perubahan data hanya lewat alur pengajuan).

### Unit & Pengguna
- `GET /admin/units` · `POST /admin/units` — daftar/buat unit (Korwil/SMP/SKB).
- `GET /admin/users` · `POST /admin/users` · `PATCH /admin/users/{id}` — kelola akun petugas & verifikator (termasuk penunjukan unit verifikator & pejabat TTE); nonaktifkan akun.

### Skala Gaji
- `GET /admin/salary-scales?asn_type=pns&golongan=III/a` — lihat skala.
- `POST /admin/salary-scales/import` — unggah skala baru saat ada regulasi gaji baru (PP/Perpres). Audit: `impor_skala_gaji`.

### Template Nomor Surat
- `GET /admin/letter-number-templates` · `POST /admin/letter-number-templates` — buat pola baru (mis. `800/{SEQ}/4.2/{YEAR}`); template baru otomatis aktif, yang lama nonaktif (surat terbit lama tidak terpengaruh karena nomor tersimpan di `letters`).

### Audit
- `GET /admin/audit-logs?submission_id=&action=&from=&to=` — telusur jejak (read-only).

## 10. Contoh Alur Lengkap (Sequence)

```
Guru      : POST /submissions            → menunggu_unit
Korwil    : POST .../approve             → menunggu_dinas
Dinas     : POST .../reject {note}       → dikembalikan_dinas
Guru      : POST .../resubmit            → menunggu_dinas  (LANGSUNG, tidak lewat unit)
Dinas     : POST .../approve             → menunggu_tte
Pimpinan  : POST /letters/{id}/sign      → terbit (nomor otomatis, PDF, TTE)
Guru      : GET /letters/{id}/download   → surat-kgb-{id}.pdf
```

Setiap panah di atas menulis satu baris `audit_logs`.
