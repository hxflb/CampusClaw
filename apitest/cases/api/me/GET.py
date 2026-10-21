"""GET /api/me"""
ORDER = 20
NAME = "GET /api/me"

from apitest.harness import json_body, mysql, request


def run(ctx: dict) -> None:
    status, raw = request("GET", "/api/me", token=ctx["student"])
    assert status == 200, raw
    me = json_body(raw)
    rows = mysql("SELECT id, username, role, class_id FROM users WHERE username='student_a1'")
    assert rows and int(rows[0][0]) == int(me["user_id"])
    assert rows[0][1] == me["username"] and rows[0][2] == me["role"] and int(rows[0][3]) == int(me["class_id"])
    plain, _ = request("GET", "/api/me")
    assert plain == 401
