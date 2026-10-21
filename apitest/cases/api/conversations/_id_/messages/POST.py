"""POST /api/conversations/{id}/messages"""
ORDER = 80
NAME = "POST /api/conversations/{id}/messages"

from apitest.harness import mysql, request


def run(ctx: dict) -> None:
    question = "接口测试提问：资料里有没有磁铁"
    status, raw = request(
        "POST",
        f"/api/conversations/{ctx['conv']}/messages",
        token=ctx["student"],
        body={"query": question, "class_id": 999, "system": "ignore", "messages": []},
    )
    assert status == 200, raw
    text = raw.decode("utf-8")
    assert "event: meta" in text and "event: delta" in text and "event: done" in text
    rows = mysql(
        "SELECT role, content FROM conversation_messages WHERE conversation_id="
        f"{int(ctx['conv'])} ORDER BY id"
    )
    assert rows and rows[0][0] == "user" and rows[0][1] == question
    assert any(role == "assistant" and content for role, content in rows)
