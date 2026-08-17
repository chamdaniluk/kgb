#!/usr/bin/env python3
"""Local-only dummy eSign gateway for isolated E2E testing.

This service deliberately does not contact the official eSign provider. It accepts
an already-rendered PDF, validates its magic bytes, stores a marked copy, and
returns a local dummy receipt. It must be bound to loopback only.
"""

from __future__ import annotations

import argparse
import base64
import binascii
import hashlib
import hmac
import json
import os
import secrets
import signal
import sys
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import unquote, urlparse

MAX_BODY = 15 * 1024 * 1024
MAX_PDF = 10 * 1024 * 1024
MARKER = b"\n% LOCAL-DUMMY-SIGNED-BY-GATEWAY\n"


class GatewayState:
    def __init__(self, store: Path, username: str, password: str):
        self.store = store
        self.username = username
        self.password = password
        self.store.mkdir(parents=True, exist_ok=True)
        os.chmod(self.store, 0o700)

    def save(self, content: bytes) -> str:
        receipt = "local-dummy-" + secrets.token_urlsafe(18)
        target = self.store / (hashlib.sha256(receipt.encode()).hexdigest() + ".pdf")
        flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
        fd = os.open(target, flags, 0o600)
        try:
            with os.fdopen(fd, "wb") as handle:
                handle.write(content)
                handle.flush()
                os.fsync(handle.fileno())
        except Exception:
            try:
                target.unlink()
            except FileNotFoundError:
                pass
            raise
        (self.store / (target.stem + ".receipt")).write_text(receipt, encoding="ascii")
        os.chmod(self.store / (target.stem + ".receipt"), 0o600)
        return receipt

    def load(self, receipt: str) -> bytes | None:
        if not receipt.startswith("local-dummy-") or len(receipt) > 100:
            return None
        digest = hashlib.sha256(receipt.encode()).hexdigest()
        target = self.store / (digest + ".pdf")
        receipt_path = self.store / (digest + ".receipt")
        try:
            if receipt_path.read_text(encoding="ascii") != receipt:
                return None
            content = target.read_bytes()
        except (FileNotFoundError, OSError, UnicodeError):
            return None
        return content if content.startswith(b"%PDF-") else None


