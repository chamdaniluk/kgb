# API DOCUMENTATION — SI CENDIKIA v1.2 (FINAL)

> Fase 3 dari 6 — **FINAL (v1.2: TTE via eSign Kominfo), disetujui owner 2026-08-16**. Sumber: PRD v1.6 + ERD v1.0 FINAL.
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
- Gaji dihitung otomatis saat submit dari `salary_scales` (PNS: golongan SIPPASN + masa kerja; PPPK: golongan SK I–XVII + masa kerja). Kombinasi tidak ditemukan → `SALARY_SCALE_NOT_FOUND`, pengajuan tidak dibuat.
- PNS: `pangkat_gol` mengikuti SIPPASN (tidak dapat diubah). PPPK: `pangkat_gol` I–XVII sesuai SK pengangkatan.
- **KP Terakhir (opsional)**: `last_kp_golongan` + `last_kp_tmt` (+ nomor/tanggal/pejabat SK KP). Bila KP lebih baru dari KGB terakhir, gaji KGB mengacu golongan KP; jangka waktu KGB tetap 2 tahun dari KGB terakhir. Tanpa KP: golongan sama, MKG dari KGB terakhir. KP sebagian → `422`.
- `GET /salary-preview` menerima parameter KP yang sama (`last_kp_golongan`, `last_kp_tmt`) dan mengembalikan `golongan_efektif`.
- Status awal mengikuti jenjang unit guru: unit Dinas langsung `menunggu_dinas` (tanpa antrean unit), jenjang lain `menunggu_unit`. Audit: `submit`.
- Setiap usulan membawa `unit_type` (TK/SD/SMP/SKB/Korwil/Dinas), `unit_district` (19 kecamatan/DINAS), dan `unit_korwil_name` (Korwil induk bila ada).

### `POST /submissions/{id}/resubmit` — submit ulang setelah ditolak
Hanya valid bila status `dikembalikan_unit` atau `dikembalikan_dinas` (`409` jika tidak). File & TMT boleh diganti.
- Dari `dikembalikan_unit` → status menjadi `menunggu_unit`.
- Dari `dikembalikan_dinas` → status **langsung** `menunggu_dinas` (keputusan owner).

### `GET /submissions/{id}/file` — unduh berkas ASN
PDF yang diunggah (hanya milik sendiri + verifikator terkait + admin). Audit: `unduh_berkas`.

## 5. Verifikasi Unit (Korwil/SMP/SKB)

Cakupan jenjang: akun Korwil melihat TK/SD sekecamatan (relasi `parent_id` ke Korwil atau kolom `district` yang sama); akun SMP/SKB melihat unitnya sendiri; akun `verifikator_unit` wajib memiliki unit. Unit Dinas tidak punya antrean unit.

### `GET /verifications/unit`
Pengajuan berstatus `menunggu_unit` dalam cakupan verifikator. Filter: `?status=`.

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

Admin Dinas melihat semua usulan (`GET` + detail) tetapi hanya dapat approve/reject usulan pegawai Dinas (`unit_type = dinas`); untuk jenjang TK/SD/SMP/SKB `POST` approve/reject mengembalikan `403 FORBIDDEN` dan hanya `verifikator_dinas` yang dapat memproses.

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
{ "passphrase": "..." }   // passphrase eSign pimpinan (diinput pimpinan, tidak pernah disimpan)
```
Proses (transaksi atomik):
1. Generate nomor dari `letter_number_templates` aktif (`{SEQ}` per tahun, `{YEAR}`).
2. Render PDF dari template e-KGB dengan data snapshot.
3. Panggil **eSign Kominfo** `POST /api/v2/sign/pdf` (mode VISIBLE, NIK pimpinan + passphrase, TTD base64, posisi blok ttd, file = base64 PDF konsep) — detail pola di PRD §10.
4. Simpan `id_dokumen` respons eSign ke `letters.tte_receipt_id`.
5. `GET /api/sign/download/{id_dokumen}` → simpan PDF final.
6. Insert `letters`, status pengajuan → `terbit`, master `teachers` diperbarui (`tmt_kgb_last ← proposed_tmt`).

→ `201` `{ "data": { "number": "800/001/4.2/2026", "issued_at": "..." } }`
→ `502/422` `TTE_FAILED` jika layanan eSign gagal (pengajuan tetap `menunggu_tte`).
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

### `POST /admin/sync-sippasn` · `POST /admin/sync-sippasn/preview` · `GET /admin/sync-sippasn/history`
Sinkronisasi master pegawai Dinas Pendidikan dari SIPP ASN (`GET {SIPPASN_BASE_URL}/api/pegawai`, tanpa kredensial; default `https://sippasn.grobogan.go.id`). SIPPASN adalah sumber utama data induk.
Seluruh pegawai aktif Dinas Pendidikan (guru + non-guru: pelaksana, pengawas, penilik, pamong, struktural) di-upsert berkunci NIP (idempoten; akun ASN baru diprovisi username=NIP/password=hash(NIP)). Jenis ASN diturunkan dari segmen NIP (bulan 01–12 = PNS, 21–22 = PPPK); kategori guru/non-guru dari jabatan. SIPPASN menang untuk nama, jenis, kategori, unit, golongan, jabatan; kolom milik KGB (masa kerja terbit, TMT) tidak ditimpa.
Pemetaan golongan: PNS → golongan SIPPASN apa adanya (fix); PPPK bergolongan resmi (I–XVII) → dipakai; PPPK dengan padanan PNS / belum bergolongan → default IX, dapat diubah saat usul KGB sesuai SK.
Selain pemicu manual, sinkron berjalan otomatis tiap malam di VPS (`SIPPASN_NIGHTLY_SYNC`, default tiap 24 jam; audit `sinkron_sippasn_malam`) dan refresh per akun terjadi setiap login ASN (audit `sinkron_sippasn_login`, best-effort). Snapshot daftar SIPPASN di-cache di memori (TTL default 6 jam, `SIPPASN_SNAPSHOT_TTL`); bila SIPPASN mati, data cache terakhir tetap dipakai.
`POST .../sync-sippasn` menerima body opsional `{ "limit": 100 }` untuk uji bertahap; respons ringkasan sinkron. `POST .../preview` hanya membaca (tanpa tulis DB, tanpa audit) dan mengembalikan hitungan + 5 contoh. `GET .../history?limit=10` membaca riwayat (`bkn_imports` asal `sippasn`).
Hak: admin, admin_dinas. Audit: `sinkron_sippasn` (+ `sinkron_sippasn_selesai`). Env: `SIPPASN_BASE_URL`, `SIPPASN_TIMEOUT_SECONDS` (default 60).

