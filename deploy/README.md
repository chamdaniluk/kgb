# Deployment SI CENDIKIA

Artefak di folder ini adalah **contoh konfigurasi**, bukan deploy otomatis.
Tidak ada file live atau credential produksi di repository.

> **Status 2026-08-25: cutover produksi telah dilakukan owner — SI CENDIKIA berjalan di `kgb.grobogankab.web.id`.** Item checklist go-live di bawah yang belum dicentang tetap harus diselesaikan atau dikonfirmasi.

## Prasyarat produksi

- PostgreSQL 16 tersedia.
- Database `si_cendikia` dan user aplikasi dibuat oleh administrator.
- Go binary dibangun pada pipeline/build host.
- Renderer PDF tersedia dan dapat menulis output pada runtime service. WeasyPrint
  direkomendasikan pada host yang memakai Chromium Snap, karena Chromium Snap
  dapat mengembalikan exit code sukses tetapi gagal membuat file PDF pada path
  service. Renderer wajib diuji sebagai user service, bukan hanya dari shell admin.
- eSign Kominfo dikonfigurasi sesuai kontrak resmi dan kredensial Dinas.
- DNS dan sertifikat TLS sudah siap.
- Direktori file privat berada di luar document root.

## Konfigurasi

1. Salin `si-cendikia.env.example` ke `/etc/si-cendikia/si-cendikia.env`.
2. Isi `DATABASE_URL`, `SESSION_SECRET`, `FILE_ROOT`, dan parameter eSign melalui secret manager.
3. Jangan commit file hasil pengisian.
4. Jika menggunakan bootstrap admin, isi `BOOTSTRAP_ADMIN_*` hanya saat provisioning awal, lalu hapus.

## Instalasi binary/service

```text
install -d -o si-cendikia -g si-cendikia -m 0750 /opt/si-cendikia /var/lib/si-cendikia/files
install -o root -g si-cendikia -m 0750 si-cendikia-server /opt/si-cendikia/si-cendikia-server
install -o root -g root -m 0644 deploy/si-cendikia.service.example /etc/systemd/system/si-cendikia.service
systemctl daemon-reload
systemctl enable --now si-cendikia
```

Sesuaikan `User`, `Group`, path, dan izin filesystem dengan kebijakan server. Perintah di atas hanya prosedur referensi dan **belum dijalankan oleh agent**.

## Nginx/TLS

1. Ganti hostname contoh dalam `nginx-si-cendikia.example.conf`.
2. Pastikan upstream `127.0.0.1:8080` cocok dengan `ADDR`.
3. Pasang sertifikat TLS.
4. Jalankan `nginx -t`, lalu reload sesuai prosedur administrator.

Limit upload harus konsisten:

```text
Nginx client_max_body_size: 7m
Aplikasi PDF maksimal: 5MB
```

## Migrasi dan seed

Aplikasi menjalankan migration SQL saat startup. Seed skala gaji dijalankan sekali oleh administrator setelah migration:

```text
psql "$DATABASE_URL" -f data/seeds/001_salary_scales.sql
```

## Backup dan restore drill

- Jadwalkan `backup-si-cendikia.example.sh` melalui cron/systemd timer.
- Simpan dump database dan direktori PDF pada media berbeda dari VPS.
- Uji `restore-si-cendikia.example.sh` ke database dan folder terisolasi secara berkala.
- Jangan menyatakan backup valid hanya karena file dump terbentuk. Jalankan restore dan probe `/readyz`.

## Health check

```text
GET /healthz  # proses HTTP hidup
GET /readyz   # database dapat diping
```

## Batas eksekusi sesi ini

Agent telah memverifikasi build/test/smoke pada environment lokal dan membuat template deployment. Agent **tidak menjalankan** systemctl, Nginx reload, perubahan DNS/TLS, pembuatan database produksi, atau deployment ke domain live tanpa instruksi deploy eksplisit dari owner.

## Verifikasi sebelum go-live

- [ ] `.env` produksi berada di secret manager.
- [x] Username petugas tidak sama dengan NIP.
- [x] Password petugas sudah di-hash saat import.
- [x] Akun pimpinan memiliki NIK, spesimen TTD, dan akses eSign yang tervalidasi.
- [ ] Backup + restore drill berhasil.
- [ ] TLS, HSTS, Nginx, dan firewall diperiksa.
- [x] E2E semua role dijalankan pada fixture live terisolasi dan dibersihkan.
- [ ] Seed BKN resmi Dinas diimpor dan ringkasannya diperiksa.
- [ ] Dokumen PDF hasil TTE diverifikasi oleh pihak Dinas.
- [x] Owner menyetujui cutover domain `kgb.grobogankab.web.id`.
- [x] Cutover produksi telah dilakukan, server berjalan live.
- [ ] Official BKN seed dan credential eSign Kominfo dipasang oleh administrator (setelah backfill & seed riwayat KGB selesai).
