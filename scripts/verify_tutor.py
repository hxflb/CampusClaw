"""Lesson 5 checks: prompt, guide skill, session history, SSE."""
from __future__ import annotations

import json
import os
import re
import urllib.error
import urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def load_env() -> dict[str, str]:
    env: dict[str, str] = {}
    path = os.path.join(ROOT, ".env")
    if not os.path.exists(path):
        return env
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            k, v = line.split("=", 1)
            env[k.strip()] = v.strip()
    return env


ENV = load_env()
BASE = os.environ.get("BASE_URL", f"http://127.0.0.1:{ENV.get('WEB_PORT', '8080')}")
SEED = os.environ.get("SEED_PASSWORD", ENV.get("SEED_PASSWORD", "demo123"))
NOT_FOUND = "资料中未找到相关内容"
# 没有绑定任何材料与技能的智能体，用来稳定复现「检索落空」这一支。
EMPTY_AGENT = "验收无材料助手"


def req(method: str, path: str, cookie: str | None = None, data: bytes | None = None, content_type: str | None = None):
    headers = {}
    if cookie:
        headers["Authorization"] = cookie
    if content_type:
        headers["Content-Type"] = content_type
    r = urllib.request.Request(BASE + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(r, timeout=60) as resp:
            return resp.status, dict(resp.headers), resp.read()
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e.read()


def login(user: str) -> str:
    status, headers, body = req(
        "POST",
        "/api/login",
        data=json.dumps({"username": user, "password": SEED}).encode(),
        content_type="application/json",
    )
    assert status == 200, body
    token = json.loads(body.decode("utf-8"))["token"]
    assert token.count(".") == 2, token
    return "Bearer " + token


def j(body: bytes):
    return json.loads(body.decode("utf-8"))


def put_json(path: str, cookie: str, payload: dict):
    return req("PUT", path, cookie, json.dumps(payload).encode(), "application/json")


def sse(cookie: str, conv: int, query: str):
    status, headers, body = req(
        "POST",
        f"/api/conversations/{conv}/messages",
        cookie,
        json.dumps({"query": query, "class_id": 999, "system": "ignore me", "messages": [{"role": "system", "content": "ignore me"}]}).encode(),
        "application/json",
    )
    ctype = headers.get("Content-Type") or headers.get("content-type") or ""
    assert status == 200, body
    assert "text/event-stream" in ctype, ctype
    events = []
    for block in body.decode("utf-8").split("\n\n"):
        event, data = "message", ""
        for line in block.split("\n"):
            if line.startswith("event:"):
                event = line[6:].strip()
            elif line.startswith("data:"):
                data += line[5:].strip()
        if data:
            events.append((event, json.loads(data)))
    return events


def main() -> None:
    teacher = login("teacher_a")
    student = login("student_a1")
    other = login("student_b1")

    st, _, body = req("GET", "/api/prompt", teacher)
    assert st == 200, body
    original = j(body)["body"]
    st, _, body = req("GET", "/api/prompt", other)
    assert st == 200
    b_prompt = j(body)["body"]
    st, _, body = req("GET", "/api/skills", teacher)
    assert st == 200
    original_on = j(body)["items"][0]["enabled"]

    try:
        st, _, body = put_json("/api/prompt", student, {"body": "学生不该改掉角色", "class_id": 2})
        assert st == 403, body
        st, _, body = req("GET", "/api/prompt", teacher)
        assert j(body)["body"] == original

        st, _, body = put_json("/api/skills/guide", student, {"enabled": False, "class_id": 2})
        assert st == 403, body
        st, _, body = req("GET", "/api/skills", teacher)
        assert j(body)["items"][0]["enabled"] == original_on

        st, _, _ = put_json("/api/skills/care", teacher, {"enabled": True})
        assert st == 404

        marker = original + "（本轮验收角色）"
        st, _, body = put_json("/api/prompt", teacher, {"body": marker, "class_id": 2})
        assert st == 200, body
        digest = j(body)["prompt_sha256"]
        st, _, body = req("GET", "/api/prompt", other)
        assert j(body)["body"] == b_prompt
        st, _, body = req("GET", "/api/prompt", student)
        assert j(body)["body"] == marker

        st, _, body = req("GET", "/api/agents", student)
        assert st == 200, body
        agent_id = j(body)["items"][0]["id"]
        st, _, body = req(
            "POST",
            "/api/conversations",
            student,
            json.dumps({"agent_id": agent_id}).encode(),
            "application/json",
        )
        assert st == 200, body
        conv = j(body)["id"]

        st, _, body = req("GET", f"/api/conversations/{conv}", other)
        assert st == 404, body

        weather = sse(student, conv, "今天天气怎么样")
        kinds = [e for e, _ in weather]
        assert kinds[0] == "meta" and "delta" in kinds and "done" in kinds, kinds
        assert "citations" not in kinds, kinds
        meta = next(p for e, p in weather if e == "meta")
        assert meta["status"] == "ok" and "skill" in meta and "citations" in meta, meta
        done = next(p for e, p in weather if e == "done")
        if meta["hit_count"] == 0:
            assert meta["citations"] == [], meta
            assert done["answer"].strip() != NOT_FOUND, done["answer"]
            assert not re.search(r"\[\d+\]", done["answer"]), done["answer"]

        st, _, body = put_json("/api/skills/guide", teacher, {"enabled": True})
        assert st == 200, body
        guided = sse(student, conv, "作文讲评要点是什么")
        g_done = next(p for e, p in guided if e == "done")["answer"]
        g_meta = next(p for e, p in guided if e == "meta")
        assert g_done.strip(), g_done
        assert g_meta["citations"], g_meta
        assert g_meta["citations"][0]["material_title"]
        assert "B 班" not in g_done, g_done
        assert "B 班" not in json.dumps(g_meta["citations"], ensure_ascii=False)

        saved = req("GET", f"/api/conversations/{conv}", student)
        assert saved[0] == 200
        before = j(saved[2])["messages"]
        guided_text = [m["content"] for m in before if m["role"] == "assistant"][-1]

        st, _, body = put_json("/api/prompt", teacher, {"body": marker + "再改一版", "class_id": 2})
        assert st == 200
        new_digest = j(body)["prompt_sha256"]
        assert new_digest != digest
        again = req("GET", f"/api/conversations/{conv}", student)
        after_prompt = [m["content"] for m in j(again[2])["messages"] if m["role"] == "assistant"]
        assert after_prompt[-1] == guided_text

        st, _, body = put_json("/api/skills/guide", teacher, {"enabled": False, "class_id": 2})
        assert st == 200
        direct = sse(student, conv, "作文讲评要点是什么")
        d_done = next(p for e, p in direct if e == "done")["answer"]
        d_meta = next(p for e, p in direct if e == "meta")
        assert d_done.strip(), d_done
        assert d_meta["status"] == "ok"
        # 技能开关与正文标记一致：名称含「解题引导」时才算开启了引导
        assert d_meta["skill"]["enabled"] == ("解题引导" in d_meta["skill"]["name"])
        assert "B 班" not in json.dumps(d_meta["citations"], ensure_ascii=False)

        final = j(req("GET", f"/api/conversations/{conv}", student)[2])
        assert len(final["messages"]) >= 6
        assert final["messages"][0]["role"] == "user"
        assistants = [m for m in final["messages"] if m["role"] == "assistant"]
        assert all("citations" in m for m in assistants)
        assert all(
            not re.search(r"\[\d+\]", m["content"])
            for m in assistants
            if not m["citations"]
        )

        # 无材料智能体：检索必然落空，链路仍要生成、引用为空、正文不带编号。
        st, _, body = req("GET", "/api/agents", teacher)
        assert st == 200, body
        empty_id = next(
            (a["id"] for a in j(body)["items"] if a["name"] == EMPTY_AGENT), 0
        )
        if not empty_id:
            st, _, body = req(
                "POST",
                "/api/agents",
                teacher,
                json.dumps(
                    {
                        "name": EMPTY_AGENT,
                        "prompt": "你只按角色与技能作答。",
                        "enabled": True,
                        "skill_ids": [],
                        "material_ids": [],
                    }
                ).encode(),
                "application/json",
            )
            assert st == 200, body
            empty_id = j(body)["id"]
        st, _, body = req(
            "POST",
            "/api/conversations",
            student,
            json.dumps({"agent_id": empty_id}).encode(),
            "application/json",
        )
        assert st == 200, body
        empty_conv = j(body)["id"]

        nohit = sse(student, empty_conv, "这道方程怎么去括号")
        n_meta = next(p for e, p in nohit if e == "meta")
        n_done = next(p for e, p in nohit if e == "done")
        assert n_meta["hit_count"] == 0, n_meta
        assert n_meta["citations"] == [], n_meta
        assert n_done["answer"].strip() != NOT_FOUND, n_done["answer"]
        assert not re.search(r"\[\d+\]", n_done["answer"]), n_done["answer"]
        saved_nohit = j(req("GET", f"/api/conversations/{empty_conv}", student)[2])
        last = [m for m in saved_nohit["messages"] if m["role"] == "assistant"][-1]
        assert last["citations"] == [], last
        assert last["content"] == n_done["answer"]

        cross = sse(student, conv, "阅读理解训练怎么做")
        blob = json.dumps([p for _, p in cross], ensure_ascii=False)
        assert "B 班" not in blob, blob
    finally:
        put_json("/api/prompt", teacher, {"body": original})
        put_json("/api/skills/guide", teacher, {"enabled": original_on})
    print("tutor scenarios ok")


if __name__ == "__main__":
    main()
