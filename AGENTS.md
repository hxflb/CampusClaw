# CampusClaw 项目规则

- **无规约不写代码。** 业务实现必须对照 `openspec/changes/` 下当前 change 的 `proposal.md`、`design.md`、`specs/**/spec.md` 与 `tasks.md`。
- 口头约定、聊天记录不作为验收依据；范围变更先改 Markdown，再实现。
- 班级隔离与权限必须在服务端判定；不得仅靠前端隐藏按钮。
- 密码禁止明文存储；`SESSION_SECRET` 只从环境变量注入，禁止写入源码或提交真实 `.env`。
- 本仓库标准启动方式为 Docker Compose（web/api/db/qdrant）；仅 web 映射宿主端口；`GET /health` 无需登录、不探活数据库。
- 检索班级只从会话读取；Qdrant 与嵌入/对话密钥不得暴露给浏览器。
