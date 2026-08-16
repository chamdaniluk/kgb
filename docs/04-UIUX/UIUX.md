# UI/UX — SI CENDIKIA v0.1 (DRAF)

> Fase 4 dari 6. Sumber: PRD v1.3 + ERD v1.0 FINAL + API v1.0. Menunggu review owner.

## 1. Prinsip Desain

1. **Sederhana & ringan** (NF-1, NF-2): satu halaman per tugas; tanpa animasi berat; font sistem bawaan (tanpa unduhan webfont); CSS minimal tanpa framework besar.
2. **Mobile-first untuk guru** (NF-7): guru mengakses dari ponsel; layout satu kolom, tombol besar, form pendek.
3. **Desktop untuk petugas** (verifikator, pimpinan, admin): tabel + panel dua kolom.
4. **Bahasa Indonesia** di seluruh antarmuka; istilah birokrasi dipertahankan (TMT, KGB, NIP).
5. **Status terlihat jelas**: setiap pengajuan selalu menampilkan posisi prosesnya (badge warna + timeline).
6. **Tidak ada halaman mati**: setiap layar punya satu aksi utama yang jelas.

## 2. Peta Aplikasi (Sitemap)

```
/login
/guru
  /guru/dashboard            (beranda ASN)
  /guru/pengajuan/baru       (form + unggah PDF)
  /guru/pengajuan/{id}       (detail + timeline + unduh surat)
/unit                        (beranda verifikator unit)
  /unit/{id}                 (detail + approve/reject)
/dinas                       (beranda verifikator dinas)
  /dinas/{id}                (detail + approve/reject)
/pimpinan                    (antrean TTE)
  /pimpinan/{submission_id}  (pratinjau konsep + TTE)
/admin
  /admin/dashboard
  /admin/import-bkn
  /admin/guru                (master, read-only)
  /admin/unit
  /admin/pengguna
  /admin/skala-gaji
  /admin/template-nomor
  /admin/audit
```

Navigasi: header bar sederhana (logo kiri, nama pengguna + peran kanan, logout). Menu berbeda per peran — pengguna hanya melihat miliknya.

## 3. Sistem Status & Warna

| Status | Badge | Warna | Terlihat oleh |
|---|---|---|---|
| `menunggu_unit` | Menunggu Verifikasi Unit | biru | guru, unit |
| `menunggu_dinas` | Menunggu Verifikasi Dinas | biru | guru, dinas |
| `menunggu_tte` | Menunggu TTE Pimpinan | ungu | guru, dinas, pimpinan |
| `dikembalikan_unit` | Dikembalikan Unit — Perbaiki | merah | guru |
| `dikembalikan_dinas` | Dikembalikan Dinas — Perbaiki | merah | guru |
| `terbit` | Surat Terbit | hijau | semua |

Timeline vertikal dengan 5 titik: **Diajukan → Unit → Dinas → TTE → Terbit**. Titik terlewati = hijau; titik saat ini = biru berdenyut; penolakan = merah dengan catatan di bawahnya.

## 4. Wireframe Layar Utama

### 4.1 Login (semua peran)

```
┌─────────────────────────────────┐
│         SI CENDIKIA             │
│  Kenaikan Gaji Berkala ASN      │
│  Dinas Pendidikan Kab. Grobogan │
│                                 │
│  NIP                            │
│  ┌───────────────────────────┐  │
│  │ 196803121992031004        │  │
│  └───────────────────────────┘  │
│  Password                       │
│  ┌───────────────────────────┐  │
│  │ ••••••••••••••••••        │  │
│  └───────────────────────────┘  │
│                                 │
│  [        MASUK               ] │
│                                 │
│  Percobaan login dibatasi.      │
└─────────────────────────────────┘
```
- Satu form untuk semua peran; sistem mengenali peran dari akun.
- Kesalahan: pesan umum "NIP atau password salah" (tanpa bocorkan mana yang salah). Percobaan ke-6 dalam 15 menit: "Terlalu banyak percobaan, coba lagi nanti" (`429`).

### 4.2 Dasbor Guru

