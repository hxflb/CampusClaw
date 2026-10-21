## Context

承接第 4 课：登录会话里已有角色和班级，材料可按班混合检索并标出处。本课不再做检索页，而是让一次提问走完「确认身份 → 加载提示词 → 选择技能 → 检索本班 → 流式生成」。规约以 `tutor-session` 为准。

课件：[提示词、AI 技能与流式会话](https://devops.hello1023.com/%E8%AF%BE%E4%BB%B6/%E7%AC%AC5%E8%AF%BE-%E8%AF%BE%E4%BB%B6-%E6%8F%90%E7%A4%BA%E8%AF%8DAI%E6%8A%80%E8%83%BD%E4%B8%8E%E6%B5%81%E5%BC%8F%E4%BC%9A%E8%AF%9D/index.html)。

## Goals / Non-Goals

**Goals:** 教师可保存角色提示词并开关解题引导；学生在同一会话里连续追问；同一次有依据的回答用上这三者；SSE 逐段返回并落库。

**Non-Goals:** 见 `proposal.md`。

## Decisions

### D1 提示词是角色，技能是规程

每个班一行 `class_prompts.body`。预置文案只说明「你是本班解题助手」，不写解题步骤。教师 `PUT /api/prompt` 覆盖本班；请求体里的 `class_id` 丢弃，班级只取会话。学生 PUT 返回 403。

技能规程不进提示词字段。服务端在组系统消息时另起一段：开启则要求只给思路、引用标题和位置、不直接给最终答案；关闭则要求直接作答并标注 `[n]`。本课只承认 `guide`（解题引导）。其他 key 返回 404，避免把后续课的技能提前做出来。

### D2 从下一轮生效

生成开始时读取当前提示词和技能，并写入该条助手消息的 `prompt_snapshot`、`skill_enabled`。已保存的消息不重写。教师保存之后，下一次 `POST .../messages` 才用新值。SSE 的 `meta` 带 `prompt_sha256`，便于核对换了哪一版，而不把提示词原文再塞进学生可见的回答。

### D3 先技能，再检索，再生成

顺序固定：会话用户 → 本班提示词 → 解题引导开关 → `hybrid` 取本班前 4 条 → 生成。检索班级只用会话 `class_id`。无命中时正文固定为「资料中未找到相关内容」，`citations` 为空，不调用对话网关，也不沿用上一轮出处。

有命中且配置了 `CHAT_*` 时，请求对话网关并 `stream: true`。未配置或调用失败时，服务端本地作答并仍按小段 SSE 吐出：开启时只写思路、标题和字符区间；关闭时复用第 4 课的摘句作答。两种本地正文必须不同。

### D4 会话在服务端

`conversations` 属于创建者本人且带 `class_id`。他人或他班访问返回 404。新建时必须指定本班已启用智能体。还没有消息时，可以改成另一个本班已启用智能体，只更新当前会话，不新建。已有消息后返回 409，不再更换。停用只挡住新建和这次修改。标题先写「新的提问」，首轮提问改为前 24 字，之后只刷新 `updated_at`，列表按它倒序。历史只从 `conversation_messages` 读取，取末尾 6 条非空 user/assistant，每条截断 500 字；客户端带来的 `messages`、`system` 一律丢弃。`POST /api/conversations/{id}/messages` 的正文只取 `query`。助手消息记下本轮技能名。创建者删除仍走 `DELETE /api/conversations/{id}`，消息级联删除。

解题助手页上，学生能看到解题引导当前是开启还是关闭，但不能改。会话列表展开后，每一项自带该会话的删除，不在列表外再放一个只删当前项的按钮。

### D5 SSE

`Content-Type: text/event-stream`。事件顺序：`meta`、`citations`、若干 `delta`、`done`。Nginx `/api/` 设置 `proxy_buffering off` 与更长的读超时，响应头带 `X-Accel-Buffering: no`。

### D6 第 4 课接口保留

`POST /api/search`、`POST /api/ask` 行为不变。前端 `/search` 继续跳到解题助手，本课不恢复独立检索页。

## 接口

| 路径 | 说明 |
| --- | --- |
| `GET /api/prompt` | 已登录用户读取本班提示词 |
| `PUT /api/prompt` | 仅教师，`{body}` |
| `GET /api/skills` | `{items:[{key:"guide", name:"解题引导", enabled}]}` |
| `PUT /api/skills/guide` | 仅教师，`{enabled}` |
| `GET/POST /api/conversations` | 本人的会话列表 / 新建 |
| `GET /api/conversations/{id}` | 本人消息 |
| `DELETE /api/conversations/{id}` | 创建者删除会话；消息随外键级联删除；他人 404 |
| `PUT /api/conversations/{id}/agent` | 尚无消息时更换当前会话的已启用智能体；已有消息 409 |
| `POST /api/conversations/{id}/messages` | `{query}` → SSE |

## Risks

- 对话网关不遵守「不直接给最终答案」时，本地回退路径仍保证两种开关正文不同；网关路径以系统消息中的技能规程为准。
- 旧 MySQL volume 没有新表 → 启动时 `CREATE TABLE IF NOT EXISTS` 并补预置提示词与默认开启的解题引导。
