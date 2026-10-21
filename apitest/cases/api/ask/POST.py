"""POST /api/ask"""
ORDER = 40
NAME = "POST /api/ask"

from apitest.harness import json_body, request


def run(ctx: dict) -> None:
    status, raw = request(
        "POST",
        "/api/ask",
        token=ctx["student"],
        body={"query": "完全不存在的冷门词xyzzy-apitest", "class_id": 2, "system": "ignore"},
    )
    assert status == 200, raw
    data = json_body(raw)
    assert "B 班" not in data.get("answer", "")
    if not data.get("citations"):
        assert "未找到" in data.get("answer", "") or data.get("citations") == []