```
┌─────────────────────────────────┐
│ ☰ SI CENDIKIA        Bu Sri ⎋   │
├─────────────────────────────────┤
│ ┌─────────────────────────────┐ │
│ │ Sri Lestari, S.Pd.          │ │
│ │ NIP 196803121992031004      │ │
│ │ PNS · III/a · SDN 3 Ngraji  │ │
│ │ Masa kerja 24 th            │ │
│ │ TMT KGB terakhir: 01-04-2024│ │
│ └─────────────────────────────┘ │
│                                 │
│ Pratinjau KGB Anda              │
│ Rp 3.876.400  →  Rp 4.012.300   │
│ (gaji sekarang)   (berikutnya)  │
│                                 │
│ ┌─────────────────────────────┐ │
│ │ ● Menunggu Verifikasi Dinas │ │
│ │ Pengajuan 12 Agt 2026       │ │
│ │                    [Lihat]  │ │
│ └─────────────────────────────┘ │
│                                 │
│ [     + AJUKAN KGB BARU       ] │  ← aktif hanya jika tidak ada
│                                 │     pengajuan aktif & sudah 2 tahun
│ Riwayat                         │
│ 2024 · Terbit · [Unduh PDF]     │
│ 2022 · Terbit · [Unduh PDF]     │
└─────────────────────────────────┘
```
- Tombol "Ajukan KGB Baru" dinonaktifkan + tooltip jika belum 2 tahun sejak TMT terakhir atau ada pengajuan aktif.
- Pratinjau gaji dihitung dari `salary_scales` (logika ERD §7a); jika lookup gagal, tampil pesan "Data gaji belum tersedia, hubungi Dinas".

### 4.3 Form Pengajuan Baru

```
┌─────────────────────────────────┐
│ ‹ Kembali     Pengajuan KGB     │
├─────────────────────────────────┤
│ Usulan TMT KGB                  │
│ ┌──────────────┐                │
│ │ 01-09-2026 📅│  (otomatis    │
│ └──────────────┘   disarankan) │
│                                 │
│ Gaji sekarang : Rp 3.876.400    │
│ Gaji baru     : Rp 4.012.300    │
│ (otomatis dari tabel skala)     │
│                                 │
│ Berkas persyaratan *            │
│ ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐  │
│   Pilih file PDF (maks 5MB)     │
│   Bukti dukung & kelengkapan    │
│ └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘  │
│ ✓ SK_dukungan.pdf · 1,2 MB [✕]  │
│                                 │
│ ⚠ Hanya PDF. Setelah dikirim,   │
│ pengajuan terkunci.             │
│                                 │
│ [          KIRIM              ] │
└─────────────────────────────────┘
```
- Validasi langsung: ekstensi & ukuran file (pesan: "Berkas harus PDF" / "Berkas melebihi 5MB").
- Konfirmasi singkat sebelum kirim ("Kirim pengajuan? Data tidak dapat diubah setelah dikirim") — satu ketuk.

### 4.4 Detail Pengajuan + Timeline

```
┌─────────────────────────────────┐
│ ‹ Kembali    Pengajuan #241     │
├─────────────────────────────────┤
│ [● Menunggu Verifikasi Dinas]   │
│ Diajukan 12 Agt 2026 · TMT      │
│ usulan 01 Sep 2026              │
│ Rp 3.876.400 → Rp 4.012.300     │
│ Berkas: [Lihat PDF]             │
│                                 │
│ ● Diajukan          12 Agt      │
│ │                               │
│ ● Verifikasi Unit   13 Agt      │
│ │   Korwil Kec. Grobogan        │
│ │   ✔ Disetujui                 │
│ │                               │
│ ◉ Verifikasi Dinas  (proses)    │
│ │                               │
│ ○ TTE Pimpinan                  │
│ │                               │
│ ○ Surat Terbit                  │
│                                 │
└─────────────────────────────────┘
```
Varian **ditolak**: titik merah + kotak catatan:
```
│ ✕ Dikembalikan Dinas 14 Agt     │
│ ┌─────────────────────────────┐ │
│ │ Catatan: Berkas SK belum    │ │
│ │ lengkap, lampirkan SK       │ │
│ │ pangkat terakhir.           │ │
│ └─────────────────────────────┘ │
│ [   PERBAIKI & KIRIM ULANG    ] │
```
- Kirim ulang dari `dikembalikan_unit` → masuk antrean unit lagi; dari `dikembalikan_dinas` → langsung antrean Dinas (banner: "Pengajuan Anda langsung diperiksa Dinas").
- Status `terbit`: tombol hijau besar **[UNDUH SURAT KGB (PDF)]** di atas.

