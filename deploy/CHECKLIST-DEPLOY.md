# Check-list Deploy Cepat ke VPS Produksi

## Prasyarat yang Sudah Selesai ✅

- [x] Commit kode final: `604f882` - "feat: satu rumus jadwal KGB anti-lompat siklus + cap gaji puncak"
- [x] Binary Windows ready: `src/si-cendikia-server.exe` (23 MB) untuk testing lokal
- [x] **Binary Linux siap**: `deploy/si-cendikia-server-linux-amd64` (16.4 MB) untuk VPS
- [x] Script backfill tmt_awal: `local/backfill-tmt-awal/`
- [x] Script seed riwayat KGB: `local/seed-riwayat-kgb/`
- [x] Dokumentasi lengkap: `deploy/DEPLOYMENT.md`

---

## Langkah di VPS (Copy-Paste Commands)

### 1. Login & Setup Environment

```bash
# Jalankan login dari desktop shortcut atau manually:
ssh <username>@<vps-host>

# Install Go jika belum ada
sudo apt update && sudo apt install -y golang-go

# Pastikan versi Go 1.22+
go version
```

### 2. Siapkan Directory & User Service

```bash
cd ~/si-cendikia/src

# Buat user service si-cendikia
sudo groupadd si-cendikia || true
sudo useradd -r -g si-cendikia si-cendikia

# Buat direktori
sudo install -d -o si-cendikia -g si-cendikia -m 0750 /opt/si-cendikia /var/lib/si-cendikia/files

# Salin binary (build manual di server)
go build -ldflags="-s -w" -o ../deploy/si-cendikia-server ./cmd/server

# Atau upload dari workstation via scp:
# scp ../deploy/si-cendikia-server-linux-amd64 <user>@<vps-host>:~/si-cendikia/deploy/si-cendikia-server

sudo cp ../deploy/si-cendikia-server /opt/si-cendikia/
sudo chown root:si-cendikia /opt/si-cendikia/si-cendikia-server

# Salin migrations
sudo cp -r ../../migrations/ /opt/si-cendikia/migrations/
sudo chown -R si-cendikia:si-cendikia /opt/si-cendikia
```

### 3. Instal Systemd Service

```bash
sudo nano /etc/systemd/system/si-cendikia.service
```

Paste isi ini:
```ini
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

Aktifkan & start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now si-cendikia
sudo systemctl status si-cendikia   # Harus active (running)
```

Log monitoring:
```bash
sudo journalctl -u si-cendikia -f
```

### 4. Konfigurasi Environment (.env)

```bash
sudo nano /etc/si-cendikia/si-cendikia.env
```

Isi dengan nilai produksi (JANGAN commit!):
```ini
DATABASE_URL=postgresql://<app-db-user>:<app-db-pass>@localhost:5432/si_cendikia?sslmode=require
SESSION_SECRET=<generate random 32+ char string via openssl rand-base64 | head -c 32>
ADDR=127.0.0.1:8080
SECURE_COOKIE=true
MIGRATIONS_DIR=/opt/si-cendikia/migrations
FILE_ROOT=/var/lib/si-cendikia/files

# eSign Kominfo PRODUCTION (isi sesuai kredensial Dinas)
ESIGN_BASE_URL=https://eservice.kominfo.go.id/api/v2/sign
ESIGN_USERNAME=<username_eSign_prod>
ESIGN_PASSWORD=<password_eSign_prod>
```

Restart service setelah diedit:
```bash
sudo systemctl restart si-cendikia
sudo journalctl -u si-cendikia -n 50
```

Verifikasi health check:
```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
```

Harusnya output:
```json
{"data":{"status":"ok"}}
{"data":{"status":"ready"}}
```

### 5. Impor Skala Gaji & Data BKN

Seed skala gaji (run sekali):
```bash
sudo -u postgres psql -d si_cendikia -f /opt/si-cendikia/data/seeds/001_salary_scales.sql
```

