"""HTTP scenario checks against a running Compose stack (web on WEB_PORT)."""
from __future__ import annotations

import json
import os
import urllib.error
import urllib.request

BASE = os.environ.get("BASE_URL", "http://127.0.0.1:8080")
SEED = os.environ.get("SEED_PASSWORD", "demo123")


def req(method: str, path: str, cookie: str | None = None, data: bytes | None = None, content_type: str | None = None):
    headers = {}
    if cookie:
        headers["Authorization"] = cookie
    if content_type:
        headers["Content-Type"] = content_type
    r = urllib.request.Request(BASE + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(r, timeout=20) as resp:
            body = resp.read()
            return resp.status, dict(resp.headers), body
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e.read()


def login(user: str) -> str:
    status, headers, body = req(
        "POST",
        "/api/login",
        data=json.dumps({"username": user, "password": SEED}).encode(),
        content_type="application/json",
    )
    assert status == 200, body
    token = json.loads(body.decode("utf-8"))["token"]
    assert token.count(".") == 2, token
    return "Bearer " + token


def json_body(body: bytes):
    return json.loads(body.decode("utf-8"))


def main() -> None:
    st, _, body = req("GET", "/health")
    assert st == 200 and json_body(body).get("status") == "ok", body

    st, _, body = req("GET", "/api/materials")
    assert st == 401
    text = body.decode("utf-8", "replace")
    assert "B 班" not in text and "A 班" not in text

    a1 = login("student_a1")
    st, _, body = req("GET", "/api/materials", cookie=a1)
    assert st == 200
    titles = [i["title"] for i in json_body(body)["items"]]
    assert any("A 班" in t for t in titles)
    assert not any("B 班" in t for t in titles)

    ta = login("teacher_a")
    st, _, body = req("GET", "/api/materials?class_id=2", cookie=ta)
    assert st == 200
    b_items = json_body(body)["items"]
    b_id = next(i["id"] for i in b_items if "B 班" in i["title"])

    st, _, body = req("GET", f"/api/materials/{b_id}", cookie=a1)
    assert st == 404
    assert "B 班" not in body.decode("utf-8", "replace")

    boundary = "----ccbound"
    parts = (
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"title\"\r\n\r\nshould-fail\r\n"
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"x.txt\"\r\nContent-Type: text/plain\r\n\r\nx\r\n"
        f"--{boundary}--\r\n"
    ).encode()
    st, _, _ = req("POST", "/api/materials", cookie=a1, data=parts, content_type=f"multipart/form-data; boundary={boundary}")
    assert st == 403

    payload = (
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"title\"\r\n\r\n验收上传A\r\n"
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"class_id\"\r\n\r\n1\r\n"
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; filename=\"ok.md\"\r\nContent-Type: text/markdown\r\n\r\n验收正文\r\n"
        f"--{boundary}--\r\n"
    ).encode()
    st, _, body = req("POST", "/api/materials", cookie=ta, data=payload, content_type=f"multipart/form-data; boundary={boundary}")
    assert st == 201, body
    new_id = json_body(body)["material_id"]

    st, _, body = req("GET", "/api/materials", cookie=a1)
    assert any(i["id"] == new_id for i in json_body(body)["items"])

    st, _, _ = req("DELETE", f"/api/materials/{new_id}", cookie=a1)
    assert st == 403
    st, _, _ = req("DELETE", f"/api/materials/{new_id}", cookie=ta)
    assert st == 200

    gone = login("student_a1")
    st, _, _ = req("POST", "/api/logout", cookie=gone)
    assert st == 200
    st, _, body = req("GET", "/api/materials", cookie=gone)
    assert st == 401, body
    tampered = gone[:-1] + ("A" if gone[-1] != "A" else "B")
    st, _, _ = req("GET", "/api/materials", cookie=tampered)
    assert st == 401
    print("verify_api: ok")


if __name__ == "__main__":
    main()
