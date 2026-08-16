# PRD — SI CENDIKIA v0.1 (DRAF)

**Sistem Cepat Efektif Non-stop Digital Informasi Kenaikan Gaji Berkala ASN**

| | |
|---|---|
| Versi | 0.1 (draf, menunggu review owner) |
| Tanggal | 2026-08-16 |
| Owner | Chamdani — Dinas Pendidikan Kabupaten Grobogan |
| Status | Fase 1 dari 6 (PRD > DATABASE ERD > API DOCUMENTATION > UI/UX > ARCHITECTURE > TECH STACK) |

## 1. Latar Belakang

Kenaikan Gaji Berkala (KGB) adalah hak ASN yang diberikan setiap dua tahun sekali selama memenuhi syarat. Proses pengajuannya di Kabupaten Grobogan saat ini berjalan melalui aplikasi e-KGB yang sudah live. Owner menghendaki sistem baru yang **lebih sederhana dan lebih ringan**, dengan satu tujuan inti: **terbitnya surat/SK KGB** bagi ASN yang memenuhi syarat.

Sistem e-KGB live (`kgb.grobogankab.web.id`) berfungsi sebagai referensi domain dan alur kerja, bukan sebagai basis kode proyek ini.

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
| **Pimpinan** | Pejabat Dinas yang berwenang | Tanda Tangan Elektronik (TTE) atas konsep surat |
| **Admin Sistem** | Operator yang ditunjuk Dinas | Kelola master data (daftar ASN, unit, pejabat TTE), lihat audit trail |

> Piloting tahap awal khusus guru. Struktur data dirancang agar nanti dapat diperluas ke tenaga kependidikan lain tanpa perubahan besar.

## 4. Alur Utama (Happy Path)

1. **Login** — ASN masuk dengan NIP sebagai username dan password.
2. **Buat pengajuan** — ASN mengisi/melengkapi data pengajuan (sebagian data profil dapat diambil dari master data), lalu mengunggah berkas persyaratan dalam bentuk **PDF, maksimal 5MB per file**.
3. **Submit** — Pengajuan terkunci (tidak dapat diedit) dan masuk antrean verifikasi.
4. **Verifikasi unit** — Verifikator Korwil/SMP/SKB memeriksa kelengkapan dan kebenaran. Jika tidak sesuai, pengajuan ditolak dengan catatan (ASN dapat memperbaiki dan mengirim ulang). Jika sesuai, diteruskan ke Dinas.
5. **Verifikasi Dinas** — Verifikator Dinas memeriksa ulang. Tolak dengan catatan, atau setujui untuk dibuatkan konsep surat.
6. **Konsep surat** — Sistem membuat konsep surat/SK KGB berdasarkan data pengajuan.
7. **TTE pimpinan** — Pimpinan membubuhkan Tanda Tangan Elektronik pada konsep surat.
8. **Penerbitan** — Surat/SK final diterbitkan sebagai **PDF**, dapat diunduh ASN dan Dinas.
9. **Audit trail** — Setiap perubahan status, pemeriksaan, dan unduhan dicatat.

## 5. Kebutuhan Fungsional

### 5.1 Autentikasi & Otorisasi
- F-1 Login ASN dengan NIP + password.
- F-2 Login petugas (unit, Dinas, pimpinan, admin) dengan akun yang dikelola admin.
- F-3 Pembatasan akses berbasis peran (ASN hanya melihat pengajuannya sendiri; verifikator hanya unitnya).
- F-4 Batasi percobaan login berulang (rate limiting) untuk mencegah brute-force.

### 5.2 Pengajuan KGB
- F-5 ASN membuat pengajuan baru; sistem menghitung/menampilkan kelayakan awal (contoh: masa kerja sejak KGB terakhir) jika data tersedia.
- F-6 Unggah berkas persyaratan: format **PDF saja**, ukuran **maksimal 5MB per file**. Validasi dilakukan di sisi klien dan server.
- F-7 Pengajuan yang sudah disubmit terkunci; perbaikan hanya lewat mekanisme tolak-kembali.
- F-8 ASN memantau status pengajuan secara real-time (status terakhir + riwayat).
- F-9 ASN mengunduh surat/SK yang sudah terbit.

### 5.3 Verifikasi Berjenjang
- F-10 Verifikator unit melihat daftar pengajuan unitnya, dapat menyetujui (dengan catatan opsional) atau menolak (dengan catatan wajib).
- F-11 Verifikator Dinas melihat pengajuan yang lolos unit; hak setujui/tolak yang sama.
- F-12 Pengajuan ditolak dikembalikan ke ASN; ASN dapat memperbaiki dan submit ulang.
- F-13 Setiap aksi verifikasi tercatat: siapa, kapan, keputusan, catatan.

### 5.4 TTE & Penerbitan
- F-14 Konsep surat/SK KGB dibuat otomatis dari data pengajuan yang disetujui (template dokumen).
- F-15 Pimpinan melakukan TTE atas konsep surat.
- F-16 Setelah TTE, surat final berformat PDF diterbitkan dan diberi nomor.
- F-17 Surat final tidak dapat diubah (immutable); unduhan dicatat.

### 5.5 Administrasi
- F-18 Admin mengelola master data ASN (nama, NIP, status PNS/PPPK, unit kerja, TMT/gaji berkala terakhir).
- F-19 Admin mengelola unit kerja (Korwil/SMP/SKB) dan menugaskan verifikator.
- F-20 Admin menunjuk pejabat TTE.
- F-21 Audit trail dapat dilihat admin untuk semua pengajuan.

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

## 8. Open Questions (menunggu keputusan owner)

> ⚠️ Fase berikutnya (DATABASE ERD) sebaiknya menunggu jawaban ini agar tidak ada efek berantai.

1. **Keamanan login ASN**: brief menyebut password = NIP juga. Ini memudahkan pengguna, tetapi NIP adalah data semi-publik, sehingga akun mudah ditebak. Opsi: (a) biarkan NIP sebagai password namun wajib ganti password saat login pertama, atau (b) password awal = NIP lalu sistem memaksa penggantian. Mana yang dipilih?
2. **Daftar berkas wajib**: sistem live memakai SK KGB terakhir, SKP (predikat terakhir), dan SK pangkat. Untuk SI CENDIKIA, berkas apa saja yang wajib diunggah? Apakah cukup dua seperti e-KGB, atau ada perubahan?
3. **Mekanisme TTE**: TTE pimpinan memakai apa — sertifikat elektronik BSrE/BSSN, tanda tangan gambar + metadata, atau mekanisme lain? Ini memengaruhi arsitektur.
4. **Sumber data ASN**: master data guru diambil dari mana — impor CSV/manual oleh admin, tarik dari Dapodik/PTK, atau salinan dari database e-KGB yang ada?
5. **Format surat**: apakah template surat/SK KGB mengikuti format e-KGB yang sudah ada (yang menghasilkan `surat-kgb-{id}.pdf`), atau ada format baru dari BKPPD?

## 9. Lampiran: Referensi Domain (dari sistem live)

Fakta yang dikonfirmasi dari source e-KGB (read-only):
- Alur status mencakup draf, verifikasi unit, pemeriksaan Dinas, persetujuan, penerbitan.
- Formulir pengajuan memuat field: `proposed_tmt` (usulan TMT), `sk_kgb`, `skp`, `sk_pangkat`, `skp_predikat_sebelumnya`, `skp_predikat_terakhir`, `declared_no_block`.
- Output surat dinamai `surat-kgb-{id}.pdf` dan diunduh melalui endpoint terotorisasi.
- Autentikasi sistem live memakai CodeIgniter Shield (session, group, permission).
