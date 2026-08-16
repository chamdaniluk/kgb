# PRD — SI CENDIKIA v1.0

**Sistem Cepat Efektif Non-stop Digital Informasi Kenaikan Gaji Berkala ASN**

| | |
|---|---|
| Versi | 1.1 (final — v1.0 + fitur perubahan data kepegawaian; keputusan owner 2026-08-16) |
| Tanggal | 2026-08-16 |
| Owner | Chamdani — Dinas Pendidikan Kabupaten Grobogan |
| Status | Fase 1 dari 6 SELESAI ✅ — lanjut DATABASE ERD |

## 1. Latar Belakang

Kenaikan Gaji Berkala (KGB) adalah hak ASN yang diberikan setiap dua tahun sekali selama memenuhi syarat. Proses pengajuannya di Kabupaten Grobogan saat ini berjalan melalui aplikasi e-KGB yang sudah live. Owner menghendaki sistem baru yang **lebih sederhana dan lebih ringan**, dengan satu tujuan inti: **terbitnya surat/SK KGB** bagi ASN yang memenuhi syarat.

Sistem e-KGB live (`kgb.grobogankab.web.id`) berfungsi sebagai referensi domain, alur kerja, dan template surat, bukan sebagai basis kode proyek ini.

## 2. Tujuan dan Non-Tujuan

### Tujuan
1. ASN (PNS dan PPPK) dapat mengajukan KGB secara daring dengan langkah sesedikit mungkin.
2. Verifikasi berjalan berjenjang dan tercatat: unit kerja → Dinas → TTE pimpinan.
3. Hasil akhir berupa surat/SK KGB dalam format PDF yang dapat diunduh.
4. Sistem ringan untuk dijalankan dan dirawat (prinsip kesederhanaan menjadi kriteria desain utama).

### Non-Tujuan (di luar cakupan)
1. Perhitungan otomatis gaji/pembayaran — sistem ini hanya menerbitkan dokumen pengesahan.
2. Integrasi payroll atau SIMPEG (untuk tahap awal).
3. Pengajuan selain KGB (kenaikan pangkat, mutasi, dsb).

## 3. Pengguna dan Peran

| Peran | Siapa | Hak utama |
|---|---|---|
| **ASN (piloting: guru)** | Guru PNS dan PPPK di lingkungan Dinas Pendidikan Grobogan | Login, buat & kirim pengajuan, unggah berkas, pantau status, unduh surat terbit |
| **Verifikator Unit** | Petugas Korwil (SD), admin SMP, admin SKB sesuai tempat ASN bekerja | Tinjau pengajuan dari unitnya, setujui/tolak dengan catatan, teruskan ke Dinas |
| **Verifikator Dinas** | Petugas Dinas Pendidikan | Tinjau pengajuan yang lolos unit, setujui/tolak, teruskan ke pimpinan |
| **Pimpinan** | Pejabat Dinas yang berwenang (Kepala Dinas) | Tanda Tangan Elektronik (TTE) atas konsep surat |
| **Admin Sistem** | Operator yang ditunjuk Dinas | Kelola master data (impor file BKN, unit, pejabat TTE), lihat audit trail |

> Piloting tahap awal khusus guru. Struktur data dirancang agar nanti dapat diperluas ke tenaga kependidikan lain tanpa perubahan besar.

## 4. Alur Utama (Happy Path)

1. **Login** — ASN masuk dengan NIP sebagai username dan NIP sebagai password.
2. **Buat pengajuan** — ASN mengisi/melengkapi data pengajuan (data profil diambil dari master data), lalu mengunggah **satu berkas PDF** persyaratan, **maksimal 5MB**.
3. **Submit** — Pengajuan terkunci (tidak dapat diedit) dan masuk antrean verifikasi.
4. **Verifikasi unit** — Verifikator Korwil/SMP/SKB memeriksa kelengkapan dan kebenaran. Jika tidak sesuai, pengajuan ditolak dengan catatan (ASN dapat memperbaiki dan mengirim ulang). Jika sesuai, diteruskan ke Dinas.
5. **Verifikasi Dinas** — Verifikator Dinas memeriksa ulang. Tolak dengan catatan, atau setujui untuk dibuatkan konsep surat.
6. **Konsep surat** — Sistem membuat konsep surat/SK KGB berdasarkan data pengajuan, mengikuti template e-KGB yang sudah ada.
7. **TTE pimpinan** — Pimpinan membubuhkan Tanda Tangan Elektronik (sertifikat elektronik BSrE/BSSN) pada konsep surat.
8. **Penerbitan** — Surat/SK final diterbitkan sebagai **PDF**, dapat diunduh ASN dan Dinas.
9. **Audit trail** — Setiap perubahan status, pemeriksaan, dan unduhan dicatat.

