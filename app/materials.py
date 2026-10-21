from __future__ import annotations

import uuid
from pathlib import Path

from flask import Blueprint, abort, flash, jsonify, redirect, render_template, request, session, url_for
from werkzeug.utils import secure_filename

from app.auth import login_required, teacher_required
from app.config import UPLOAD_DIR
from app.db import count_rows, db_session, get_class, get_material_by_id, list_classes, list_materials
from app.knowledge import ALLOWED_EXT, MAX_BYTES, parse_file

materials_bp = Blueprint("materials", __name__)


def is_teacher() -> bool:
    return session.get("role") == "teacher"


def viewer_class_id() -> int:
    """Students always use session class. Teachers may switch via class_id."""
    home = int(session["class_id"])
    if not is_teacher():
        return home
    raw = request.args.get("class_id")
    if raw is None or raw == "":
        return int(session.get("view_class_id") or home)
    try:
        cid = int(raw)
    except ValueError:
        return home
    if get_class(cid) is None:
        abort(400)
    session["view_class_id"] = cid
    return cid


def can_read_class(class_id: int) -> bool:
    if is_teacher():
        return get_class(class_id) is not None
    return class_id == int(session["class_id"])


def parse_teacher_class_id() -> int | tuple:
    raw = request.form.get("class_id")
    if raw is None or str(raw).strip() == "":
        cid = int(session.get("view_class_id") or session["class_id"])
        if get_class(cid) is None:
            return jsonify({"error": "invalid_class"}), 400
        return cid
    try:
        cid = int(raw)
    except ValueError:
        return jsonify({"error": "invalid_class"}), 400
    if get_class(cid) is None:
        return jsonify({"error": "invalid_class"}), 400
    return cid


@materials_bp.get("/")
@login_required()
def home():
    return redirect(url_for("materials.list_page"))


@materials_bp.get("/materials")
@login_required()
def list_page():
    class_id = viewer_class_id()
    klass = get_class(class_id)
    rows = list_materials(class_id)
    return render_template(
        "materials.html",
        materials=rows,
        role=session.get("role"),
        username=session.get("username"),
        class_id=class_id,
        class_name=klass["name"] if klass else str(class_id),
        classes=list_classes() if is_teacher() else [],
    )


@materials_bp.get("/materials/<int:material_id>")
@login_required()
def detail_page(material_id: int):
    row = get_material_by_id(material_id)
    if row is None:
        abort(404)
    if not can_read_class(row["class_id"]):
        abort(403)
    return render_template("detail.html", material=row, role=session.get("role"))


@materials_bp.get("/api/materials")
@login_required(api=True)
def list_api():
    class_id = viewer_class_id()
    rows = list_materials(class_id)
    return jsonify(
        {
            "items": [
                {
                    "id": r["id"],
                    "title": r["title"],
                    "class_id": r["class_id"],
                    "uploader": r["uploader"],
                    "created_at": r["created_at"],
                }
                for r in rows
            ]
        }
    )


@materials_bp.get("/api/materials/<int:material_id>")
@login_required(api=True)
def get_one(material_id: int):
    row = get_material_by_id(material_id)
    if row is None:
        return jsonify({"error": "not_found"}), 404
    if not can_read_class(row["class_id"]):
        return jsonify({"error": "forbidden"}), 403
    return jsonify(
        {
            "id": row["id"],
            "title": row["title"],
            "class_id": row["class_id"],
            "body_text": row["body_text"],
        }
    )


@materials_bp.post("/api/materials/upload")
@teacher_required
def upload():
    parsed = parse_teacher_class_id()
    if not isinstance(parsed, int):
        return parsed
    class_id = parsed

    before_materials = count_rows("materials")
    before_knowledge = count_rows("knowledge_entries")
    file = request.files.get("file")
    if file is None or not file.filename:
        return jsonify({"error": "missing_file"}), 400

    raw_name = secure_filename(file.filename) or "untitled.txt"
    suffix = Path(raw_name).suffix.lower()
    if suffix not in ALLOWED_EXT:
        return jsonify({"error": "unsupported_type"}), 400

    payload = file.read()
    if not payload or not payload.strip():
        return jsonify({"error": "empty_file"}), 400
    if len(payload) > MAX_BYTES:
        return jsonify({"error": "too_large"}), 400

    dest_dir = UPLOAD_DIR / str(class_id)
    dest_dir.mkdir(parents=True, exist_ok=True)
    stored_name = f"{uuid.uuid4().hex}_{raw_name}"
    dest = dest_dir / stored_name
    dest.write_bytes(payload)

    try:
        body = parse_file(dest)
    except ValueError as exc:
        dest.unlink(missing_ok=True)
        return jsonify({"error": str(exc)}), 400

    title = (request.form.get("title") or "").strip() or Path(raw_name).stem
    user_id = session["user_id"]

    try:
        with db_session() as conn:
            cur = conn.execute(
                """
                INSERT INTO materials (title, class_id, file_path, uploaded_by, body_preview)
                VALUES (?, ?, ?, ?, ?)
                """,
                (title, class_id, str(dest.as_posix()), user_id, body[:200]),
            )
            material_id = cur.lastrowid
            conn.execute(
                """
                INSERT INTO knowledge_entries (material_id, class_id, body_text)
                VALUES (?, ?, ?)
                """,
                (material_id, class_id, body),
            )
    except Exception:
        dest.unlink(missing_ok=True)
        return jsonify({"error": "db_error"}), 400

    after_materials = count_rows("materials")
    after_knowledge = count_rows("knowledge_entries")
    if after_materials != before_materials + 1 or after_knowledge != before_knowledge + 1:
        dest.unlink(missing_ok=True)
        return jsonify({"error": "db_error"}), 400

    session["view_class_id"] = class_id
    wants_html = "text/html" in (request.headers.get("Accept") or "")
    if wants_html:
        flash("已上传到班级并写入知识库")
        return redirect(url_for("materials.list_page", class_id=class_id))
    return jsonify({"material_id": material_id, "title": title, "class_id": class_id}), 201


