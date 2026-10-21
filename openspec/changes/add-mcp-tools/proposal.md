# 接入本班 MCP 工具

## Why

第 6 课要求解题助手在检索之外调用教师登记的工具，并且按课件的调用循环来做：模型拿到工具声明后自己决定调不调，宿主校验参数、执行、把结果作为工具消息回传，再让模型组织回答。工具由同仓的 MCP 服务提供，令牌不能进前端或 Git。

## What Changes

- 新增 `mcp-server/`，由 Compose 与主应用一起启动，提供 `GET /health` 和 `POST /mcp`。
- 教师登记本班服务器的名称、Base URL 与可选令牌（只写不读）；连接时握手并发现工具，失败停在未连接；断开后对外清单清空。
- 技能的工具可以是 `knowledge_search`，或已连接服务器上的 `工具名@服务器名`。
- 提问命中这类技能时，宿把该技能引用的工具转成工具声明随提问发给模型；模型回结构化 `tool_calls` 后，宿主按发现结果的输入结构校验参数、带令牌执行、把结果或失败原因作为工具消息回传，循环直到模型不再请求或达到上限。
- 每轮调用先推 `tool_calls` 事件再推正文；审计记录工具引用、状态、用户、会话与轮次。
- 工具返回作为不可信数据先过句式检查，命中即整条丢弃并换成固定说明。
- 补充 OpenAPI、管理页面、对话页与接口测试。不改班级隔离和现有检索主路径。

## Impact

- `mcp-server/`、`docker-compose.yml`、`backend/internal/{server,tutor,mcpclient,config,store}/`、`frontend/src/{api.ts,pages/Admin.tsx,pages/Ask.tsx}`、`frontend/public/openapi.json`、`apitest/cases/`。
