"""GET /api/prompt"""
ORDER = 50
NAME = "GET /api/prompt"

from apitest.harness import json_body, login, mysql, request


def run(ctx: dict) -> None:
    status, raw = request("GET", "/api/prompt", token=ctx["student"])
    assert status == 200, raw
    body = json_body(raw)["body"]
    rows = mysql(f"SELECT body FROM class_prompts WHERE class_id={int(ctx['student_class'])}")
    assert rows and rows[0][0] == body

    teacher_status, teacher = login("teacher_a")
    assert teacher_status == 200, teacher
    ctx["teacher"] = teacher["token"]
    denied, denied_raw = request("PUT", "/api/prompt", token=ctx["student"], body={"body": "学生不该改掉", "class_id": 2})
    assert denied == 403, denied_raw
    again = mysql(f"SELECT body FROM class_prompts WHERE class_id={int(ctx['student_class'])}")
    assert again[0][0] == body
