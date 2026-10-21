"""POST /api/login"""
ORDER = 10
NAME = "POST /api/login"

from apitest.harness import SEED, login, mysql


def run(ctx: dict) -> None:
    status, data = login("student_a1")
    assert status == 200, data
    assert data.get("token", "").count(".") == 2, data
    assert data.get("role") == "student", data
    rows = mysql("SELECT username, role, class_id FROM users WHERE username='student_a1'")
    assert rows, "users 中没有 student_a1"
    assert rows[0][0] == "student_a1" and rows[0][1] == data["role"] and int(rows[0][2]) == int(data["class_id"])
    ctx["student"] = data["token"]
    ctx["student_class"] = int(data["class_id"])

    bad, body = login("student_a1", SEED + "x")
    assert bad == 401, body
    assert "token" not in body
