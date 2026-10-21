## 1. 仓库与文档（T1）

- [x] 1.1 确认仓根 README 含价值/场景/不做三行说明 — verify: 打开 `README.md` 可见三行且与 proposal Non-goals 一致
- [x] 1.2 添加 Go 模块 `backend/`、前端 `frontend/` 与仓根 Compose — verify: 存在 `backend/go.mod`、`frontend/package.json`、`docker-compose.yml`

## 2. 数据模型与种子（T2）

- [x] 2.1 api 启动时建表：classes、users、sessions、materials、knowledge_entries，以及 lectures/homework/assistants/skills 占位表 — verify: Compose 后 MySQL 含上述表
- [x] 2.2 幂等种子：班级 A/B、teacher_a、student_a1、student_b1；A/B 班可区分标题材料；口令来自 `SEED_PASSWORD` 只存哈希 — verify: 用预置账号可登录
- [x] 2.3 列表按 `class_id` 查询 — verify: 学生 A1 列表不含 B 班标题

## 3. 登录、会话与密码（T3）

- [x] 3.1 `POST /api/login` JSON；会话 Cookie `sid` HttpOnly SameSite=Lax；`SESSION_SECRET`/`DB_PASSWORD`/`SEED_PASSWORD` 缺失则启动失败 — verify: 教师 A、学生 A1 登录后 `GET /api/me` 含 role 与 class_id
- [x] 3.2 未登录访问 API 返回 401 且无材料字段；前端引导登录页 — verify: 无 cookie 时 GET `/api/materials` 为 401
- [x] 3.3 密码 bcrypt 校验；错误密码失败 — verify: 错误口令 401，库中无明文

## 4. 班级隔离（T4）

- [x] 4.1 学生材料列表固定会话班级，忽略客户端 `class_id` — verify: A1 列表仅 A 班标题
- [x] 4.2 学生按 material id 跨班访问 404，响应无他班正文/标题 — verify: A1 GET B 班材料 id 为 404
- [x] 4.3 教师可用 `class_id` 查看/上传到已存在班级 — verify: 教师上传到 B 班后 B1 可见、A1 不可见

## 5. 角色权限与上传入库（T5）

- [x] 5.1 学生 POST 上传 403；DB 与 uploads 无新增 — verify: 学生 A1 上传后行数不变
- [x] 5.2 教师上传：落盘 → 解析 txt/md → 同事务写入 materials + knowledge_entries — verify: 201 且两表各增 1
- [x] 5.3 上传成功后本班列表可见；学生仍无法上传 — verify: A1 列表含新标题；A1 POST 仍 403
- [x] 5.4 非法扩展名 4xx 且无脏数据 — verify: `.exe` 后无孤立 DB 行
- [x] 5.5 教师 PUT/DELETE；学生调用 403 — verify: 教师可改删，学生 403

## 6. Docker Compose 与 GET /health（T6）

- [x] 6.1 `web`/`api`/`db` 三服务；仅 web 映射 `${WEB_PORT:-8080}`；db 不映射 3306 — verify: `docker compose ps` 仅 web 公布宿主端口
- [x] 6.2 `GET /health` 无需登录、不查库 — verify: `curl -sf http://localhost:8080/health` 返回 `{"status":"ok"}`
- [x] 6.3 api 连库重试 + 幂等种子 — verify: 首次 up 后可用预置账号登录
- [x] 6.4 `docker compose down` 再 `up`（不删 volume）后数据仍在 — verify: 上传后再 down/up 列表仍含该条
- [x] 6.5 README 含 Compose 步骤、登录 URL、health、`.env` 说明 — verify: 按 README 可打开登录页

## 7. 规约与收尾

- [x] 7.1 若 CLI 可用则 `openspec validate add-auth-rbac-class-knowledge --strict`
- [x] 7.2 对照 spec：跨班 404、学生上传 403、上传后列表可查 — verify: `scripts/verify_api.py`
