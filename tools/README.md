# Local E2E utilities

`local_esign_gateway.py` adalah gateway dummy **loopback-only** untuk pengujian E2E. Gateway ini tidak menghubungi eSign Kominfo dan tidak boleh diekspos melalui Nginx atau domain publik.

Kontrak yang didukung:

- `POST /api/v2/sign/pdf`
- `GET /api/sign/download/{receipt}`
- Basic Auth wajib.
- Input harus PDF dengan magic bytes `%PDF-`.
- Output diberi marker `LOCAL-DUMMY-SIGNED-BY-GATEWAY`.

`test_local_esign_gateway.py` menjalankan test kontrak tanpa database dan tanpa credential production.

Untuk E2E live sementara, gateway dijalankan sebagai unit systemd terpisah dengan listen `127.0.0.1` dan harus dihentikan setelah pengujian. Jangan menyimpan nilai `LOCAL_ESIGN_PASSWORD` di repository.
