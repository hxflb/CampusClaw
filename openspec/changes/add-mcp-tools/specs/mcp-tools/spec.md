## ADDED Requirements

### Requirement: 自建 MCP 服务

系统 MUST 在同一 Compose 项目中提供 MCP 服务。该服务 MUST 提供不鉴权的 `GET /health`，以及 Bearer 保护的 `POST /mcp`。`POST /mcp` MUST 实现 `initialize`、`tools/list` 和 `tools/call`。缺少令牌或令牌错误时 MUST 返回 401。非法请求 MUST 返回可读的 JSON-RPC 错误。`tools/list` MUST 包含 `formula_hint`、`unit_convert` 和 `weather_query`。这些名称 MUST 以小写字母开头且不含连字符。令牌 MUST 只来自环境变量，MUST NOT 写入镜像或前端。

#### Scenario: 带令牌发现工具

- **WHEN** 使用正确的 Bearer 调用 `tools/list`
- **THEN** 返回 `formula_hint`、`unit_convert` 和 `weather_query`

#### Scenario: 令牌错误

- **WHEN** 请求不带令牌或令牌不正确
- **THEN** 响应状态为 401

### Requirement: 教师登记并连接

教师 MUST 能登记本班 MCP 服务器的名称和 Base URL。接口响应 MUST NOT 包含令牌。连接 MUST 完成握手并缓存 `tools/list` 的工具。断开 MUST 清空该服务器的工具缓存，之后不能再调用。连接失败 MUST 返回明确错误，且不改变已有检索和对话。学生 MUST 收到 403。其他班的服务器 MUST 返回 404。

#### Scenario: 连接后看到工具名

- **WHEN** 教师把 Base URL 指到 Compose 中的 MCP 服务并连接
- **THEN** 管理界面列出 `formula_hint`、`unit_convert` 和 `weather_query`，响应中没有令牌

#### Scenario: 学生不能登记

- **WHEN** 学生调用登记、连接、断开或审计接口
- **THEN** 响应状态为 403

### Requirement: 技能绑定后调用

技能工具 MUST 允许 `knowledge_search` 或 `工具名@服务器名`。保存后者时，该工具 MUST 属于本班、服务器已连接且已在缓存中，否则 MUST 拒绝保存。学生提问使用勾选了该技能的已启用智能体时，服务端 MUST 带令牌调用 `tools/call`，并把文本作为数据放入用户消息，而不是系统指令。流式 `meta` MUST 包含 `tool_calls`，其中有工具引用和状态。未连接或越权时状态 MUST 为 `denied`。

#### Scenario: 提问留下成功审计

- **WHEN** 已连接的服务器上保存了含 `formula_hint@服务器名` 的技能，并且该技能勾选在当前智能体上，然后发送问题
- **THEN** `tool_calls` 中该引用的状态为 `ok`，审计表有对应记录

#### Scenario: 天气查询

- **WHEN** 已连接的服务器提供 `weather_query`，技能工具写成 `weather_query@服务器名` 并被当前智能体选中，学生提问「北京今天天气怎么样」
- **THEN** 工具返回的文本含城市和气温；检索没有命中时，回答采用这段文本，且不调用对话网关

#### Scenario: 断开后拒绝调用

- **WHEN** 教师断开服务器后，同一技能再次参与提问或再次保存
- **THEN** 调用状态为 `denied`，或保存被拒绝

### Requirement: 审计与文档

教师 MUST 能查看本班审计，每条至少含工具引用、状态、用户和时间。OpenAPI MUST 描述登记、连接、断开、审计和技能保存这些路径。Swagger 页面 MUST 能在未登录时打开。
