## Why

第 5 课任务清单要求浏览器能打开 Swagger 并试调接口，仓库里要有按 URL 分文件的接口测试和 HTML 报告，解题助手的生成要接到课程网关。登录页保持本系统原有卡片，不改成原型分栏。

## What Changes

- web 提供免登录的 `GET /openapi.json` 与 `GET /swagger/index.html`，描述现有接口的路径、入参出参、错误和是否要 Bearer。
- 新增 `apitest/`：一个接口一个脚本，`run_all.py` 生成 HTML 报告，报告不入库。核对返回值与数据库记录。
- `CHAT_BASE_URL` / `EMBEDDING_BASE_URL` 指向 [课程网关](https://ai-gateway.devops.hello1023.com/)。地址可以是站点根，也可以是控制台给出的以 `/v1` 结尾的 Base URL。密钥只放 `.env`。未配置密钥时仍用本地回答。
- 登录页恢复为系统顶栏下的原卡片，三个预置账号只负责填入表单。

## Capabilities

### New Capabilities

- `api-docs-tests`：OpenAPI、Swagger UI、接口测试报告。

### Modified Capabilities

- `tutor-session`：对话与嵌入的上游固定为课程网关的 OpenAI 兼容路径，并接受带或不带 `/v1` 的 Base URL。

## Impact

- `frontend/public/openapi.json`、`frontend/public/swagger/`、登录页样式。
- `apitest/`、`.gitignore`、`.env.example`、README。
- api 拼接 `/v1/chat/completions` 与 `/v1/embeddings` 的方式。
