"""GET /api/skills"""
ORDER = 60
NAME = "GET /api/skills"

from apitest.harness import json_body, mysql, request


def run(ctx: dict) -> None:
    status, raw = request("GET", "/api/skills", token=ctx["student"])
    assert status == 200, raw
    items = json_body(raw)["items"]
    guide = next(item for item in items if item["key"] == "guide")
    rows = mysql(
        f"SELECT enabled FROM class_skills WHERE class_id={int(ctx['student_class'])} AND skill_key='guide'"
    )
    assert rows and bool(int(rows[0][0])) == bool(guide["enabled"])
    denied, denied_raw = request("PUT", "/api/skills/guide", token=ctx["student"], body={"enabled": False, "class_id": 2})
    assert denied == 403, denied_raw
    missing, _ = request("PUT", "/api/skills/care", token=ctx["teacher"], body={"enabled": True})
    assert missing == 404
