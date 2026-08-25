# Panduan Deploy SI CENDIKIA ke VPS Produksi

## Ringkasan Status Saat Ini (2026-08-25)

| Komponen | Status | Catatan |
|---|---|---|
| Domain | ✅ kgb.grobogankab.web.id hidup | Server lokal teruji, siap cutover penuh |
| Kode sumber | ✅ Final (setelah rumus KGB seragam) | Commit `feat: satu rumus jadwal KGB, anti-lompat` tersedia |
| Artifacts | ✅ Build Windows & Linux binary | Binary di `.builds/`, script backfill ready |
| Database | ✅ Seed BKN 7.301 guru, 6.865 sudah KGB tercatat | Backfill `tmt_awal` & `seed_riwayat_kgb` siap dijalankan |
| Checklist | ⚠️ Sisa 3 item belum selesai | Backup drill, TLS/Nginx final, TTE dokumen verifikasi |

---

## Prasyarat di VPS

- PostgreSQL 16 (sudah ada dari e-KGB live)
- Go 1.22+ atau transfer binary yang sudah built
- Nginx + SSL cert aktif
- Akses sudo untuk systemd service
- Folder `/opt/si-cendikia` & `/var/lib/si-cendikia/files` (direktori file privat PDF)
- Backup ruang kosong minimal 2 GB untuk growth

---

## Langkah Deploy (Production)

### 1. Build/Bawa Binary

**Opsi A – Build langsung di VPS:**
```bash
sudo apt update && sudo apt install -y golang-go

cd ~/si-cendikia/src
go build -ldflags="-s -w" -o ../deploy/si-cendikia-server ./cmd/server
ls -lh ../deploy/si-cendikia-server  # ~23 MB
```

**Opsi B – Copy dari workstation:**
```bash
# Di workstation (Windows)
cd D:\Code\SI-CENDIKIA\si-cendikia\src
# Build cross-platform
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../deploy/si-cendikia-server-linux-amd64 ./cmd/server

# Upload ke VPS (via scp/WinSCP)
scp ../deploy/si-cendikia-server-linux-amd64 <user>@<vps-host>:~/si-cendikia/deploy/si-cendikia-server
```

### 2. Siapkan Direktori & Izin
```bash
sudo groupadd si-cendikia || true
sudo useradd -r -g si-cendikia si-cendikia

sudo install -d -o si-cendikia -g si-cendikia -m 0750 /opt/si-cendikia /var/lib/si-cendikia/files

sudo cp ~/si-cendikia/src/../deploy/si-cendikia-server /opt/si-cendikia/
sudo chown root:si-cendikia /opt/si-cendikia/si-cendikia-server

cp ~/si-cendikia/src/../migrations/*.sql /opt/si-cendikia/migrations/
sudo chown -R si-cendikia:si-cendikia /opt/si-cendikia
```

### 3. Install Service Systemd
Salin template (`deploy/si-cendikia.service.example`) menjadi `/etc/systemd/system/si-cendikia.service`:
```text
[Unit]
Description=SI CENDIKIA — Sistem KGB ASN
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=si-cendikia
Group=si-cendikia
WorkingDirectory=/opt/si-cendikia
ExecStart=/opt/si-cendikia/si-cendikia-server
Restart=on-failure
RestartSec=5s

EnvironmentFile=/etc/si-cendikia/si-cendikia.env

LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

Aktifkan service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now si-cendikia
sudo systemctl status si-cendikia   # harus active (running)
journalctl -u si-cendikia -f        # pantau log startup
```

### 4. Konfigurasi Environment

Buat `/etc/si-cendikia/si-cendikia.env` (jangan dicommit!):
```ini
DATABASE_URL=postgresql://<app-db-user>:<app-db-pass>@localhost:5432/si_cendikia?sslmode=require
SESSION_SECRET=<generate random 32+ char string via openssl/rand>
ADDR=127.0.0.1:8080
SECURE_COOKIE=true
MIGRATIONS_DIR=/opt/si-cendikia/migrations
FILE_ROOT=/var/lib/si-cendikia/files

# eSign Kominfo (prod)
ESIGN_BASE_URL=https://eservice.kominfo.go.id/api/v2/sign  # sesuaikan endpoint prod Dinas
ESIGN_USERNAME=<username eSign Dinas>
ESIGN_PASSWORD=<password eSign Dinas>
```

