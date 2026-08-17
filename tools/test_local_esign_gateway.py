#!/usr/bin/env python3
import base64
import json
import os
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request


def request(url, method="GET", body=None, headers=None):
    req = urllib.request.Request(url, method=method, data=body, headers=headers or {})
    return urllib.request.urlopen(req, timeout=5)


def main():
    script = os.path.join(os.path.dirname(__file__), "local_esign_gateway.py")
    with tempfile.TemporaryDirectory() as store:
        proc = subprocess.Popen(
            [sys.executable, script, "--listen", "127.0.0.1:0", "--store", store,
             "--username", "dummy-user", "--password", "dummy-pass"],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
        )
        try:
            line = proc.stdout.readline().strip()
            if not line:
                raise AssertionError(proc.stderr.read())
            info = json.loads(line)
            base = f"http://{info['listen']}"
            pdf = b"%PDF-1.4\n% local gateway test\n%%EOF\n"
            payload = json.dumps({
                "nik": "0000000000000000",
                "passphrase": "dummy-passphrase",
                "file": [base64.b64encode(pdf).decode()],
                "signatureProperties": [{"tampilan": "VISIBLE"}],
            }).encode()
            auth = "Basic " + base64.b64encode(b"dummy-user:dummy-pass").decode()
            with request(base + "/api/v2/sign/pdf", "POST", payload, {
                "Content-Type": "application/json", "Authorization": auth,
            }) as resp:
                result = json.loads(resp.read())
            receipt = result["id_dokumen"]
            with request(base + "/api/sign/download/" + receipt, "GET", None, {
                "Authorization": auth,
            }) as resp:
                signed = resp.read()
            assert signed.startswith(b"%PDF-"), signed[:10]
            assert b"LOCAL-DUMMY-SIGNED" in signed, signed[-100:]
            try:
                request(base + "/api/v2/sign/pdf", "POST", payload, {})
            except urllib.error.HTTPError as exc:
                assert exc.code == 401, exc.code
            else:
                raise AssertionError("unauthenticated signing unexpectedly succeeded")
            print(json.dumps({"status": "PASS", "receipt": receipt, "bytes": len(signed)}))
        finally:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=5)


if __name__ == "__main__":
    main()
