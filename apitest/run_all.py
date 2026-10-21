"""Run every apitest case and write an HTML report."""
from __future__ import annotations

import importlib.util
import sys
import traceback
from datetime import datetime
from html import escape
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

CASES = Path(__file__).resolve().parent / "cases"
REPORT = Path(__file__).resolve().parent / "report.html"


def discover():
    found = []
    for path in CASES.rglob("*.py"):
        if path.name.startswith("_"):
            continue
        spec = importlib.util.spec_from_file_location(path.stem + str(path.stat().st_mtime_ns), path)
        if spec is None or spec.loader is None:
            continue
        mod = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(mod)
        if not hasattr(mod, "run"):
            continue
        found.append((getattr(mod, "ORDER", 1000), getattr(mod, "NAME", path.stem), mod.run))
    found.sort(key=lambda item: item[0])
    return found


def main() -> int:
    ctx: dict = {}
    rows = []
    failed = 0
    for _order, name, fn in discover():
        try:
            fn(ctx)
            rows.append((name, True, "通过"))
            print("ok", name)
        except Exception as exc:  # noqa: BLE001 — report each case
            failed += 1
            detail = traceback.format_exc().strip().splitlines()[-1]
            if str(exc) and str(exc) not in detail:
                detail = str(exc)
            rows.append((name, False, detail))
            print("FAIL", name, detail)
    write(rows)
    print(f"report {REPORT}")
    return 1 if failed else 0


def write(rows: list[tuple[str, bool, str]]) -> None:
    body = []
    for name, ok, detail in rows:
        body.append(
            "<tr class='{cls}'><td>{name}</td><td>{state}</td><td>{detail}</td></tr>".format(
                cls="ok" if ok else "bad",
                name=escape(name),
                state="通过" if ok else "失败",
                detail=escape(detail),
            )
        )
    passed = sum(1 for _, ok, _ in rows if ok)
    html = """<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>CampusClaw 接口测试</title>
<style>
body {{ font-family: "Microsoft YaHei", sans-serif; margin: 32px; color: #1b2420; }}
table {{ border-collapse: collapse; width: 100%; }}
td, th {{ border: 1px solid #d5e3db; padding: 8px 10px; text-align: left; }}
.ok td:nth-child(2) {{ color: #146348; }}
.bad td:nth-child(2) {{ color: #c0392b; }}
</style></head><body>
<h1>接口测试报告</h1>
<p>{when} · 通过 {passed} / {total}</p>
<table><thead><tr><th>接口</th><th>结果</th><th>说明</th></tr></thead><tbody>
{rows}
</tbody></table>
</body></html>
""".format(when=escape(datetime.now().strftime("%Y-%m-%d %H:%M:%S")), passed=passed, total=len(rows), rows="\n".join(body))
    REPORT.write_text(html, encoding="utf-8")


if __name__ == "__main__":
    raise SystemExit(main())
