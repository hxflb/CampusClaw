## Context

第 3 课将第 2 课 change 实施为可发布形态。信任边界在 API：浏览器只访问 Nginx；认证、授权、隔离与上传校验全部在 Go。Flask + SQLite 单体仅作为第 2 课示意栈，本迭代按课程需求改为前后端分离。

## Goals / Non-Goals

**Goals:**

- 登录可用，服务端会话能判定 `user_id`、`role`、`class_id`（后两项每次从用户表读取）。
- 学生上传 403；跨班按 ID 访问 **404**（与资源不存在同形）。
- 教师上传：落盘 → 解析 → 同事务写入 materials / knowledge_entries。
- Compose 三服务 `web` / `api` / `db`；仅 web 映射宿主端口；`GET /health` 只做进程存活。

**Non-Goals:** 见 `proposal.md`。另：不做 JWT/OAuth、不做多副本、不把 3306 映射到宿主、不把 uploads 当 Nginx 静态目录。

## Decisions

### D1 请求链路：浏览器 → Nginx → Go → MySQL

浏览器只访问 web。Nginx 托管前端静态资源，将 `/api` 与 `/health` 反向代理到 api。前端同源调用 `/api`，开发与生产路径一致。上传目录只挂在 api，Nginx 不得 `alias` 暴露。

**否决：** 数据库映射 3306 方便调试（扩大攻击面）；本机直接跑二进制作为发布形态。

### D2 服务端会话 Cookie，不用 JWT

会话 ID 为不透明随机串，存 MySQL `sessions` 表。Cookie：`HttpOnly`、`SameSite=Lax`，本机 HTTP 不加 `Secure`。登录换发新 ID 并删除旧会话。角色与班级不写入 Cookie、不进 localStorage。

### D3 bcrypt + 失败响应同形

口令只存 bcrypt。账号不存在时仍做一次等价耗时的哈希比较。限流（用户名+IP）触发后仍返回与口令错误相同的 JSON，不用 429。

### D4 单库 `class_id` 隔离

列表 `WHERE class_id = ?`。按 ID 先取行再校验班级（fetch-then-check），学生跨班 404。教师可查看/上传到任一已存在班级（课堂产品需求），学生忽略客户端 `class_id`。

### D5 教师修改与删除

`PUT` / `DELETE /api/materials/{id}` 仅教师；学生 403。删除同事务去掉 knowledge 行并删除磁盘文件。

### D6 配置外置、失败即停

`SESSION_SECRET`、`DB_PASSWORD`、`SEED_PASSWORD` 缺失则 api **启动失败**，无内置默认密钥。`.env` 不入库。

### D7 单实例

会话与限流在本进程 + MySQL 会话表；本迭代单实例，多副本不做。

## 接口一览

| 方法与路径 | 鉴权 | 说明 |
| --- | --- | --- |
| `POST /api/login` | 无 | 校验口令、限流、签发会话 |
| `POST /api/logout` | 会话 | 删会话并清 Cookie |
| `GET /api/me` | 会话 | 角色与班级 |
| `GET /api/classes` | 会话 | 教师全部班级；学生仅本班 |
| `GET /api/materials` | 会话 | 学生固定本班；教师可用 `class_id`；支持 `q` 搜索 |
| `GET /api/materials/{id}` | 会话 | 详情与正文；学生跨班 404 |
| `GET /api/materials/{id}/file` | 会话 | 下载原文件；学生跨班 404 |
| `POST /api/materials` | 教师 | 上传入库 201 |
| `PUT /api/materials/{id}` | 教师 | 修改标题/正文/班级/文件 |
| `DELETE /api/materials/{id}` | 教师 | 删除 |
| `GET /health` | 无 | `{"status":"ok"}`，不查库 |

## 数据与种子

表：`classes`、`users`、`sessions`、`materials`、`knowledge_entries`。`materials`/`knowledge_entries`.`class_id` NOT NULL + INDEX。

种子幂等：班级 A/B；`teacher_a`（教师，A）、`student_a1`（学生，A）、`student_b1`（学生，B）；两班可区分标题材料。口令来自 `SEED_PASSWORD`，只存哈希。

## Compose

- `web`：Nginx + 前端构建产物，映射 `${WEB_PORT:-8080}:80`
- `api`：Go；挂载 uploads volume；等待 db 就绪后监听
- `db`：MySQL 8.0，**不映射宿主端口**；named volume 持久化
- 标准启动：`cp .env.example .env` → 填写密钥 → `docker compose up --build`

## Risks

- 首次 MySQL 初始化可能短暂 502 → api 重试连库；health 不混入 DB 探活。
- 进程内限流在多副本失效 → Non-goal。
