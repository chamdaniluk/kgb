# Panduan Perubahan, Deploy, dan Manajemen SI CENDIKIA

Dokumen kerja untuk owner/operator: cara mengubah kode lalu menaruhnya ke produksi,
cara mengelola layanan lewat dasbor aplikasi sendiri, dan jebakan yang sudah terbukti.

Ditulis 2026-09-12 dari pemeriksaan langsung pada server produksi dan kode sumber
(`sip/si-cendikia`, git HEAD 2026-09-11). Bagian yang **sudah diuji** ditandai ✅,
sisanya prosedur baku yang belum dijalankan pada sesi penulisan.

---

## 0. Fakta produksi (terverifikasi 2026-09-12)

| Komponen | Nilai aktual |
|---|---|
| URL publik | `https://kgb.grobogankab.web.id` (nginx 80 → 301 https, TLS Let's Encrypt) |
| Aplikasi | `/opt/si-cendikia/si-cendikia-server`, systemd unit `si-cendikia`, `User=ubuntu` |
| Alamat internal | `ADDR=127.0.0.1:18081` (bukan 8080 — 8080 milik aplikasi ekgb) |
| Nginx | `proxy_pass http://127.0.0.1:18081`, `client_max_body_size 7m`, timeout 90s → `deploy/nginx-kgb.grobogankab.web.id.conf` |
| Basis data | PostgreSQL 16 di VPS, database `si_cendikia` (akses: `sudo -u postgres psql -d si_cendikia`) |
| Berkas PDF privat | `FILE_ROOT=/var/lib/si-cendikia/files` |
| Migrasi | `MIGRATIONS_DIR=/opt/si-cendikia/migrations` — diterapkan otomatis saat start, tercatat di tabel `schema_migrations` (terakhir: `018_skb_unit.sql`, 2026-09-09) |
| Env service | `/etc/si-cendikia/si-cendikia.env` (rahasia — jangan dibaca-tampilkan, jangan dicommit) |
| Sumber kanonik | `D:\Code\SI-CENDIKIA\sip\si-cendikia` ← **pakai ini** |
| Salinan lama (arsip) | `D:\Code\SI-CENDIKIA\arsip\si-cendikia-beku-20260825` (git HEAD 2026-08-25) — sudah diarsipkan 2026-09-12, jangan dipakai untuk build/deploy |
| Alat bantu di workstation | `vps/vps_ssh.py "<perintah>"`, `vps/vps_sftp.py upload\|download <lokal> <remote>` (kunci `~/.ssh/vps_ed25519`) |

Akun produksi saat ini: admin `admin.disdik` (aktif), verifikator unit 92, admin dinas 6 aktif,
pimpinan `admin.tte`, ASN 8.917. Role `verifikator_dinas` belum berakun — tahap verifikasi Dinas
dijalankan akun `admin_dinas`.

---

## 1. Alur perubahan & deploy dari lokal

### 1.1 Sebelum mulai (checklist 5 menit)

1. Pastikan bekerja di **salinan kanonik**: `D:\Code\SI-CENDIKIA\sip\si-cendikia`.
2. `git status` bersih / hanya berisi perubahan yang memang akan dideploy.
3. Semua perubahan yang menyentuh skema sudah punya berkas migrasi baru bernomor lanjut.
4. Catat "tag" deploy hari ini (contoh: `tambah-koreksi-dinas`) — dipakai untuk nama cadangan binary.

### 1.2 Build binary Linux (dari workstation)

```bash
cd /d/Code/SI-CENDIKIA/sip/si-cendikia/src
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" \
  -o ../deploy/si-cendikia-server-linux-amd64 ./cmd/server
ls -lh ../deploy/si-cendikia-server-linux-amd64     # ~16 MB
```

Catatan: build Windows (`go build -o si-cendikia-server.exe ./cmd/server`) hanya untuk uji lokal.
Alternatif build langsung di VPS juga mungkin (Go tersedia di `/snap/bin/go`), tetapi pola yang
sudah berjalan selama ini adalah kirim binary dari lokal.

### 1.3 Kirim binary ke VPS ✅ (jalur transfer teruji)

```bash
cd /d/Code/SI-CENDIKIA/sip
python vps/vps_sftp.py upload \
  D:/Code/SI-CENDIKIA/sip/si-cendikia/deploy/si-cendikia-server-linux-amd64 \
  /opt/si-cendikia/si-cendikia-server.new
```

`/opt/si-cendikia` milik `ubuntu`, jadi transfer tidak perlu `sudo`.
Berkas sementara ditulis dengan akhiran `.new` supaya binary aktif belum tersentuh.

### 1.4 Kirim migrasi baru (hanya bila ada)

```bash
python vps/vps_sftp.py upload \
  D:/Code/SI-CENDIKIA/sip/si-cendikia/src/migrations/019_nama_migrasi.sql \
  /opt/si-cendikia/migrations/019_nama_migrasi.sql
```

Kirim **sebelum** restart: migrasi diterapkan otomatis saat service start.

### 1.5 Pasang & restart

```bash
python vps/vps_ssh.py "set -e
  TAG=\$(date +%Y%m%d-%H%M)-tag-perubahan
  sudo cp -a /opt/si-cendikia/si-cendikia-server /opt/si-cendikia/si-cendikia-server.bak.\$TAG
  install -m 0755 /opt/si-cendikia/si-cendikia-server.new /opt/si-cendikia/si-cendikia-server
  sudo systemctl restart si-cendikia
  sleep 2
  systemctl is-active si-cendikia
  curl -s http://127.0.0.1:18081/readyz
  journalctl -u si-cendikia -n 15 --no-pager | tail -15"
```

Konvensi nama cadangan yang sudah dipakai di server: `si-cendikia-server.bak.YYYYMMDD-<tag>`
(contoh nyata: `.bak.20260911-jabatan`, `.bak.20260909-admindinas`). Binary lama **jangan dihapus**
selama beberapa hari — itulah jalur rollback.
✅ Bentuk perintah blok ini sudah diuji (escaping `$TAG` lolos, menghasilkan `TAG=20260912-1911-tag-perubahan`).
⚠️ `systemctl restart si-cendikia` sendiri belum dijalankan pada sesi penulisan dokumen ini.

### 1.6 Verifikasi pasca-deploy ✅ (perintah sudah diuji, tanpa deploy)

```bash
python vps/vps_ssh.py "systemctl is-active si-cendikia
  curl -s http://127.0.0.1:18081/healthz; echo
  curl -s http://127.0.0.1:18081/readyz;  echo
  curl -s -o /dev/null -w 'beranda=%{http_code}\n' https://kgb.grobogankab.web.id/
  curl -s https://kgb.grobogankab.web.id/api/v1/public/stats | head -c 160; echo"
```

Hasil normal yang sudah terbukti: `active`, `{"data":{"status":"ok"}}`, `{"data":{"status":"ready"}}`,
`beranda=200`, statistik publik memuat `teachers_total`.

Uji fungsional satu akun (bukti bahwa login & role masih jalan):

```bash
python vps/vps_ssh.py "curl -s -o /tmp/l.json -w 'HTTP:%{http_code}\n' \
  -X POST https://kgb.grobogankab.web.id/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{\"username\":\"admin.disdik\",\"password\":\"<SANDI>\"}'; cat /tmp/l.json; rm -f /tmp/l.json"
```

### 1.7 Catat

Tambahkan baris keputusan di `docs/00-DECISIONS.md` (tanggal, perubahan, alasan, migrasi/endpoint,
cara uji) dan, bila perlu, tag git. Sertakan nama berkas cadangan binary yang dibuat pada langkah 1.5.

---

## 2. Rollback

```bash
python vps/vps_ssh.py "ls -t /opt/si-cendikia/si-cendikia-server.bak.* | head -5"   # pilih cadangan
python vps/vps_ssh.py "set -e
  sudo systemctl stop si-cendikia
  install -m 0755 /opt/si-cendikia/si-cendikia-server.bak.20260911-06xx-tag /opt/si-cendikia/si-cendikia-server
  sudo systemctl start si-cendikia
  systemctl is-active si-cendikia; curl -s http://127.0.0.1:18081/readyz"
```

**Batas rollback:** menurunkan binary **tidak** membatalkan migrasi yang sudah diterapkan.
Kalau perubahan menyentuh skema dan perilaku lama tidak kompatibel dengan skema baru, rollback
butuh skrip SQL pembalik tersendiri. Karena itu aturan penulisan migrasi: **aditif** (tambah tabel,
kolom nullable, atau kolom berdefault) — jangan menimpa/menghapus data.

---

## 3. Manajemen lewat dasbor aplikasi (tanpa SSH)

Masuk ke `https://kgb.grobogankab.web.id/login`, lalu aplikasi memilih tampilan berdasarkan role.

### 3.1 Dasbor Administrasi (role `admin`)

Terbuka otomatis setelah login sebagai `admin` (`/app`). Yang tersedia:

| Fitur di dasbor | Cara pakai | Efek |
|---|---|---|
| **Kelola akun** | tombol *Kelola akun* → tab **Admin / Guru / Unit / Dinas** + filter unit, kecamatan, tipe; kolom: username, nama, kelompok, role, unit, aktif | Buat akun petugas baru, ubah nama/role/unit/NIK/spesimen TTD/NIP/jabatan, ganti sandi, aktif/nonaktifkan akun |
| **Data ASN** | tombol *Lihat data ASN* + filter | Lihat master guru/pegawai (NIP, nama, PNS/PPPK, unit) — perubahan data mengikuti pengajuan KGB, bukan diedit manual |
| **Log aktivitas** | tombol *Log aktivitas* + filter aksi / pelaksana / rentang tanggal | Menelusuri siapa melakukan apa (impor, sinkron, verifikasi, koreksi) |
| **Skala gaji** | tombol *Skala gaji* | Lihat tabel gaji PNS & PPPK per golongan dan masa kerja |
| **Pola nomor surat** | form *Pola nomor* + pratinjau `{SEQ}`/`{YEAR}` | Menyetel format nomor SK; pola yang disimpan langsung aktif |
| **Pengumuman uji coba** | form banner (`aktif/tidak` + tanggal berakhir) | Menyalakan/mematikan banner "Masa Uji Coba Internal" di beranda |
| **Impor BKN** | unggah berkas Excel/CSV | Menambah/memperbarui master ASN; hasil ditampilkan: ditambah / diperbarui / dilewati / total |

### 3.2 Tampilan role lain (alur layanan KGB)

| Role | Tampilan | Yang dikerjakan |
|---|---|---|
| `asn` | Dasbor ASN | Ajukan KGB, unggah PDF, pantau status, unduh surat terbit |
| `verifikator_unit` | Antrean + **Pantauan Unit** + Riwayat + Nominasi KGB | Verifikasi usulan unitnya (Korwil sekecamatan / SMP / SKB) |
| `admin_dinas` / `verifikator_dinas` | Antrean Dinas + Riwayat + Nominasi KGB | Verifikasi tahap Dinas; `admin_dinas` dapat **Koreksi Data** di tahap `menunggu_dinas` |
| `pimpinan` | Antrean TTE | Tanda tangan elektronik / tolak konsep surat |

### 3.3 Yang belum punya tombol di dasbor (harus lewat API)

| Kebutuhan | Endpoint |
|---|---|
| Impor akun petugas (Excel/CSV) | `POST /api/v1/admin/import-users` |
| Sinkron data dari SIPP ASN | `POST /api/v1/admin/sync-sippasn`, `.../preview` (tanpa menulis), `GET .../history` |
| Impor skala gaji | `POST /api/v1/admin/salary-scales/import` |

Pola memanggil API secara manual dari VPS (semua mutasi wajib header CSRF):

```bash
python vps/vps_ssh.py "rm -f /tmp/ck
  curl -s -c /tmp/ck -X POST https://kgb.grobogankab.web.id/api/v1/auth/login \
    -H 'Content-Type: application/json' -d '{\"username\":\"admin.disdik\",\"password\":\"<SANDI>\"}' >/dev/null
  CSRF=\$(curl -s -b /tmp/ck https://kgb.grobogankab.web.id/api/v1/me | python3 -c 'import sys,json;print(json.load(sys.stdin)[\"data\"][\"csrf\"])')
  curl -s -b /tmp/ck -X PATCH https://kgb.grobogankab.web.id/api/v1/admin/users/<ID> \
    -H \"X-CSRF-Token: \$CSRF\" -H 'Content-Type: application/json' -d '{\"is_active\":false}'; echo
  rm -f /tmp/ck"
```

### 3.4 Resep harian (semua lewat dasbor)

| Kebutuhan | Langkah |
|---|---|
| Menambah verifikator unit baru | Login `admin` → **Kelola akun** → tab *Unit* → isi username (jangan sama dengan NIP), sandi awal, nama, unit kerja → simpan → minta petugas ganti sandi saat login pertama |
| Menambah admin dinas / pimpinan | Tab *Admin* / *Dinas* → isi username, sandi, nama. Untuk **pimpinan** wajib isi NIK + spesimen TTD; kalau kosong akun otomatis nonaktif |
| Ganti sandi petugas | **Kelola akun** → baris akun → ubah sandi → simpan |
| Mutasi/promosi petugas | Ubah *role* dan/atau *unit* pada baris akun. Role `verifikator_unit` wajib punya unit kerja |
| Menghentikan akses petugas (pensiun/mutasi) | Set *Aktif* = tidak. Sesi yang sedang berjalan langsung mati (`withAuth` membaca ulang status akun tiap request). Jangan hapus akun |
| Mengubah format nomor surat | Form *Pola nomor* → contoh `800/{SEQ}/4.2/{YEAR}` → pratinjau → simpan (langsung aktif) |
| Mengumumkan/mengakhiri masa uji coba | Form banner → centang aktif + tanggal berakhir → simpan |
| Impor data ASN (BKN) | Form *Impor BKN* → unggah Excel/CSV → periksa ringkasan "ditambah / diperbarui / dilewati". Bila "dilewati" besar, periksa kolom wajib pada berkas |
| Memantau pekerjaan petugas | **Log aktivitas** → filter pelaksana + rentang tanggal (mis. siapa menolak usulan pada tanggal tertentu) |

---

## 4. Aturan akun (dari kode & data produksi)

1. **Akun tidak bisa dihapus** — tidak ada endpoint delete, dan `audit_logs.actor_user_id`
   ber-FK ke `users(id)` tanpa `ON DELETE`. Kebijakan benar: `is_active=false`.
2. **Username tidak bisa diubah** (PATCH hanya nama/role/unit/sandi/NIK/TTD/NIP/jabatan).
   Kalau butuh nama akun yang rapi: buat akun baru → verifikasi login → nonaktifkan yang lama.
3. **Jangan biarkan nol admin aktif.** Tidak ada pengaman "admin terakhir"; satu-satunya guard di
   `store/admin.go` adalah untuk role `pimpinan` (wajib NIK + spesimen TTD, kalau kosong akun dibuat nonaktif).
4. **`BOOTSTRAP_ADMIN_*` hanya sekali pakai.** Bila kuncinya tetap ada di env, setiap restart
   service me-reset sandi dan mengaktifkan ulang akun tersebut (`store/bootstrap.go`).
   Env produksi sekarang tidak memuatnya — jangan ditambahkan untuk akun produksi jangka panjang.
5. Akun sisa (nonaktif) **sengaja dipertahankan** sebagai jejak audit: `admin.lokal-uji` (seed riwayat KGB,
   koreksi jalur Dinas), `admin.deploy-20260904` (sinkron SIPPASN 4 Sep), `provisioning-admin-20260816131145`
   (impor 99 akun petugas pertama).
6. Akun `asn` tidak dibuat manual — lahir dari impor BKN/sinkron SIPPASN (username = NIP).

---

## 5. Pemeriksaan rutin (read-only, aman dijalankan kapan saja)

```bash
python vps/vps_ssh.py "sudo -u postgres psql -d si_cendikia -P pager=off -c \"
  select role, count(*) filter (where is_active) as aktif, count(*) as total
  from users group by 1 order by 3 desc;\""
```

```sql
-- status usulan per tahap
select status, count(*) from submissions group by 1 order by 2 desc;
-- migrasi terakhir yang masuk
select version, applied_at from schema_migrations order by applied_at desc limit 5;
-- jejak terakhir
select id, actor_user_id, action, created_at from audit_logs order by id desc limit 20;
-- surat terbit bulan ini
select date_trunc('month', issued_at) as bulan, count(*) from letters group by 1 order by 1 desc limit 6;
```

---

## 6. Jebakan yang sudah terverifikasi

1. **Salinan sumber ganda — sudah dibereskan 2026-09-12.** Dulu `build-dan-jalankan.bat` menunjuk
   `D:\Code\SI-CENDIKIA\si-cendikia\src` yang beku di 2026-08-25 (git HEAD `604f882`), sehingga build
   dari sana menghasilkan kode lama. Kini semua skrip menunjuk `sip\si-cendikia\src` (HEAD aktif) dan
   salinan lama dipindahkan ke `arsip\si-cendikia-beku-20260825`. Skrip satu-pakai Agustus
   (`patch_*.py`, `fix*.py`, `.smoke_*.py`) ikut diarsipkan ke `arsip\patch-sesi-lama-20260821\`,
   dan rujukan di `local\e2e\*.py` serta `local\backfill-tmt-awal\terapkan-lokal.sql` sudah
   diarahkan ke salinan aktif.
2. **Port 8080 di VPS dipakai aplikasi lain** (`server`, ekgb). SI CENDIKIA di `127.0.0.1:18081`.
3. **Audit tidak lengkap.** `handleAdminUpdateUser` (`internal/httpapi/admin_handlers.go`) tidak menulis
   audit sama sekali — perubahan akun (nonaktifkan, ganti role, reset sandi) tidak berjejak. Hanya
   pembuatan akun lewat API yang menulis `buat_pengguna`.
4. **IP pengguna tidak tercatat.** `clientIP()` memakai `r.RemoteAddr` tanpa `X-Forwarded-For`; karena
   app di belakang nginx, semua baris audit ber-IP `127.0.0.1` (2.162 dari 2.181 baris). Penanda non-IP
   yang ada: kosong (skrip), `scheduler` (sync malam), `vps-deploy` (skrip deploy).
5. **Dokumen `deploy/DEPLOYMENT.md` (2026-08-25) sudah usang**: menyebut port 8080, user service
   `si-cendikia`, `/var/lib/si-cendikia/files` sebagai usulan, dan daftar role tanpa `admin_dinas`.
   Pakai dokumen ini sebagai acuan operasional.
6. **Postgres lokal** (`sip/si-cendikia/pgdata`, port 5433) sering tidak jalan; `build-dan-jalankan.bat`
   memakai DB 5432 di `C:\Users\Acer\pgsql`. Untuk uji lokal, pastikan dulu mana yang hidup.

---

## 7. Berkas rujukan

- `deploy/nginx-kgb.grobogankab.web.id.conf` — vhost produksi yang dipakai.
- `deploy/si-cendikia.service.example`, `deploy/si-cendikia.env.example` — template service/env.
- `deploy/backup-si-cendikia.example.sh`, `deploy/restore-si-cendikia.example.sh` — contoh backup/restore.
- `docs/00-DECISIONS.md` — riwayat keputusan produk & teknis (basis "mengapa" setiap perubahan).
- `docs/03-API-DOCUMENTATION/API.md` — daftar endpoint (matriks role-nya masih 5 kolom, belum memuat
  `admin_dinas`).
- `vps/vps_ssh.py`, `vps/vps_sftp.py` — akses VPS dari workstation.