> **Pemeliharaan data**: impor file BKN hanya dilakukan sekali di awal sebagai seeding master data. Setelah itu, pemutakhiran data kepegawaian dilakukan lewat pengajuan perubahan oleh guru, diverifikasi Dinas dengan bukti berkas (lihat §5.6).

## 5. Kebutuhan Fungsional

### 5.1 Autentikasi & Otorisasi
- F-1 Login ASN dengan NIP sebagai username dan NIP sebagai password (kebijakan owner; password tetap di-hash saat disimpan).
- F-2 Login petugas (unit, Dinas, pimpinan, admin) dengan akun yang dikelola admin.
- F-3 Pembatasan akses berbasis peran (ASN hanya melihat pengajuannya sendiri; verifikator hanya unitnya).
- F-4 Batasi percobaan login berulang (rate limiting) untuk mencegah brute-force.

### 5.2 Pengajuan KGB
- F-5 ASN membuat pengajuan baru; sistem menampilkan data kepegawaian dari master (hasil impor file BKN) beserta perhitungan gaji berkala berikutnya.
- F-6 Unggah berkas persyaratan: **satu berkas PDF saja**, ukuran **maksimal 5MB**. Validasi dilakukan di sisi klien dan server.
- F-7 Pengajuan yang sudah disubmit terkunci; perbaikan hanya lewat mekanisme tolak-kembali.
- F-8 ASN memantau status pengajuan secara real-time (status terakhir + riwayat).
- F-9 ASN mengunduh surat/SK yang sudah terbit.

### 5.3 Verifikasi Berjenjang
- F-10 Verifikator unit melihat daftar pengajuan unitnya, dapat menyetujui (dengan catatan opsional) atau menolak (dengan catatan wajib).
- F-11 Verifikator Dinas melihat pengajuan yang lolos unit; hak setujui/tolak yang sama.
- F-12 Pengajuan ditolak dikembalikan ke ASN; ASN dapat memperbaiki dan submit ulang.
- F-13 Setiap aksi verifikasi tercatat: siapa, kapan, keputusan, catatan.

### 5.4 TTE & Penerbitan
- F-14 Konsep surat/SK KGB dibuat otomatis dari data pengajuan yang disetujui, mengikuti template e-KGB (kop Dinas Pendidikan Grobogan; isi memuat nama, NIP, unit kerja, TMT, gaji lama → gaji baru; blok tanda tangan Kepala Dinas; footer keaslian dokumen).
- F-15 Pimpinan melakukan TTE dengan **sertifikat elektronik BSrE/BSSN**.
- F-16 Setelah TTE, surat final berformat PDF diterbitkan dan diberi nomor.
- F-17 Surat final tidak dapat diubah (immutable); unduhan dicatat.

### 5.5 Administrasi
- F-18 Admin melakukan **impor file BKN sekali di awal** untuk seeding master data ASN (nama, NIP, status PNS/PPPK, unit kerja, golongan/ruang, gaji pokok, TMT KGB terakhir). Setelah seeding, pemutakhiran lewat mekanisme §5.6.
- F-19 Admin mengelola unit kerja (Korwil/SMP/SKB) dan menugaskan verifikator.
- F-20 Admin menunjuk pejabat TTE.
- F-21 Audit trail dapat dilihat admin untuk semua pengajuan.