def _wants_html() -> bool:
    return "text/html" in (request.headers.get("Accept") or "")


def _unlink_stored(path_str: str | None) -> None:
    if not path_str:
        return
    path = Path(path_str)
    if path.is_file():
        path.unlink(missing_ok=True)


@materials_bp.get("/materials/<int:material_id>/edit")
@login_required()
def edit_page(material_id: int):
    if not is_teacher():
        abort(403)
    row = get_material_by_id(material_id)
    if row is None:
        abort(404)
    return render_template(
        "edit.html",
        material=row,
        classes=list_classes(),
    )


@materials_bp.post("/materials/<int:material_id>/edit")
@teacher_required
def edit_save(material_id: int):
    row = get_material_by_id(material_id)
    if row is None:
        return jsonify({"error": "not_found"}), 404

    parsed = parse_teacher_class_id()
    if not isinstance(parsed, int):
        return parsed
    class_id = parsed
    title = (request.form.get("title") or "").strip()
    body = (request.form.get("body") or "").strip()
    if not title:
        return jsonify({"error": "title_required"}), 400

    new_path = row["file_path"]
    uploaded = request.files.get("file")
    created_file: Path | None = None
    if uploaded and uploaded.filename:
        raw_name = secure_filename(uploaded.filename) or "untitled.txt"
        suffix = Path(raw_name).suffix.lower()
        if suffix not in ALLOWED_EXT:
            return jsonify({"error": "unsupported_type"}), 400
        payload = uploaded.read()
        if not payload or not payload.strip() or len(payload) > MAX_BYTES:
            return jsonify({"error": "invalid_file"}), 400
        dest_dir = UPLOAD_DIR / str(class_id)
        dest_dir.mkdir(parents=True, exist_ok=True)
        created_file = dest_dir / f"{uuid.uuid4().hex}_{raw_name}"
        created_file.write_bytes(payload)
        try:
            body = parse_file(created_file)
        except ValueError as exc:
            created_file.unlink(missing_ok=True)
            return jsonify({"error": str(exc)}), 400
        new_path = str(created_file.as_posix())

    if not body:
        if created_file:
            created_file.unlink(missing_ok=True)
        return jsonify({"error": "body_required"}), 400

    try:
        with db_session() as conn:
            conn.execute(
                """
                UPDATE materials
                SET title = ?, class_id = ?, file_path = ?, body_preview = ?
                WHERE id = ?
                """,
                (title, class_id, new_path, body[:200], material_id),
            )
            exists = conn.execute(
                "SELECT id FROM knowledge_entries WHERE material_id = ?",
                (material_id,),
            ).fetchone()
            if exists:
                conn.execute(
                    """
                    UPDATE knowledge_entries
                    SET class_id = ?, body_text = ?
                    WHERE material_id = ?
                    """,
                    (class_id, body, material_id),
                )
            else:
                conn.execute(
                    """
                    INSERT INTO knowledge_entries (material_id, class_id, body_text)
                    VALUES (?, ?, ?)
                    """,
                    (material_id, class_id, body),
                )
    except Exception:
        if created_file:
            created_file.unlink(missing_ok=True)
        return jsonify({"error": "db_error"}), 400

    if created_file and row["file_path"] and row["file_path"] != new_path:
        _unlink_stored(row["file_path"])

    session["view_class_id"] = class_id
    if _wants_html():
        flash("已保存修改")
        return redirect(url_for("materials.detail_page", material_id=material_id))
    return jsonify({"material_id": material_id, "title": title, "class_id": class_id})


@materials_bp.post("/materials/<int:material_id>/delete")
@teacher_required
def delete_material(material_id: int):
    row = get_material_by_id(material_id)
    if row is None:
        return jsonify({"error": "not_found"}), 404
    class_id = int(row["class_id"])
    try:
        with db_session() as conn:
            conn.execute("DELETE FROM knowledge_entries WHERE material_id = ?", (material_id,))
            conn.execute("DELETE FROM materials WHERE id = ?", (material_id,))
    except Exception:
        return jsonify({"error": "db_error"}), 400
    _unlink_stored(row["file_path"])
    session["view_class_id"] = class_id
    if _wants_html():
        flash("已删除该资料")
        return redirect(url_for("materials.list_page", class_id=class_id))
    return jsonify({"deleted": material_id})
