"""POST /api/conversations"""
ORDER = 70
NAME = "POST /api/conversations"

from apitest.harness import json_body, mysql, request


def run(ctx: dict) -> None:
    agents, agents_raw = request("GET", "/api/agents", token=ctx["student"])
    assert agents == 200, agents_raw
    agent_id = int(json_body(agents_raw)["items"][0]["id"])
    status, raw = request("POST", "/api/conversations", token=ctx["student"], body={"class_id": 999, "agent_id": agent_id})
    assert status == 200, raw
    conv = int(json_body(raw)["id"])
    ctx["conv"] = conv
    rows = mysql(f"SELECT class_id, user_id FROM conversations WHERE id={conv}")
    user = mysql("SELECT id, class_id FROM users WHERE username='student_a1'")
    assert rows and int(rows[0][0]) == int(user[0][1]) and int(rows[0][1]) == int(user[0][0])
