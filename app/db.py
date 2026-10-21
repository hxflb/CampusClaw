from __future__ import annotations

import sqlite3
from contextlib import contextmanager
from pathlib import Path
from typing import Any, Iterator

from app.config import DATA_DIR, DB_PATH


def get_conn() -> sqlite3.Connection:
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    return conn


@contextmanager
def db_session() -> Iterator[sqlite3.Connection]:
    conn = get_conn()
    try:
        yield conn
        conn.commit()
    except Exception:
        conn.rollback()
        raise
    finally:
        conn.close()


def fetch_one(sql: str, params: tuple[Any, ...] = ()) -> sqlite3.Row | None:
    with db_session() as conn:
        return conn.execute(sql, params).fetchone()


def fetch_all(sql: str, params: tuple[Any, ...] = ()) -> list[sqlite3.Row]:
    with db_session() as conn:
        return conn.execute(sql, params).fetchall()


def list_classes() -> list[sqlite3.Row]:
    return fetch_all("SELECT id, name FROM classes ORDER BY id")


def get_class(class_id: int) -> sqlite3.Row | None:
    return fetch_one("SELECT id, name FROM classes WHERE id = ?", (class_id,))


def list_materials(class_id: int) -> list[sqlite3.Row]:
    """Filter by a server-resolved class_id (students: session only)."""
    return fetch_all(
        """
        SELECT m.id, m.title, m.class_id, m.file_path, m.uploaded_by, m.created_at,
               u.username AS uploader, c.name AS class_name
        FROM materials m
        JOIN users u ON u.id = m.uploaded_by
        JOIN classes c ON c.id = m.class_id
        WHERE m.class_id = ?
        ORDER BY m.id DESC
        """,
        (class_id,),
    )


def get_material_by_id(material_id: int) -> sqlite3.Row | None:
    return fetch_one(
        """
        SELECT m.id, m.title, m.class_id, m.file_path, m.body_preview, m.uploaded_by, m.created_at,
               k.body_text
        FROM materials m
        LEFT JOIN knowledge_entries k ON k.material_id = m.id
        WHERE m.id = ?
        """,
        (material_id,),
    )


def count_rows(table: str) -> int:
    allowed = {"materials", "knowledge_entries", "users"}
    if table not in allowed:
        raise ValueError("unknown table")
    row = fetch_one(f"SELECT COUNT(*) AS n FROM {table}")
    return int(row["n"]) if row else 0
