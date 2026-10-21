"""POST /api/search"""
ORDER = 30
NAME = "POST /api/search"

from apitest.harness import json_body, mysql, request


def run(ctx: dict) -> None:
    status, raw = request("POST", "/api/search", token=ctx["student"], body={"query": "磁铁", "mode": "hybrid", "class_id": 999})
    assert status == 200, raw
    data = json_body(raw)
    assert data.get("mode") == "hybrid"
    hits = data.get("hits") or []
    class_id = ctx["student_class"]
    for hit in hits:
        rows = mysql(f"SELECT class_id FROM materials WHERE id={int(hit['material_id'])}")
        assert rows and int(rows[0][0]) == class_id, hit
        assert "B 班" not in hit.get("material_title", "")
