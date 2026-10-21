## Context

承接第 3 课 Nginx → Go → MySQL。本课在材料入库之后增加切分、嵌入、Qdrant 与问答。规约以第 4 课 `knowledge-retrieval` 为准。课件：[可追溯知识库检索](https://devops.hello1023.com/%E8%AF%BE%E4%BB%B6/%E7%AC%AC4%E8%AF%BE-%E8%AF%BE%E4%BB%B6-%E5%8F%AF%E8%BF%BD%E6%BA%AF%E7%9F%A5%E8%AF%86%E5%BA%93%E6%A3%80%E7%B4%A2/)。

## Goals / Non-Goals

**Goals:** 本班自然语言检索可回溯；三种模式；无切片不生成；Qdrant 仅内网。

**Non-Goals:** 见 `proposal.md`。

## Decisions

### D1 正文在 MySQL，向量在 Qdrant

`knowledge_chunks.chunk_text` 与偏移在 MySQL。Qdrant 集合 `campusclaw_chunks`，余弦距离，点 id = `knowledge_chunks.id`。payload 仅 `class_id`、`material_id`、`knowledge_entry_id`、`chunk_id`、`chunk_index`。摘录永远回表，不读 payload 正文。

### D2 三种检索

| 模式 | 行为 |
| --- | --- |
| keyword | MySQL `FULLTEXT ... WITH PARSER ngram`（token 2），不嵌入、不访问 Qdrant |
| vector | 问句嵌入后 Qdrant 按会话 `class_id` 过滤，余弦 < 0.35 丢弃，再回表 |
| hybrid | 两路独立过滤后 RRF（k=60）；缺席一路不计分。关键词一路再丢掉得分不足本路最高分四成的切片，避免长材料因个别二字偶合进入依据。默认 |

空查询 400。Qdrant 不可用时 keyword 仍可用；vector/hybrid 503。

### D3 班级只来自会话

检索与问答丢弃 query/body/header 中的 `class_id`。学生与教师检索均用会话班级（教师不能靠请求改班偷看）。跨班词 → 200 且 hits 空，不用 403。

### D4 切分

`auto`：最大 800 字、重叠 80，优先空行/换行/句号。种子与未指定策略用 auto。  
`custom`：最大 100–2000，重叠 0–50%，可去 URL/邮箱、折叠空白。  
`hierarchy`：按 `#`/`##`/`###` 分章，过长再 auto。  
预处理不改 `body_text`。重建须显式请求，先删旧切片与旧向量。

### D5 嵌入与对话网关

仅 api 调用。配置了 `EMBEDDING_*` 则走兼容 OpenAI Embeddings 的课程/网关；未配置则用服务端确定性哈希向量（384 维）保证本机可复现，密钥仍不下发浏览器。对话同理：配置了 `CHAT_*` 才调用生成；否则从命中切片里抽出最贴近问题的句子作为短答并打 `[n]`，不把切片原文再贴进回答。无命中仍不「编造」。

### D6 问答

`POST /api/ask` 先 hybrid 取本班前 4 条。无命中：固定「资料中未找到相关内容」，citations 空，不调对话。有命中：标题+序号+正文交给生成，要求直接回答问题，句末用 `[n]` 标注；切片原文只出现在 citations，不重复进回答正文。丢弃客户端 `system`。非流式。未配置对话网关时，服务端从命中句中选取与问题最接近的一句作答。

### D7 Compose

服务 `web`/`api`/`db`/`qdrant`。仅 web 映射宿主端口。`GET /health` 仍不探活 DB/Qdrant。

## 接口

| 路径 | 说明 |
| --- | --- |
| `POST /api/search` | `{query, mode}`；hits 含标题、chunk_index、字符区间、摘录、分路 score/rank；vector/hybrid 可含 `query_vector` |
| `POST /api/ask` | 问答 |
| `POST /api/materials/{id}/reindex` | 教师重建 |

## Risks

- 本地哈希嵌入同义召回弱 → 有网关时替换，不改接口。
- 旧 MySQL volume 无 ngram 表 → 启动时 `CREATE TABLE IF NOT EXISTS`。