Setelah diedit → reload service:
```bash
sudo systemctl restart si-cendikia
sudo journalctl -u si-cendikia -n 100
```

Harusnya:
```
healthz: {"data":{"status":"ok"}}
readyz : {"data":{"status":"ready"}}
```

Verifikasi:
```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
curl https://kgb.grobogankab.web.id/api/v1/public/stats
```

### 5. Migrasi Basis Data (sekali saat launch)

Aplikasi menjalankan migrasi otomatis saat start-up pertama kali.
Jika ingin manual:
```bash
sudo -u si-cendikia psql -d si_cendikia -f /opt/si-cendikia/migrations/001_schema.sql
sudo -u si-cendikia psql -d si_cendikia -f /opt/si-cendikia/migrations/002_application_features.sql
... sampai 010_tmt_awal.sql
```

### 6. Impor Skala Gaji & Data BKN

Seed skala gaji (run sekali):
```bash
sudo -u postgres psql -d si_cendikia -f /opt/si-cendikia/data/seeds/001_salary_scales.sql
```

Backfill `tmt_awal` dari file BKN resmi Dinas (yang dipakai untuk seeding awal):
```bash
# Generate CSV dulu (di server/workstation, menggunakan python openpyxl sama seperti skrip lokal)
python3 backfill-tmt-awal/buat_csv_backfill.py /path/to/file-BKN-Dinas.xlsx > /tmp/tmt_awal.csv

# Terapkan
sudo -u postgres psql -d si_cendikia -v csv_path="'/tmp/tmt_awal.csv'" \
    -f backfill-tmt-awal/terapkan-backfill.sql
```

Seed riwayat KGB (isi `tmt_kgb_last` hingga Sep 2026):
```bash
# Pastikan tmt_awal sudah diisi sebelumnya!
sudo -u postgres psql -d si_cendikia -v batas="'2026-09-30'" \
    -v actor_user="admin.disdik1" -f seed-riwayat-kgb/terapkan-backfill.sql
```

Cek hasil:
```bash
sudo -u postgres psql -d si_cendikia -P pager=off -c "SELECT role, COUNT(*) FROM users WHERE is_active GROUP BY role;"
sudo -u postgres psql -d si_cendikia -P pager=off -c "SELECT 'total_guru',COUNT(*)::text FROM teachers UNION ALL SELECT 'tmt_awal_terisi',COUNT(*) FROM teachers WHERE tmt_awal IS NOT NULL UNION ALL SELECT 'tmt_kgb_last_terisi',COUNT(*) FROM teachers WHERE tmt_kgb_last IS NOT NULL;"
```

### 7. Konfigurasi Nginx & SSL

Copy template konfigurasi (edit sesuai hostname):
```bash
sudo nano /etc/nginx/sites-available/si-cendikia
```

Contoh isi:
```nginx
server {
    listen 80;
    listen [::]:80;
    server_name kgb.grobogankab.web.id;

    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name kgb.grobogankab.web.id;

    ssl_certificate /etc/letsencrypt/live/kgb.grobogankab.web.id/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/kgb.grobogankab.web.id/privkey.pem;

    client_max_body_size 7m;

    location /static/ {
        alias /opt/si-cendikia/static/;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection upgrade;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_cache_bypass $http_cookie;
        proxy_no_cache $http_cookie;
    }
}
```

Test & enable site:
```bash
sudo nginx -t
sudo ln -sf /etc/nginx/sites-available/si-cendikia /etc/nginx/sites-enabled/
sudo systemctl reload nginx
```

### 8. Verifikasi Pasca Deploy

Halaman publik tanpa login:
```bash
curl -sS https://kgb.grobogankab.web.id/ | grep -q "SI CENDIKIA" && echo "✅ Beranda OK"
curl -sS https://kgb.grobogankab.web.id/panduan
curl -sS https://kgb.grobogankab.web.id/alur
```