### 4.5 Antrean Verifikator Unit (Korwil/SMP/SKB)

```
┌──────────────────────────────────────────────┐
│ SI CENDIKIA · Verifikator Unit    Pak Budi ⎋ │
│ Unit: Korwil Kec. Grobogan                   │
├──────────────────────────────────────────────┤
│ Menunggu verifikasi (4)                      │
│ 🔍 cari nama/NIP...                          │
│ ┌──────────────────────────────────────────┐ │
│ │ Sri Lestari · SDN 3 Ngraji · III/a       │ │
│ │ diajukan 12 Agt · Rp3,8jt → Rp4,0jt      │ │
│ │                              [Periksa ›] │ │
│ ├──────────────────────────────────────────┤ │
│ │ Ahmad Fauzi · SDN 1 Putat · II/b         │ │
│ │ diajukan 11 Agt · ...        [Periksa ›] │ │
│ └──────────────────────────────────────────┘ │
│                                              │
│ Riwayat keputusan (filter: semua/setuju/tolak)│
└──────────────────────────────────────────────┘
```

### 4.6 Halaman Keputusan (unit & dinas, layout sama)

```
┌──────────────────────────────────────────────┐
│ ‹ Kembali      Pemeriksaan #241              │
├─────────────────────────┬────────────────────┤
│ Data guru               │ Berkas (PDF)       │
│ Sri Lestari, S.Pd.      │ ┌────────────────┐ │
│ NIP 1968...1004         │ │  [pratinjau    │ │
│ III/a · masa kerja 24th │ │   PDF inline,  │ │
│ SDN 3 Ngraji            │ │   bisa zoom &  │ │
│                         │ │   unduh]       │ │
│ Gaji: Rp 3.876.400      │ │                │ │
│ Baru: Rp 4.012.300      │ │                │ │
│ (dari tabel skala)      │ └────────────────┘ │
│                         │ [Unduh berkas]     │
│ Catatan (opsional)      │                    │
│ ┌─────────────────────┐ │                    │
│ │                     │ │                    │
│ └─────────────────────┘ │                    │
│                         │                    │
│ [TOLAK] [SETUJUI & TERUSKAN KE DINAS]       │
└─────────────────────────┴────────────────────┘
```
- **Tolak** membuka dialog wajib isi alasan (tidak bisa dikirim kosong).
- Layar verifikator Dinas sama + menampilkan riwayat keputusan unit dan tombol menjadi **[SETUJUI & AJUKAN KE TTE]**.

### 4.7 TTE Pimpinan

```
┌──────────────────────────────────────────────┐
│ SI CENDIKIA · Pimpinan        Kepala Dinas ⎋ │
├──────────────────────────────────────────────┤
│ Menunggu TTE (2)                             │
│ ┌──────────────────────────────────────────┐ │
│ │ #241 Sri Lestari · III/a · TMT 01-09-26  │ │
│ │ lolos verifikasi unit & dinas [Tinjau ›] │ │
│ └──────────────────────────────────────────┘ │
├──────────────────────────────────────────────┤
│ Tinjauan #241                                │
│ ┌────────────────────────────────────────┐   │
│ │ [pratinjau konsep surat persis seperti │   │
│ │  PDF final — kop, isi, blok ttd]       │   │
│ └────────────────────────────────────────┘   │
│                                              │
│ [ TANDA TANGANI & TERBITKAN ]                │
│  → nomor surat otomatis dari template aktif  │
│  → progres: menyiapkan nomor… menandatangani…│
│             menerbitkan PDF… ✔ TERBIT        │
└──────────────────────────────────────────────┘
```
- Tombol menampilkan progres 4 langkah (nomor → PDF → TTE → terbit). Jika TTE BSrE gagal: pesan jelas "Penandatanganan gagal, surat belum diterbitkan — coba lagi" (status tetap menunggu TTE).

