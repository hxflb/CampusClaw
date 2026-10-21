"""HTTP and MySQL helpers for apitest cases."""
from __future__ import annotations

import json
import os
import subprocess
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def load_env() -> dict[str, str]:
    env: dict[str, str] = {}
    path = ROOT / ".env"
    if not path.exists():
        return env
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        k, v = line.split("=", 1)
        env[k.strip()] = v.strip()
    return env


ENV = load_env()
BASE = os.environ.get("BASE_URL", f"http://127.0.0.1:{ENV.get('WEB_PORT', '8080')}")
SEED = os.environ.get("SEED_PASSWORD", ENV.get("SEED_PASSWORD", "demo123"))


def request(method: str, path: str, token: str | None = None, body: dict | None = None, raw: bytes | None = None, content_type: str | None = None):
    data = raw
    headers: dict[str, str] = {}
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    if content_type:
        headers["Content-Type"] = content_type
    if token:
        headers["Authorization"] = token if token.lower().startswith("bearer ") else "Bearer " + token
    req = urllib.request.Request(BASE + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()


def json_body(raw: bytes):
    return json.loads(raw.decode("utf-8"))


def login(username: str, password: str | None = None) -> tuple[int, dict]:
    status, raw = request("POST", "/api/login", body={"username": username, "password": password if password is not None else SEED})
    try:
        data = json_body(raw)
    except json.JSONDecodeError:
        data = {"raw": raw.decode("utf-8", "replace")}
    return status, data


def mysql(sql: str) -> list[list[str]]:
    user = ENV.get("DB_USER", "")
    password = ENV.get("DB_PASSWORD", "")
    name = ENV.get("DB_NAME", "")
    if not user or not password or not name:
        raise RuntimeError("数据库账号未写在 .env")
    proc = subprocess.run(
        ["docker", "compose", "exec", "-T", "-e", f"MYSQL_PWD={password}", "db", "mysql", "--default-character-set=utf8mb4", "-u", user, "-N", "-B", name, "-e", sql],
        cwd=ROOT,
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )
    if proc.returncode != 0:
        err = (proc.stderr or proc.stdout or "mysql failed").strip()
        raise RuntimeError(err.splitlines()[-1] if err else "mysql failed")
    rows = []
    for line in proc.stdout.splitlines():
        if line.strip():
            rows.append(line.split("\t"))
    return rows
