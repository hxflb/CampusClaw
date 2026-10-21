from __future__ import annotations

from functools import wraps

import bcrypt
from flask import Blueprint, flash, jsonify, redirect, render_template, request, session, url_for

from app.db import fetch_one

auth_bp = Blueprint("auth", __name__)


def login_required(api: bool = False):
    def decorator(fn):
        @wraps(fn)
        def wrapper(*args, **kwargs):
            if not session.get("user_id"):
                if api or request.path.startswith("/api/"):
                    return jsonify({"error": "unauthorized"}), 401
                return redirect(url_for("auth.login"))
            return fn(*args, **kwargs)

        return wrapper

    return decorator


def teacher_required(fn):
    @wraps(fn)
    @login_required(api=True)
    def wrapper(*args, **kwargs):
        if session.get("role") != "teacher":
            return jsonify({"error": "forbidden"}), 403
        return fn(*args, **kwargs)

    return wrapper


@auth_bp.get("/login")
def login():
    if session.get("user_id"):
        return redirect(url_for("materials.list_page"))
    return render_template("login.html")


@auth_bp.post("/login")
def login_post():
    username = (request.form.get("username") or "").strip()
    password = request.form.get("password") or ""
    user = fetch_one(
        "SELECT id, username, password_hash, role, class_id FROM users WHERE username = ?",
        (username,),
    )
    ok = False
    if user:
        try:
            ok = bcrypt.checkpw(password.encode("utf-8"), user["password_hash"].encode("utf-8"))
        except ValueError:
            ok = False
    if not user or not ok:
        flash("用户名或密码错误")
        return render_template("login.html"), 401

    session.clear()
    session["user_id"] = user["id"]
    session["username"] = user["username"]
    session["role"] = user["role"]
    session["class_id"] = user["class_id"]
    session.permanent = True
    return redirect(url_for("materials.list_page"))


@auth_bp.get("/logout")
def logout():
    session.clear()
    return redirect(url_for("auth.login"))
