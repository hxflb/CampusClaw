from __future__ import annotations

import os

from flask import Flask

from app.auth import auth_bp
from app.config import DATA_DIR, UPLOAD_DIR
from app.errors import register_error_handlers
from app.materials import materials_bp


def create_app() -> Flask:
    secret = os.environ.get("SECRET_KEY")
    if not secret:
        raise RuntimeError("SECRET_KEY is required; set it via environment or .env (see .env.example)")

    DATA_DIR.mkdir(parents=True, exist_ok=True)
    UPLOAD_DIR.mkdir(parents=True, exist_ok=True)

    app = Flask(__name__)
    app.config["SECRET_KEY"] = secret
    app.config["TEMPLATES_AUTO_RELOAD"] = True
    app.config["SESSION_COOKIE_HTTPONLY"] = True
    app.config["SESSION_COOKIE_SAMESITE"] = "Lax"

    app.register_blueprint(auth_bp)
    app.register_blueprint(materials_bp)
    register_error_handlers(app)

    @app.get("/health")
    def health():
        return {"status": "ok"}

    return app
