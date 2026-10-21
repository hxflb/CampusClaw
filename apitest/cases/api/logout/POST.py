"""POST /api/logout"""
ORDER = 90
NAME = "POST /api/logout"

from apitest.harness import request


def run(ctx: dict) -> None:
    status, raw = request("POST", "/api/logout", token=ctx["student"])
    assert status == 200, raw
    again, _ = request("GET", "/api/me", token=ctx["student"])
    assert again == 401