Backfill `tmt_awal` dari file BKN resmi Dinas:
```bash
# Upload file BKN ke server dan generate CSV
python3 ~/si-cendikia/local/backfill-tmt-awal/buat_csv_backfill.py /path/to/file-BKN-Dinas.xlsx > /tmp/tmt_awal.csv

# Terapkan
sudo -u postgres psql -d si_cendikia -v csv_path="'/tmp/tmt_awal.csv'" \
    -f ~/si-cendikia/local/backfill-tmt-awal/terapkan-backfill.sql

# Verifikasi hasil
sudo -u postgres psql -d si_cendikia -P pager=off -c "SELECT 'total_guru',COUNT(*)::text FROM teachers UNION ALL SELECT 'tmt_awal_terisi',COUNT(*) FROM teachers WHERE tmt_awal IS NOT NULL;"
```

Expected output:
```
total_guru      | 7301
tmt_awal_terisi | 7172
```

Seed riwayat KGB:
```bash
sudo -u postgres psql -d si_cendikia -v batas="'2026-09-30'" \
    -v actor_user="admin.disdik1" -f ~/si-cendikia/local/seed-riwayat-kgb/terapkan-backfill.sql

# Verifikasi
sudo -u postgres psql -d si_cendikia -P pager=off -c "SELECT 'tmt_kgb_last_terisi',COUNT(*) FROM teachers WHERE tmt_kgb_last IS NOT NULL;"
```

Expected: ~6.865 guru

### 6. Nginx & SSL Configuration

Edit konfigurasi Nginx:
```bash
sudo nano /etc/nginx/sites-available/si-cendikia
```

Pastikan:
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

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Enable & test:
```bash
sudo ln -sf /etc/nginx/sites-available/si-cendikia /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### 7. Final Verification

Cek API publik tanpa login:
```bash
curl -sS https://kgb.grobogankab.web.id/ | grep -q "SI CENDIKIA" && echo "✅ Beranda OK"
curl -sS https://kgb.grobogankab.web.id/api/v1/public/stats | jq '.data'
```

Login admin (gunakan akun asli dari daftar dinas):
```
Username: admin.disdik1  (atau admin lainnya)
Password: (lihat local/akun-admin/daftar-akun-petugas.xlsx - pemilik saja)
```

Kunjungi halaman laporan nominasi:
http://kgb.grobogankab.web.id/app?tab=nominations

Periksa keranjang:
- belum_lengkap ≈ 129
- terlambat = 0 ✅
- segera ≈ 24
- nominasi ≈ 1610
- mendatang ≈ 5538

Setup backup cron:
```bash
crontab -e

# Backup DB harian jam 2 pagi, retensi 30 hari
0 2 * * * pg_dump -h localhost -U postgres -d si_cendikia | bzip2 > /backup/kgb-si-cendikia-$(date +\%Y\%m\%d).bz2
find /backup -name 'kgb-si-cendikia-*.bz2' -mtime +30 -delete
```

---

## Checklist Akhir

Setelah semua selesai:

- [ ] Backup database berhasil (test restore ke DB terpisah)
- [ ] HTTPS/SSL valid (cek browser → padlock icon)
- [ ] Audit trail tercatat (menu Riwayat kegiatan)
- [ ] Login petugas berfungsi dengan benar
- [ ] Laporan Nominasi menampilkan data lengkap
- [ ] Monitoring aktif (grafik online/offline dashboard)
- [ ] SOP operasional didokumentasikan untuk tim Dinas

**Selamat! SI CENDIKIA sudah live produksi.** 🎉

---

## Kontak Darurat

Jika terjadi issue:
1. Cek logs: `sudo journalctl -u si-cendikia -f`
2. Test health: `curl http://127.0.0.1:8080/healthz`
3. Rollback: stop service, restore backup DB, kembalikan binary versi sebelumnya

Owner: Chamdani (Dinas Pendidikan Grobogan)
Development support: tim teknis Dinas/Internal
