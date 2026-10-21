## ADDED Requirements

### Requirement: OpenAPI 与 Swagger

系统 MUST 在未登录时提供 `GET /openapi.json` 与 Swagger UI 页面。文档 MUST 列出现有接口的方法、路径、主要入参出参，并标明哪些接口需要 `Authorization: Bearer`。文档 MUST NOT 描述本仓库没有的路径。

#### Scenario: 打开文档

- **WHEN** 未登录用户请求 `GET /openapi.json` 和 Swagger 页面
- **THEN** 两者都返回 HTTP 200
- **AND** OpenAPI 中 `POST /api/login` 不要求 Bearer
- **AND** `GET /api/materials` 要求 Bearer

### Requirement: 按接口存放的测试与报告

仓库 MUST 含 `apitest/run_all.py` 与按 URL 放置的用例，例如 `apitest/cases/api/login/POST.py` 对应 `POST /api/login`。一键运行 MUST 生成 HTML 报告。报告文件 MUST NOT 被要求提交。用例 MUST 至少覆盖登录、检索、提示词或技能、创建会话并发送一条消息，并且 MUST 将接口返回与数据库中的对应记录核对。

#### Scenario: 登录结果与用户表一致

- **WHEN** 用例使用预置学生账号登录并读取 `GET /api/me`
- **THEN** 返回的用户名、角色和班级与 `users` 表中该账号的行一致

#### Scenario: 会话消息写入数据库

- **WHEN** 用例创建会话并发送一条非空问题
- **THEN** `conversation_messages` 中存在该会话的用户消息，正文与提交的问题一致