### 5.6 Perubahan Data Kepegawaian (setelah seeding)
- F-22 Guru dapat mengajukan **perubahan data kepegawaiannya sendiri** (mis. pangkat/golongan, gaji pokok, unit kerja, TMT KGB terakhir) dengan mengisi nilai baru.
- F-23 Setiap perubahan wajib melampirkan **bukti berkas PDF, maksimal 5MB** (mis. SK pangkat baru, SK mutasi).
- F-24 Perubahan bersifat **tidak aktif sampai disetujui Dinas**: data master guru baru diperbarui setelah persetujuan (mengikuti pola "Persetujuan Perubahan Profil" e-KGB).
- F-25 Verifikator Dinas menyetujui atau menolak perubahan dengan catatan; penolakan wajib disertai alasan.
- F-26 Semua permintaan perubahan tercatat di audit trail: siapa, kapan, field apa, nilai lama → baru, keputusan, catatan.
- F-27 Permintaan perubahan yang masih menunggu **tidak memblokir** pengajuan KGB (pengajuan memakai snapshot data saat submit).

## 6. Kebutuhan Non-Fungsional

| ID | Kebutuhan |
|---|---|
| NF-1 | **Ringan**: berjalan di satu VPS kecil (1–2 core, 2GB RAM); dependensi minimal |
| NF-2 | Sederhana: satu aplikasi monolitik lebih diutamakan daripada microservice |
| NF-3 | Audit trail lengkap untuk seluruh perubahan status |
| NF-4 | Berkas unggahan disimpan di luar document root; hanya dapat diakses lewat endpoint terotorisasi |
| NF-5 | Password disimpan dengan hashing kuat (bcrypt/argon2); tidak pernah disimpan/dikirim polos |
| NF-6 | Backup basis data terjadwal |
| NF-7 | Antarmuka responsif (dipakai dari ponsel) |

## 7. Metrik Keberhasilan

1. ASN dapat menyelesaikan pengajuan (login → submit) dalam waktu singkat tanpa bantuan.
2. Tidak ada pengajuan yang hilang atau berubah tanpa jejak audit.
3. Surat yang terbit memiliki format konsisten dan dapat diverifikasi keasliannya.
4. Beban server tetap rendah pada puncak penggunaan (periode massal KGB).

## 8. Keputusan Owner (2026-08-16) — pengganti open questions

| # | Pertanyaan | Keputusan owner |
|---|---|---|
| 1 | Kebijakan login ASN | **NIP sebagai username DAN NIP sebagai password**, seperti itu seterusnya. (Dicatat sebagai risiko yang diterima; mitigasi via rate-limiting F-4 dan hashing NF-5) |
| 2 | Berkas wajib | **Satu berkas PDF saja** per pengajuan, maksimal 5MB |
| 3 | Mekanisme TTE | **Sertifikat elektronik BSrE/BSSN** (kedepannya; desain harus mengakomodasi) |
| 4 | Sumber data guru | **File BKN milik Dinas Pendidikan** — impor **hanya sekali di awal** sebagai seeding; pemutakhiran selanjutnya lewat pengajuan perubahan guru + verifikasi Dinas dengan bukti berkas |
| 5 | Format surat | **Mengikuti template e-KGB** yang sudah ada (acuan resmi) |

## 9. Lampiran: Referensi Domain (dari sistem live)

Fakta yang dikonfirmasi dari source e-KGB (read-only):
- Template surat (`app/Views/letters/template.php`) memuat variabel: `number` (nomor surat), `teacher_name`, `nip`, `unit_name`, `proposed_tmt` (TMT KGB), `current_salary` (gaji lama), `next_salary` (gaji baru), `issuedAt` (tanggal terbit). Kop: Pemerintah Kabupaten Grobogan — Dinas Pendidikan, Jl. Bhayangkara No. 1 Purwodadi. Blok tanda tangan: Kepala Dinas Pendidikan.
- Formulir pengajuan memuat field: `proposed_tmt`, `sk_kgb`, `skp`, `sk_pangkat`, `skp_predikat_sebelumnya`, `skp_predikat_terakhir`, `declared_no_block`.
- Output surat dinamai `surat-kgb-{id}.pdf` dan diunduh melalui endpoint terotorisasi.
- Autentikasi sistem live memakai CodeIgniter Shield (session, group, permission).