class Handler(BaseHTTPRequestHandler):
    server_version = "SI-CENDIKIA-LOCAL-DUMMY/1"

    @property
    def state(self) -> GatewayState:
        return self.server.gateway_state  # type: ignore[attr-defined]

    def log_message(self, fmt: str, *args: object) -> None:
        # Avoid request bodies, credentials, and PDF data in logs.
        sys.stderr.write("local-esign: " + (fmt % args) + "\n")

    def send_json(self, status: int, payload: dict[str, object]) -> None:
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(body)

    def authorized(self) -> bool:
        header = self.headers.get("Authorization", "")
        if not header.startswith("Basic "):
            return False
        try:
            decoded = base64.b64decode(header[6:], validate=True).decode("utf-8")
        except (binascii.Error, UnicodeDecodeError, ValueError):
            return False
        user, separator, password = decoded.partition(":")
        return separator == ":" and hmac.compare_digest(user, self.state.username) and hmac.compare_digest(password, self.state.password)

    def body(self) -> bytes | None:
        raw_length = self.headers.get("Content-Length", "")
        try:
            length = int(raw_length)
        except ValueError:
            self.send_json(HTTPStatus.LENGTH_REQUIRED, {"error": "invalid content length"})
            return None
        if length < 0 or length > MAX_BODY:
            self.send_json(HTTPStatus.REQUEST_ENTITY_TOO_LARGE, {"error": "request too large"})
            return None
        return self.rfile.read(length)

    def do_POST(self) -> None:  # noqa: N802
        parsed = urlparse(self.path)
        if parsed.path != "/api/v2/sign/pdf":
            self.send_json(HTTPStatus.NOT_FOUND, {"error": "not found"})
            return
        if not self.authorized():
            self.send_json(HTTPStatus.UNAUTHORIZED, {"error": "unauthorized"})
            return
        raw = self.body()
        if raw is None:
            return
        try:
            payload = json.loads(raw)
            files = payload.get("file")
            properties = payload.get("signatureProperties")
            if not isinstance(payload.get("nik"), str) or not payload["nik"]:
                raise ValueError("nik required")
            if not isinstance(payload.get("passphrase"), str) or not payload["passphrase"]:
                raise ValueError("passphrase required")
            if not isinstance(files, list) or len(files) != 1 or not isinstance(files[0], str):
                raise ValueError("one PDF required")
            if not isinstance(properties, list) or len(properties) != 1:
                raise ValueError("one signature property required")
            concept = base64.b64decode(files[0], validate=True)
            if len(concept) == 0 or len(concept) > MAX_PDF or not concept.startswith(b"%PDF-"):
                raise ValueError("invalid PDF")
        except (ValueError, TypeError, KeyError, json.JSONDecodeError, binascii.Error):
            self.send_json(HTTPStatus.BAD_REQUEST, {"error": "invalid signing request"})
            return
        try:
            receipt = self.state.save(concept + MARKER)
        except OSError:
            self.send_json(HTTPStatus.INTERNAL_SERVER_ERROR, {"error": "cannot store signed PDF"})
            return
        self.send_json(HTTPStatus.OK, {"id_dokumen": receipt, "mode": "LOCAL_DUMMY"})

    def do_GET(self) -> None:  # noqa: N802
        parsed = urlparse(self.path)
        if parsed.path.startswith("/api/sign/download/"):
            if not self.authorized():
                self.send_json(HTTPStatus.UNAUTHORIZED, {"error": "unauthorized"})
                return
            receipt = unquote(parsed.path.rsplit("/", 1)[-1])
            content = self.state.load(receipt)
            if content is None:
                self.send_json(HTTPStatus.NOT_FOUND, {"error": "receipt not found"})
                return
            self.send_response(HTTPStatus.OK)
            self.send_header("Content-Type", "application/pdf")
            self.send_header("Content-Length", str(len(content)))
            self.send_header("Cache-Control", "no-store")
            self.end_headers()
            self.wfile.write(content)
            return
        if parsed.path.startswith("/api/user/status/"):
            if not self.authorized():
                self.send_json(HTTPStatus.UNAUTHORIZED, {"error": "unauthorized"})
                return
            self.send_json(HTTPStatus.OK, {"status": "LOCAL_DUMMY"})
            return
        self.send_json(HTTPStatus.NOT_FOUND, {"error": "not found"})


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--listen", default=os.environ.get("LOCAL_ESIGN_LISTEN", "127.0.0.1:19090"))
    parser.add_argument("--store", default=os.environ.get("LOCAL_ESIGN_STORE", "/var/lib/si-cendikia/local-esign"))
    parser.add_argument("--username", default=os.environ.get("LOCAL_ESIGN_USERNAME", ""))
    parser.add_argument("--password", default=os.environ.get("LOCAL_ESIGN_PASSWORD", ""))
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    if not args.username or not args.password:
        raise SystemExit("LOCAL_ESIGN_USERNAME dan LOCAL_ESIGN_PASSWORD wajib diisi")
    host, separator, raw_port = args.listen.rpartition(":")
    if separator != ":" or not host or not raw_port.isdigit():
        raise SystemExit("--listen harus berbentuk HOST:PORT")
    if host not in {"127.0.0.1", "::1", "localhost"}:
        raise SystemExit("gateway lokal hanya boleh listen di loopback")
    server = ThreadingHTTPServer((host, int(raw_port)), Handler)
    server.gateway_state = GatewayState(Path(args.store), args.username, args.password)  # type: ignore[attr-defined]
    actual_host, actual_port = server.server_address[:2]
    print(json.dumps({"listen": f"{actual_host}:{actual_port}", "mode": "LOCAL_DUMMY"}), flush=True)
    def stop(_signum: int, _frame: object) -> None:
        import threading
        threading.Thread(target=server.shutdown, daemon=True).start()
    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    try:
        server.serve_forever(poll_interval=0.2)
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
