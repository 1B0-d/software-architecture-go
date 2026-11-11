"""Smoke-check two independent Go executables with temporary, isolated SQLite files.

Run: python standalone_check.py. No external services or network destinations.
"""
import json
import os
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent
GO = os.environ.get("GO", "go")
CREATE_NO_WINDOW = getattr(subprocess, "CREATE_NO_WINDOW", 0)

def reserve_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]

def request(url, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=3) as response:
        return response.status, json.load(response)

def wait_ready(url, proc):
    for _ in range(100):
        if proc.poll() is not None:
            raise RuntimeError(f"service exited early: {proc.returncode}")
        try:
            with urllib.request.urlopen(url, timeout=1) as response:
                if response.status == 200:
                    return
        except (OSError, urllib.error.URLError):
            time.sleep(0.1)
    raise RuntimeError(f"service did not become ready: {url}")

def main():
    with tempfile.TemporaryDirectory(prefix="go_smoke_") as d:
        path = Path(d)
        child_env = os.environ.copy()
        child_env.update({"GOTOOLCHAIN": "local", "GOPROXY": "off"})
        processes = []
        try:
            for name in ("catalog", "reviews"):
                subprocess.run([GO, "build", "-o", str(path / (name + ".exe")), "./cmd/" + name], cwd=ROOT, env=child_env, check=True, creationflags=CREATE_NO_WINDOW)
            cp, rp = reserve_port(), reserve_port()
            while rp == cp:
                rp = reserve_port()
            child_env.update({"CATALOG_ADDR": f"127.0.0.1:{cp}", "CATALOG_DB": str(path / "catalog.db"), "REVIEWS_ADDR": f"127.0.0.1:{rp}", "REVIEWS_DB": str(path / "reviews.db"), "CATALOG_URL": f"http://127.0.0.1:{cp}"})
            for name in ("catalog", "reviews"):
                processes.append(subprocess.Popen([str(path / (name + ".exe"))], cwd=ROOT, env=child_env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, creationflags=CREATE_NO_WINDOW))
            wait_ready(f"http://127.0.0.1:{cp}/", processes[0])
            # /reviews requires an ID, so process liveness and its first request verify readiness.
            status, vehicle = request(f"http://127.0.0.1:{cp}/vehicles", {"name": "Example car"})
            assert status == 201 and vehicle["id"] > 0
            for _ in range(30):
                try:
                    status, review = request(f"http://127.0.0.1:{rp}/reviews", {"vehicle_id": vehicle["id"], "text": "Good"})
                    break
                except (OSError, urllib.error.URLError):
                    if processes[1].poll() is not None:
                        raise RuntimeError("reviews exited before starting")
                    time.sleep(0.1)
            else:
                raise RuntimeError("reviews did not start")
            assert status == 201 and review["vehicle_id"] == vehicle["id"]
            status, rows = request(f"http://127.0.0.1:{rp}/reviews?vehicle_id={vehicle['id']}")
            assert status == 200 and len(rows) == 1 and rows[0]["text"] == "Good"
            print("PASS: two separate processes; POST vehicle 201; POST review 201; GET reviews 200; isolated temporary SQLite files")
        finally:
            for proc in reversed(processes):
                if proc.poll() is None:
                    proc.terminate()
                    try:
                        proc.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        proc.kill()
                        proc.wait()

if __name__ == "__main__":
    main()
