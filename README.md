# CampusClaw

**价值：** 教师规定解题助手的角色和是否给思路，学生就本班材料连续追问，回答可回溯出处。  
**场景：** 登录后按班隔离；教师保存提示词并开关解题引导；学生在同一会话里追问，回答以 SSE 逐段返回，并引用本班检索结果。  
**本学期不做：** 作业提交与批改、错题讲解、关怀式对话、异步队列、交叉编码器重排、把 Qdrant 端口映射到本机。

> 详细文档见 **[docs/project.md](docs/project.md)**（PDF 版：**[docs/project.pdf](docs/project.pdf)**）：架构与部署拓扑、接口手册与错误码、SSE 与工具循环契约、数据模型、安全设计、测试与运维、已知限制。

请求链路：**浏览器 → Nginx → Go → MySQL / Qdrant**。仅 Nginx（`web`）映射到本机端口。MCP 服务 `mcp-demo` 只在 Compose 网络内提供 `POST /mcp`，令牌来自 `.env` 的 `MCP_AUTH_TOKEN`。教师在智能体管理的 MCP 页登记 `http://mcp-demo:8090`。工具调用按第 6 课的循环走：宿主把发现到的工具声明随提问发给模型，模型提出调用、宿主按 `inputSchema` 校验参数并执行、结果作为工具消息回传后再生成；轮次与累计次数上限由 `MCP_MAX_ROUNDS`、`MCP_MAX_CALLS` 控制。

## 预置账号

口令来自 `.env` 的 `SEED_PASSWORD`（库中只存 bcrypt）。示例环境可用 `demo123`：

| 用户名 | 角色 | 班级 |
| --- | --- | --- |
| `teacher_a` | 教师 | A 班 |
| `student_a1` | 学生 | A 班 |
| `student_b1` | 学生 | B 班 |

检索班级只来自会话。学生跨班打开材料详情仍为 **HTTP 404**。跨班检索为 **200 + 空 hits**。

## Docker Compose（标准启动）

```powershell
Copy-Item .env.example .env
# 填写 SESSION_SECRET、DB_PASSWORD、MYSQL_ROOT_PASSWORD、SEED_PASSWORD
docker compose up --build
```

- 登录：http://localhost:8080/login
- 解题助手：http://localhost:8080/ask
- 接口文档：http://localhost:8080/swagger/ （OpenAPI 为 `/openapi.json`，这两处无需登录）
- `GET /health` 无需登录
- 服务：`web`、`api`、`db`、`qdrant`（**不**映射 6333/3306）

对话和嵌入走 [课程网关](https://ai-gateway.devops.hello1023.com/)。在网关登录后，把 Base URL 和 API Key 写入 `.env` 的 `CHAT_*` 与 `EMBEDDING_*`。对话模型为 `course-chat`，嵌入模型为 `course-embedding`，向量长度用 `EMBEDDING_DIM`。未填写密钥时不调用网关，回答仍由服务端根据命中材料生成。

接口测试（需要 Compose 已启动）：

```powershell
python apitest/run_all.py
```

报告写在 `apitest/report.html`，不要提交。

## 规约

- 第 3 课：`openspec/changes/add-auth-rbac-class-knowledge/`
- 第 4 课：`openspec/changes/add-traceable-vector-retrieval/`
- 第 5 课：`openspec/changes/add-prompt-skill-streaming-tutor/`
- 文档与网关：`openspec/changes/add-swagger-apitest-course-gateway/`