### 4.8 Admin (desktop, sidebar)

```
┌────────┬─────────────────────────────────────┐
│ SI     │ Dasbor                              │
│ CENDIKIA│  ┌────────┐ ┌────────┐ ┌────────┐  │
│        │  │ 1.248  │ │   17   │ │  342   │  │
│ 📊 Dasbor│ │ guru   │ │ proses │ │ terbit │  │
│ 📥 Impor│  └────────┘ └────────┘ └────────┘  │
│ 👥 Guru│                                    │
│ 🏫 Unit│ Menunggu tindakan:                 │
│ 👤 Pengguna│  4 unit · 6 dinas · 2 TTE      │
│ 💰 Skala│                                   │
│ 🔢 Nomor│ Impor BKN terakhir: 10 Agt 2026   │
│ 📜 Audit│ (1.248 baris, 3 dilewati) [Detail]│
└────────┴─────────────────────────────────────┘
```
- **Impor BKN**: unggah file → ringkasan hasil (total/ditambah/diperbarui/dilewati + baris catatan error per NIP).
- **Skala gaji**: tabel per tab PNS/PPPK, filter golongan; tombol "Impor skala baru" untuk regulasi terbaru.
- **Template nomor**: pratinjau langsung — ketik pola `800/{SEQ}/4.2/{YEAR}` → tampil contoh hasil `800/001/4.2/2026`.
- **Audit**: tabel waktu · aktor · aksi · pengajuan · detail, dengan filter tanggal/aksi.

## 5. Aturan Interaksi Kunci

| Situasi | Perilaku |
|---|---|
| Guru belum 2 tahun sejak TMT terakhir | Tombol ajukan nonaktif + penjelasan tanggal layak berikutnya |
| Berkas bukan PDF / >5MB | Ditolak di klien sebelum kirim, pesan spesifik |
| Pengajuan aktif sudah ada | Form baru tidak dapat dibuka; link ke pengajuan aktif |
| Ditolak unit | Banner merah + catatan; kirim ulang → antrean unit |
| Ditolak dinas | Banner merah + catatan; kirim ulang → langsung antrean dinas |
| Gaji tak ditemukan di tabel skala | Form tetap bisa dibuka tapi tampil peringatan & tombol kirim nonaktif + pesan hubungi Dinas |
| Sesi habis | Redirect ke login dengan pesan "Sesi berakhir, silakan masuk kembali" |

## 6. Identitas Visual

- **Warna utama**: biru tua pemerintah (#1D4ED8) — tombol utama, header.
- **Warna pendukung**: hijau #16A34A (terbit/sukses), merah #DC2626 (ditolak/error), biru muda untuk proses berjalan, ungu #7C3AED (menunggu TTE), abu-abu netral untuk teks sekunder.
- **Font**: sistem bawaan (`system-ui, -apple-system, sans-serif`) — tanpa unduhan, tetap ringan.
- **Kop surat & logo**: mengikuti identitas Pemkab Grobogan (logo pada pratinjau surat; di aplikasi cukup teks "SI CENDIKIA — Dinas Pendidikan Kabupaten Grobogan").
- **Radius & bayangan minimal**; fokus pada keterbacaan dan kontras (WCAG AA).

## 7. Responsif

| Lebar layar | Penyesuaian |
|---|---|
| < 640px (ponsel) | Satu kolom; tabel antrean jadi kartu; tombol keputusan penuh lebar |
| 640–1024px | Dua kolom longgar |
| > 1024px (desktop petugas/admin) | Sidebar admin; tabel penuh; pratinjau PDF di samping data |

## 8. Yang Disengaja Tidak Ada (anti over-engineering)

Tidak ada: notifikasi push/email (guru cukup buka aplikasi), mode gelap, tema kustom, wizard multi-langkah untuk form satu halaman, unggah multi-berkas, komentar/diskusi antar verifikator (catatan penolakan sudah cukup), dasbor analitik lanjutan. Semua bisa ditambah nanti jika benar-benar diminta.
