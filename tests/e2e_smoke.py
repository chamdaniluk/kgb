#!/usr/bin/env python3
"""Smoke/E2E check for a locally running SI CENDIKIA instance."""
from __future__ import annotations

import os
import shutil
from pathlib import Path

from playwright.sync_api import sync_playwright

BASE_URL = os.getenv("SI_CENDIKIA_BASE_URL", "http://127.0.0.1:18080")
USERNAME = os.getenv("SI_CENDIKIA_E2E_USERNAME", "198001012005011001")
PASSWORD = os.getenv("SI_CENDIKIA_E2E_PASSWORD", USERNAME)


def main() -> None:
    executable = os.getenv("CHROMIUM_BIN") or shutil.which("chromium")
    if not executable:
        raise SystemExit("chromium tidak ditemukan")

    errors: list[str] = []
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True, executable_path=executable, args=["--no-sandbox"])
        context = browser.new_context()
        page = context.new_page()
        page.on("console", lambda message: errors.append(f"console {message.type}: {message.text}") if message.type == "error" else None)
        page.on("pageerror", lambda error: errors.append(f"pageerror: {error}"))

        page.goto(BASE_URL + "/", wait_until="networkidle")
        assert "Kenaikan Gaji Berkala ASN" in page.locator("h1").inner_text()
        assert page.locator("a[href='/login']").count() >= 1

        page.goto(BASE_URL + "/login", wait_until="networkidle")
        page.get_by_label("Username").fill(USERNAME)
        page.get_by_label("Password").fill(PASSWORD)
        page.get_by_role("button", name="Masuk").click()
        page.wait_for_url("**/app", timeout=10_000)
        page.wait_for_load_state("networkidle")
        assert "Dasbor Guru" in page.locator("h1").inner_text()
        assert "Guru E2E" in page.locator("body").inner_text()
        assert "Rp 3.497.300" in page.locator("body").inner_text()
        assert "Rp 3.607.500" in page.locator("body").inner_text()

        page.locator("input[name='proposed_tmt']").fill("2026-09-01")
        page.locator("input[name='file']").set_input_files({
            "name": "dukungan.pdf",
            "mimeType": "application/pdf",
            "buffer": b"%PDF-1.4\n% E2E\n",
        })
        page.get_by_role("button", name="Kirim pengajuan").click()
        page.wait_for_timeout(800)
        body = page.locator("body").inner_text()
        assert "Pengajuan berhasil dikirim." in body or "Pengajuan #" in body
        assert "Menunggu Verifikasi Unit" in body

        if errors:
            raise AssertionError("JavaScript errors: " + "; ".join(errors))
        print("E2E PASS: beranda -> login -> dashboard guru -> submit PDF")
        browser.close()


if __name__ == "__main__":
    main()
