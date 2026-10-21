# 403 跨班访问：响应不含他班标题/正文/路径
from flask import render_template_string

ERROR_PAGE = """
{% extends "base.html" %}
{% block title %}无法访问{% endblock %}
{% block body %}
<section class="card">
  <div class="card-icon"><i class="fa-solid fa-shield-halved" aria-hidden="true"></i></div>
  <h1>无法访问</h1>
  <p class="lead">你没有权限查看该资源，或资源不属于当前班级。</p>
  <p class="crumb">
    <a class="btn btn-ghost" href="{{ url_for('materials.list_page') }}">
      <i class="fa-solid fa-arrow-left" aria-hidden="true"></i> 返回本班列表
    </a>
  </p>
</section>
{% endblock %}
"""


def register_error_handlers(app):
    @app.errorhandler(403)
    def forbidden(_e):
        return render_template_string(ERROR_PAGE), 403

    @app.errorhandler(404)
    def not_found(_e):
        return render_template_string(ERROR_PAGE), 404
