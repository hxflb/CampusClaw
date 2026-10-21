"""对照 spec 关键 Scenario 的本地验收。"""
from __future__ import annotations

import io
import os
import sqlite3
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))
os.environ.setdefault("SECRET_KEY", "verify-secret-key")

from app.config import DB_PATH  # noqa: E402
from app import create_app  # noqa: E402
from scripts.init_db import init_db  # noqa: E402


def login(client, username: str, password: str = "demo123"):
    return client.post(
        "/login",
        data={"username": username, "password": password},
        follow_redirects=False,
    )


def main() -> int:
    init_db()
    app = create_app()
    failures: list[str] = []

    with app.test_client() as client:
        h = client.get("/health")
        if h.status_code != 200 or h.get_json().get("status") != "ok":
            failures.append("GET /health 失败")

        unauth = client.get("/materials", follow_redirects=False)
        if unauth.status_code != 302 or "/login" not in (unauth.headers.get("Location") or ""):
            failures.append("未登录访问 /materials 未重定向登录页")
        if "A 班" in unauth.get_data(as_text=True) or "B 班" in unauth.get_data(as_text=True):
            failures.append("未登录响应泄露材料标题")

        bad = login(client, "teacher_a", "wrong")
        if bad.status_code != 401:
            failures.append("错误密码应失败")
        if "password_hash" in bad.get_data(as_text=True):
            failures.append("错误密码响应泄露哈希")

        ok = login(client, "teacher_a")
        if ok.status_code != 302:
            failures.append("教师登录应成功")

        listed = client.get("/materials")
        text = listed.get_data(as_text=True)
        if listed.status_code != 200 or "A 班春季教研材料" not in text:
            failures.append("教师 A 列表应含 A 班材料")
        if "B 班春季教研材料" in text:
            failures.append("教师 A 列表泄露 B 班标题")

        # B 班材料 id
        conn = sqlite3.connect(DB_PATH)
        b_id = conn.execute(
            "SELECT id FROM materials WHERE class_id = 2 ORDER BY id LIMIT 1"
        ).fetchone()[0]
        conn.close()

        teacher_b = client.get(f"/api/materials/{b_id}")
        tb = teacher_b.get_json() or {}
        if teacher_b.status_code != 200 or tb.get("class_id") != 2:
            failures.append(
                f"教师应能查看 B 班材料，实际 {teacher_b.status_code} {teacher_b.get_data(as_text=True)}"
            )

        switched = client.get("/materials?class_id=2")
        if "B 班春季教研材料" not in switched.get_data(as_text=True):
            failures.append("教师切换到 B 班后应看到 B 班列表")

        up = client.post(
            "/api/materials/upload",
            data={
                "class_id": "1",
                "title": "A 班新讲义",
                "file": (io.BytesIO("新入库正文\n".encode("utf-8")), "note.md"),
            },
            content_type="multipart/form-data",
        )
        if up.status_code != 201 or not up.get_json().get("material_id"):
            failures.append(f"教师上传应 201，实际 {up.status_code} {up.get_data(as_text=True)}")

        listed2 = client.get("/api/materials").get_json()["items"]
        titles = [i["title"] for i in listed2]
        if "A 班新讲义" not in titles:
            failures.append("上传后教师列表不可见新记录")

        client.get("/logout")
        login(client, "student_a1")
        st_list = client.get("/api/materials").get_json()["items"]
        st_titles = [i["title"] for i in st_list]
        if "A 班新讲义" not in st_titles:
            failures.append("学生 A1 应看到教师新上传")
        if any("B 班" in t for t in st_titles):
            failures.append("学生 A1 列表出现 B 班")
        spoof = client.get("/api/materials?class_id=2").get_json()["items"]
        if any("B 班" in i["title"] for i in spoof):
            failures.append("学生用 class_id 参数仍看到 B 班")
        html_cross = client.get(f"/materials/{b_id}")
        if html_cross.status_code != 403:
            failures.append("学生跨班页面应为 403")
        if "B 班春季" in html_cross.get_data(as_text=True):
            failures.append("学生跨班 HTML 泄露标题")

        before = sqlite3.connect(DB_PATH)
        n_m = before.execute("SELECT COUNT(*) FROM materials").fetchone()[0]
        n_k = before.execute("SELECT COUNT(*) FROM knowledge_entries").fetchone()[0]
        before.close()

        denied = client.post(
            "/api/materials/upload",
            data={"file": (io.BytesIO(b"x"), "x.md")},
            content_type="multipart/form-data",
        )
        if denied.status_code != 403:
            failures.append(f"学生上传应为 403，实际 {denied.status_code}")

        after = sqlite3.connect(DB_PATH)
        if after.execute("SELECT COUNT(*) FROM materials").fetchone()[0] != n_m:
            failures.append("学生上传后 materials 行数变化")
        if after.execute("SELECT COUNT(*) FROM knowledge_entries").fetchone()[0] != n_k:
            failures.append("学生上传后 knowledge 行数变化")
        hashes = [r[0] for r in after.execute("SELECT password_hash FROM users")]
        after.close()
        if any(h == "demo123" for h in hashes) or not all(h.startswith("$2") for h in hashes):
            failures.append("密码未使用 bcrypt 哈希")

        client.get("/logout")
        login(client, "teacher_a")
        to_b = client.post(
            "/api/materials/upload",
            data={
                "class_id": "2",
                "title": "教师发到 B 班",
                "file": (io.BytesIO("给B班的通知\n".encode("utf-8")), "b.md"),
            },
            content_type="multipart/form-data",
        )
        if to_b.status_code != 201 or to_b.get_json().get("class_id") != 2:
            failures.append(f"教师应能选择 B 班上传，实际 {to_b.status_code} {to_b.get_data(as_text=True)}")

        client.get("/logout")
        login(client, "student_b1")
        b_titles = [i["title"] for i in client.get("/api/materials").get_json()["items"]]
        if "教师发到 B 班" not in b_titles:
            failures.append("B 班学生应看到教师选班上传的材料")

        client.get("/logout")
        login(client, "student_a1")
        a_titles = [i["title"] for i in client.get("/api/materials").get_json()["items"]]
        if "教师发到 B 班" in a_titles:
            failures.append("A 班学生不应看到发给 B 班的材料")

        client.get("/logout")
        login(client, "teacher_a")
        created = client.post(
            "/api/materials/upload",
            data={
                "class_id": "1",
                "title": "待改资料",
                "file": (io.BytesIO("原文\n".encode("utf-8")), "old.md"),
            },
            content_type="multipart/form-data",
        )
        mid = created.get_json()["material_id"]
        edited = client.post(
            f"/materials/{mid}/edit",
            data={"class_id": "1", "title": "已改标题", "body": "改后正文"},
        )
        if edited.status_code != 200 or edited.get_json().get("title") != "已改标题":
            failures.append(f"教师修改失败 {edited.status_code} {edited.get_data(as_text=True)}")
        shown = client.get(f"/api/materials/{mid}").get_json()
        if shown.get("body_text") != "改后正文":
            failures.append("修改后知识库正文未更新")

        client.get("/logout")
        login(client, "student_a1")
        if client.get(f"/materials/{mid}/edit").status_code != 403:
            failures.append("学生打开修改页应为 403")
        deny_edit = client.post(
            f"/materials/{mid}/edit",
            data={"class_id": "1", "title": "学生篡改", "body": "x"},
        )
        if deny_edit.status_code != 403:
            failures.append("学生修改应为 403")
        deny_del = client.post(f"/materials/{mid}/delete")
        if deny_del.status_code != 403:
            failures.append("学生删除应为 403")
        still = client.get(f"/api/materials/{mid}").get_json()
        if still.get("title") != "已改标题":
            failures.append("学生写操作后教师修改结果被破坏")

        client.get("/logout")
        login(client, "teacher_a")
        deleted = client.post(f"/materials/{mid}/delete")
        if deleted.status_code != 200 or deleted.get_json().get("deleted") != mid:
            failures.append(f"教师删除失败 {deleted.status_code}")
        if client.get(f"/api/materials/{mid}").status_code != 404:
            failures.append("删除后按 ID 仍能读到材料")

        client.get("/logout")
        login(client, "teacher_a")
        bad_file = client.post(
            "/api/materials/upload",
            data={"class_id": "1", "file": (io.BytesIO(b"MZ"), "virus.exe")},
            content_type="multipart/form-data",
        )
        if bad_file.status_code != 400:
            failures.append("非法扩展名应为 400")

    if failures:
        print("FAIL")
        for f in failures:
            print("-", f)
        return 1
    print("PASS: login / isolation / student-403 / upload-visible")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
