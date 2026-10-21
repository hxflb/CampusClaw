from __future__ import annotations

import sqlite3
import sys
from pathlib import Path

import bcrypt

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))

from app.config import DATA_DIR, DB_PATH, UPLOAD_DIR  # noqa: E402


SEED_PASSWORD = "demo123"


def hash_password(plain: str) -> str:
    return bcrypt.hashpw(plain.encode("utf-8"), bcrypt.gensalt()).decode("utf-8")


def init_db() -> None:
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    UPLOAD_DIR.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(DB_PATH)
    conn.execute("PRAGMA foreign_keys = ON")
    cur = conn.cursor()

    cur.executescript(
        """
        CREATE TABLE IF NOT EXISTS classes (
            id INTEGER PRIMARY KEY,
            name TEXT NOT NULL UNIQUE
        );
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY,
            username TEXT NOT NULL UNIQUE,
            password_hash TEXT NOT NULL,
            role TEXT NOT NULL CHECK (role IN ('teacher', 'student')),
            class_id INTEGER NOT NULL REFERENCES classes(id)
        );
        CREATE TABLE IF NOT EXISTS lectures (
            id INTEGER PRIMARY KEY,
            class_id INTEGER NOT NULL REFERENCES classes(id),
            title TEXT NOT NULL
        );
        CREATE TABLE IF NOT EXISTS homeworks (
            id INTEGER PRIMARY KEY,
            class_id INTEGER NOT NULL REFERENCES classes(id),
            title TEXT NOT NULL
        );
        CREATE TABLE IF NOT EXISTS assistants (
            id INTEGER PRIMARY KEY,
            class_id INTEGER NOT NULL REFERENCES classes(id),
            name TEXT NOT NULL
        );
        CREATE TABLE IF NOT EXISTS skills (
            id INTEGER PRIMARY KEY,
            class_id INTEGER NOT NULL REFERENCES classes(id),
            name TEXT NOT NULL
        );
        CREATE TABLE IF NOT EXISTS materials (
            id INTEGER PRIMARY KEY,
            title TEXT NOT NULL,
            class_id INTEGER NOT NULL REFERENCES classes(id),
            file_path TEXT NOT NULL,
            uploaded_by INTEGER NOT NULL REFERENCES users(id),
            body_preview TEXT,
            created_at TEXT NOT NULL DEFAULT (datetime('now'))
        );
        CREATE TABLE IF NOT EXISTS knowledge_entries (
            id INTEGER PRIMARY KEY,
            material_id INTEGER NOT NULL REFERENCES materials(id),
            class_id INTEGER NOT NULL REFERENCES classes(id),
            body_text TEXT NOT NULL,
            created_at TEXT NOT NULL DEFAULT (datetime('now'))
        );
        """
    )

    if cur.execute("SELECT COUNT(*) FROM classes").fetchone()[0] == 0:
        cur.execute("INSERT INTO classes (id, name) VALUES (1, 'A 班'), (2, 'B 班')")

    pw = hash_password(SEED_PASSWORD)
    if cur.execute("SELECT COUNT(*) FROM users").fetchone()[0] == 0:
        cur.executemany(
            "INSERT INTO users (username, password_hash, role, class_id) VALUES (?, ?, ?, ?)",
            [
                ("teacher_a", pw, "teacher", 1),
                ("student_a1", pw, "student", 1),
                ("student_b1", pw, "student", 2),
            ],
        )

    if cur.execute("SELECT COUNT(*) FROM lectures").fetchone()[0] == 0:
        cur.execute("INSERT INTO lectures (class_id, title) VALUES (1, 'A 班讲义占位')")
        cur.execute("INSERT INTO homeworks (class_id, title) VALUES (1, 'A 班作业占位')")
        cur.execute("INSERT INTO assistants (class_id, name) VALUES (1, 'A 班助手占位')")
        cur.execute("INSERT INTO skills (class_id, name) VALUES (1, 'A 班技能占位')")

    teacher_a = cur.execute("SELECT id FROM users WHERE username = 'teacher_a'").fetchone()[0]
    if cur.execute("SELECT COUNT(*) FROM materials").fetchone()[0] == 0:
        seed_dir = UPLOAD_DIR / "seed"
        seed_dir.mkdir(parents=True, exist_ok=True)
        a_path = seed_dir / "a_ban.txt"
        b_path = seed_dir / "b_ban.txt"
        a_body = "A 班春季教研材料：本周作文讲评要点。"
        b_body = "B 班春季教研材料：本周阅读理解训练。"
        a_path.write_text(a_body, encoding="utf-8")
        b_path.write_text(b_body, encoding="utf-8")
        cur.execute(
            "INSERT INTO materials (title, class_id, file_path, uploaded_by, body_preview) VALUES (?, ?, ?, ?, ?)",
            ("A 班春季教研材料", 1, str(a_path.as_posix()), teacher_a, a_body),
        )
        a_id = cur.lastrowid
        # B 班材料由教师 A 以外的归属：uploaded_by 仍用 teacher_a 仅作外键；class_id=2
        cur.execute(
            "INSERT INTO materials (title, class_id, file_path, uploaded_by, body_preview) VALUES (?, ?, ?, ?, ?)",
            ("B 班春季教研材料", 2, str(b_path.as_posix()), teacher_a, b_body),
        )
        b_id = cur.lastrowid
        cur.execute(
            "INSERT INTO knowledge_entries (material_id, class_id, body_text) VALUES (?, ?, ?)",
            (a_id, 1, a_body),
        )
        cur.execute(
            "INSERT INTO knowledge_entries (material_id, class_id, body_text) VALUES (?, ?, ?)",
            (b_id, 2, b_body),
        )

    conn.commit()
    conn.close()


if __name__ == "__main__":
    init_db()
    print(f"initialized {DB_PATH}")
