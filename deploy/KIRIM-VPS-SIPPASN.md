# Kirim ke VPS — rilis sinkron SIPPASN (2026-09-03)

## Isi paket (di folder ini)
- `deploy/si-cendikia-server-linux-amd64` — biner Linux (ELF x86-64, Go, static).
  Salin ke `/opt/si-cendikia/si-cendikia-server` (ganti nama saat salin).
- `src/migrations/011_sync_sippasn.sql`, `012_kategori_pegawai.sql` — migrasi baru
  (011: `bkn_imports.asal_data`; 012: `teachers.kategori`).
- `src/data/seeds/001_salary_scales.sql` — skala gaji (sudah ada; pastikan termuat).
- `deploy/si-cendikia.env.example` — tambahan env SIPPASN (contoh).

## Langkah di VPS (setelah salin biner + migrasi)
1. Hentikan service: `sudo systemctl stop si-cendikia`
2. Ganti biner + salin migrasi 011–012 ke `/opt/si-cendikia/migrations/`
3. Tambahkan ke `/etc/si-cendikia/si-cendikia.env` (opsional, default sudah jalan):
   `SIPPASN_NIGHTLY_SYNC=true` (default true), `SIPPASN_SNAPSHOT_TTL=6h`
4. Jalankan service: migrasi 011–012 teraplikasi otomatis saat start.
   Pastikan seed gaji termuat (lihat DEPLOYMENT.md §5):
   `sudo -u postgres psql -d si_cendikia -f /opt/si-cendikia/data/seeds/001_salary_scales.sql`
5. Seeding awal (dari PC admin / curl VPS):
   `POST /api/v1/admin/sync-sippasn/preview` dulu (cek hitungan),
   lalu `POST /api/v1/admin/sync-sippasn` (tanpa limit).
   Ekspektasi: ~8.900 pegawai Disdik (guru + non-guru).
6. Verifikasi: `GET /api/v1/admin/sync-sippasn/history`, cek log
   `journalctl -u si-cendikia | grep -i sippasn`.

## Catatan
- Sinkron malam aktif default (tiap 24 jam). SIPPASN menang untuk data induk;
  kolom KGB (masa kerja terbit, TMT) tidak ditimpa.
- PNS: golongan fix SIPPASN. PPPK: golongan I–XVII dipilih sesuai SK saat usul KGB.
- 2 test gagal bawaan (`TestMeSkalaTidakAda`, `TestNominasi...`) gagal juga di
  HEAD murni — tidak terkait rilis ini, jangan dijadikan penahan kirim.
