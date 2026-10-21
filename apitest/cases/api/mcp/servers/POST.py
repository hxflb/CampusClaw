"""MCP 登记、连接、技能调用与审计"""
ORDER = 100
NAME = "POST /api/mcp/servers"

import json

from apitest.harness import ENV, json_body, login, mysql, request

TOKEN = ENV.get("MCP_AUTH_TOKEN", "")


SKILL = """名称：公式提示
适用条件：公式,换算
资料：数学
工具：formula_hint@campus-tools
启用：是

先调用 formula_hint 取得提示，再按提示作答，不计算最终得数。
"""


def run(ctx: dict) -> None:
    teacher_status, teacher = login("teacher_a")
    assert teacher_status == 200, teacher
    token = teacher["token"]
    student_status, student = login("student_a1")
    assert student_status == 200, student
    student_token = student["token"]

    denied, denied_raw = request("POST", "/api/mcp/servers", token=student_token, body={"name": "campus-tools", "base_url": "http://mcp-demo:8090"})
    assert denied == 403, denied_raw
    audit_denied, _ = request("GET", "/api/mcp/audit", token=student_token)
    assert audit_denied == 403

    status, raw = request("GET", "/api/mcp/servers", token=token)
    assert status == 200, raw
    items = json_body(raw)["items"]
    server = next((item for item in items if item["name"] == "campus-tools"), None)
    if server is None:
        created, created_raw = request("POST", "/api/mcp/servers", token=token, body={
            "name": "campus-tools",
            "base_url": "http://mcp-demo:8090",
            "token": TOKEN,
        })
        assert created == 201, created_raw
        server = json_body(created_raw)
    assert "has_token" in server, server
    assert server["has_token"] is True, server
    if TOKEN:
        assert TOKEN not in json.dumps(server), server
    server_id = int(server["id"])

    connected, connected_raw = request("POST", f"/api/mcp/servers/{server_id}/connect", token=token)
    assert connected == 200, connected_raw
    tools = json_body(connected_raw)["tools"]
    names = [tool["name"] for tool in tools]
    assert "formula_hint" in names and "unit_convert" in names and "weather_query" in names, tools
    assert all(tool.get("description") for tool in tools), tools
    if TOKEN:
        assert TOKEN not in connected_raw.decode()

    skill_id = 0
    agent = None
    original: list = []
    skill_status, skill_raw = request("POST", "/api/skill-files", token=token, body={"source": SKILL})
    assert skill_status == 200, skill_raw
    skill_id = int(json_body(skill_raw)["id"])
    agents, agents_raw = request("GET", "/api/agents", token=token)
    assert agents == 200, agents_raw
    agent = next(item for item in json_body(agents_raw)["items"] if item["enabled"])
    original = list(agent["skill_ids"])
    saved, saved_raw = request("PUT", f"/api/agents/{agent['id']}", token=token, body={
        "name": agent["name"],
        "prompt": agent["prompt"],
        "enabled": True,
        "skill_ids": original + [skill_id],
        "material_ids": agent["material_ids"],
    })
    assert saved == 200, saved_raw

    try:
        conv, conv_raw = request("POST", "/api/conversations", token=student_token, body={"agent_id": agent["id"]})
        assert conv == 200, conv_raw
        conv_id = json_body(conv_raw)["id"]
        asked, asked_raw = request("POST", f"/api/conversations/{conv_id}/messages", token=student_token, body={
            "query": "公式换算下一步做什么",
            "agent_id": agent["id"],
        })
        assert asked == 200, asked_raw
        meta = sse_meta(asked_raw)
        assert meta["status"] == "ok", meta
        assert "tool_calls" not in meta, meta
        call = next(item for item in sse_calls(asked_raw) if item["ref"] == "formula_hint@campus-tools")
        assert call["status"] == "ok", call
        rows = mysql("SELECT tool_ref, status, round_no, conversation_id FROM mcp_audit WHERE tool_ref='formula_hint@campus-tools' AND status='ok' ORDER BY id DESC LIMIT 1")
        assert rows and rows[0][0] == "formula_hint@campus-tools" and rows[0][1] == "ok"
        assert int(rows[0][2]) >= 1 and int(rows[0][3]) == int(conv_id), rows

        off, off_raw = request("POST", f"/api/mcp/servers/{server_id}/disconnect", token=token)
        assert off == 200, off_raw
        again, again_raw = request("POST", f"/api/conversations/{conv_id}/messages", token=student_token, body={
            "query": "公式还要怎么换算",
            "agent_id": agent["id"],
        })
        assert again == 200, again_raw
        denied_call = next(item for item in sse_calls(again_raw) if item["ref"] == "formula_hint@campus-tools")
        assert denied_call["status"] == "denied", again_raw
        rejected, _ = request("PUT", f"/api/skill-files/{skill_id}", token=token, body={"source": SKILL})
        assert rejected == 400
    finally:
        if agent is not None:
            request("PUT", f"/api/agents/{agent['id']}", token=token, body={
                "name": agent["name"],
                "prompt": agent["prompt"],
                "enabled": agent["enabled"],
                "skill_ids": original,
                "material_ids": agent["material_ids"],
            })
        if skill_id:
            request("DELETE", f"/api/skill-files/{skill_id}", token=token)
        request("POST", f"/api/mcp/servers/{server_id}/connect", token=token)


def sse_meta(raw: bytes) -> dict:
    for block in raw.decode("utf-8").split("\n\n"):
        event, data = "message", ""
        for line in block.splitlines():
            if line.startswith("event:"):
                event = line[6:].strip()
            elif line.startswith("data:"):
                data += line[5:].strip()
        if event == "meta" and data:
            return json.loads(data)
    raise AssertionError(raw[:500])


def sse_calls(raw: bytes) -> list:
    """收集所有 tool_calls 事件里的调用记录。"""
    out = []
    for block in raw.decode("utf-8").split("\n\n"):
        event, data = "message", ""
        for line in block.splitlines():
            if line.startswith("event:"):
                event = line[6:].strip()
            elif line.startswith("data:"):
                data += line[5:].strip()
        if event == "tool_calls" and data:
            out.extend(json.loads(data).get("calls") or [])
    return out
