#!/usr/bin/env python3
"""Generator seed skala gaji SI CENDIKIA.

Mentranskripsi otomatis seeder e-KGB (read-only):
  /var/www/ekgb/app/Database/Seeds/SalaryTableSeeder.php
menjadi SQL idempoten untuk tabel salary_scales.

Sumber angka (per komentar seeder e-KGB): transkripsi Lampiran
PP 5/2024 (PNS) dan Perpres 11/2024 (PPPK) dari
docs/referensi-persyaratan-kgb.md Bagian 9.

Jalankan: python3 src/data/seeds/tools/generate_salary_seed.py
"""
import re
from pathlib import Path

SRC = Path("/var/www/ekgb/app/Database/Seeds/SalaryTableSeeder.php")
OUT = Path(__file__).resolve().parent.parent / "001_salary_scales.sql"

ROW = re.compile(r"\['(PNS|PPPK)',\s*'([^']+)',\s*(\d+),\s*(\d+),\s*'([^']+)'\]")


def main() -> None:
    rows = ROW.findall(SRC.read_text())
    if len(rows) < 500:
        raise SystemExit(f"hanya {len(rows)} baris terbaca — seeder sumber berubah? berhenti, jangan tebak.")

    header = [
        "-- 001_salary_scales.sql — seed skala gaji pokok SI CENDIKIA",
        "-- DIBUAT OLEH: tools/generate_salary_seed.py (jangan edit manual).",
        "-- SUMBER ANGKA: /var/www/ekgb/app/Database/Seeds/SalaryTableSeeder.php,",
        "--   yaitu transkripsi Lampiran PP 5/2024 (PNS) & Perpres 11/2024 (PPPK)",
        "--   dari docs/referensi-persyaratan-kgb.md Bagian 9 (sistem e-KGB, read-only).",
        "-- Idempoten: aman dijalankan berulang. Jalankan sekali setelah migrasi.",
        "",
        "INSERT INTO salary_scales (asn_type, golongan, masa_kerja_tahun, gaji) VALUES",
    ]

    values = []
    dasar_hukum = set()
    for jenis, gol, mkg, gaji, dasar in rows:
        dasar_hukum.add(f"{jenis}={dasar}")
        values.append(f"('{jenis.lower()}', '{gol}', {mkg}, {gaji})")

    body = ",\n".join(values)
    footer = "\nON CONFLICT (asn_type, golongan, masa_kerja_tahun) DO NOTHING;\n"

    OUT.write_text("\n".join(header) + body + footer + f"-- total: {len(rows)} baris ({', '.join(sorted(dasar_hukum))})\n")
    print(f"OK: {len(rows)} baris -> {OUT}")


if __name__ == "__main__":
    main()