Endpoint API publik:
```bash
curl -sS https://kgb.grobogankab.web.id/api/v1/public/stats | jq '.data'
```

Login petugas uji (login sebagai akun asli Dinas):
```
Username: admin.disdik1
Password: (lihat daftar akun resmi di local/akun-admin — pemilik saja)
```

Laporan Nominasi KGB:
```
Menu → Administrasi → Laporan → Nominasi KGB
```

Periksa keranjang distribusi (harusnya ~sama dengan hasil lokal):
- belum_lengkap: ≈ 129
- terlambat: 0 (setelah seed riwayat)
- segera: ≈ 24
- nominasi: ≈ 1610
- mendatang: ≈ 5538

Jurnal aplikasi (monitor logs):
```bash
sudo journalctl -u si-cendikia -f
```

### 9. Setup Backup Berkala (CRON)

Tambahkan ke crontab `si-cendikia` user atau system cron:
```bash
# Backup DB harian jam 2 pagi, retensi 30 hari
0 2 * * * pg_dump -h localhost -U postgres -d si_cendikia | bzip2 > /backup/kgb-si-cendikia-$(date +\%Y\%m\%d).bz2
find /backup -name 'kgb-si-cendikia-*.bz2' -mtime +30 -delete

# Sync file PDF ke storage eksternal (rsync ke NAS/cloud)
rsync -avz /var/lib/si-cendikia/files/ backupuser@nas:/krankenhaus-si-cendikia/files/
```

### 10. Rollback Plan (jika terjadi masalah)

Stop service:
```bash
sudo systemctl stop si-cendikia
```

Restore dari backup terakhir:
```bash
pg_restore -d si_cendikia < /backup/restore-database-latest.bz2
```

Kembalikan binary versi sebelumnya (jika diperlukan):
```bash
mv /opt/si-cendikia/si-cendikia-server /opt/si-cendikia/si-cendikia-server.new
mv /opt/si-cendikia/si-cendikia-server-v1.old /opt/si-cendikia/si-cendikia-server
sudo systemctl start si-cendikia
```

Catat: semua perubahan data (`teachers.tmt_awal`, `tmt_kgb_last`, `submissions`, `letters`, audit_logs) permanen — restore hanya mengembalikan ke titik backup sebelum perubahan.

---

## Checklist Akhir Sebelum Go-Live Penuh

1. [ ] Backup database saat ini berhasil (test restore terlebih dahulu)
2. [ ] Credentials eSign Kominfo produksi dipasang di `.env`
3. [ ] Akun pimpinan divalidasi (NIK, TTD image base64 tersedia di database/user table)
4. [ ] Dokumen PDF hasil uji coba TTE diverifikasi oleh Kepala Dinas/Kominfo
5. [ ] Monitoring dashboard (grafik online/offline, error log alerting) disiapkan
6. [ ] Tim IT Dinas mendapat akses jurnal logs (via `journalctl`)
7. [ ] Dokumentasi SOP operasional (cara input pengajuan baru, cara cek laporan, prosedur troubleshooting sederhana) diserahkan

---

## Kontak Teknis

Owner: Chamdani (Dinas Pendidikan Grobogan)
Development contact: tim teknis Dinas/Pengembang internal

---

**Catatan Penting**:
- File `.env`, `local/akun-admin/daftar-akun-petugas.xlsx` (termasuk password), dan arsip kredensial eSign **TIDAK PERNAH** dimasukkan ke Git repository. Simpan di tempat aman dengan enkripsi.
- Script backfill CSV (`buat_csv_backfill.py`) boleh dijalankan ulang kapan pun jika perlu refresh dari file BKN terbaru.
- Audit trail mencatat semua aksi penting (`seed_riwayat_kgb`, `impor_bkn`, `submit`, `tte`, dsb.) — gunakan untuk investigasi jika ada discrepancy.

---

Dokumen ini dibuat berdasarkan keputusan owner 2026-08-25 dan test lokal yang sukses. Silakan jalankan langkah-langkah di atas pada server produksi setelah mendapatkan otorisasi penuh.