### `GET /admin/teachers` · `GET /admin/teachers/{id}`
Master ASN (read-only; perubahan data hanya lewat alur pengajuan).
Filter daftar: `q`, `unit_id`, `unit_type` (sd/smp/skb/tk/korwil/dinas), `kecamatan` (19 kecamatan/DINAS; cocok ke korwil `KORWILCAM <X>` atau parent korwil). Respons teacher menyertakan `unit_type`.

### `GET /admin/trial-notice` · `PUT /admin/trial-notice`
Saklar banner "Pemberitahuan Pelaksanaan Uji Coba Internal" di beranda (diubah lewat kartu "Masa uji coba internal" di Dasbor Admin). GET mengembalikan `{enabled, until, until_tampil, until_efektif, sumber}` (`sumber`: `db` bila admin pernah menyimpan, `env` bila dari `TRIAL_UNTIL`, `default` 7 hari). PUT menerima `{enabled?, until?}` (`until`: `YYYY-MM-DD`/RFC3339, atau `null`/kosong = kembali ke env/default); tanggal salah → 422/400.
Hak: admin, admin_dinas (UI hanya di Dasbor Admin peran `admin`). Audit: `ubah_pengumuman_uji_coba`.

### Unit & Pengguna
- `GET /admin/units` · `POST /admin/units` — daftar/buat unit (Korwil/SMP/SKB).
- `GET /admin/users` · `POST /admin/users` · `PATCH /admin/users/{id}` — kelola akun petugas & verifikator (termasuk penunjukan unit verifikator & pejabat TTE); nonaktifkan akun.
- Filter daftar pengguna: `role_group` (admin/guru/unit/dinas), `q`, `unit_id`, `unit_type`, `kecamatan`. Respons menyertakan `role_group` + `unit_type`.
- `GET /admin/filter-options` — opsi dropdown dasbor (`units`, `kecamatan`, `unit_types`, `role_groups`).
- `GET /admin/audit-logs` — filter `action`, `actor` (nama/username), `from`/`to` (`YYYY-MM-DD`); `GET /admin/audit-logs/actions` — daftar aksi untuk dropdown.

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

## 11. Statistik Publik (beranda, tanpa login)

### `GET /public/stats`
Statistik agregat untuk beranda publik (PRD F-28). **Tanpa token**; hanya data ringkasan, tidak ada data pribadi guru.

```json
{ "data": {
    "teachers_total": 1248,
    "submissions_active": 17,
    "letters_issued": 342,
    "submissions_per_month": [
      { "month": "2026-07", "count": 3 },
      { "month": "2026-08", "count": 9 }
    ],
    "per_status": {
      "menunggu_unit": 4, "menunggu_dinas": 6, "menunggu_tte": 2,
      "dikembalikan": 3, "terbit": 342
    },
    "per_unit": [
      { "unit": "Korwil Kec. Grobogan", "count": 12 },
      { "unit": "SMPN 1 Purwodadi", "count": 8 }
    ]
} }
```

- Halaman `Tatacara Penggunaan` (`/panduan`) dan `Alur Pengajuan` (`/alur`) adalah **konten statis** — tidak butuh endpoint.
- Respons boleh di-cache (mis. 5 menit) karena hanya agregat; audit tidak diperlukan untuk baca publik.
