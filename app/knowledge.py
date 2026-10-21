from __future__ import annotations

from pathlib import Path


ALLOWED_EXT = {".txt", ".md"}
MAX_BYTES = 2 * 1024 * 1024


def parse_file(path: Path) -> str:
    if path.suffix.lower() not in ALLOWED_EXT:
        raise ValueError("unsupported_type")
    raw = path.read_bytes()
    if not raw.strip():
        raise ValueError("empty_file")
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError:
        text = raw.decode("utf-8", errors="replace")
    text = text.strip()
    if not text:
        raise ValueError("empty_file")
    return text
