## Why

第 3 课已能登录、按班隔离并入库全文。教师仍无法用自然语言在本班材料中定位片段，也没有可回原文的出处。本变更增加可追溯检索与有依据才生成的简短问答。

## What Changes

- 上传/重建索引后把 `knowledge_entries.body_text` 切成 `knowledge_chunks`，向量写入 Qdrant（主键 = 切片 id）。
- 检索三种模式：`keyword` / `vector` / `hybrid`（默认混合，RRF k=60）。
- 班级只从会话读取；请求体中的 `class_id` 丢弃。跨班检索表现为 200 + 空 hits。
- `POST /api/ask`：混合检索前 4 条；无命中不调用对话网关，返回固定文案。
- Compose 增加 `qdrant`，不映射宿主端口。嵌入与对话密钥只在 api 容器。

## Capabilities

### New Capabilities

- `knowledge-retrieval`：切分策略、三类检索、出处字段、问答与无依据处理、检索侧班级隔离。

### Modified Capabilities

- `auth-upload`：上传成功后追加切分与索引；删除材料时同步删除切片与向量。

## Impact

- api：切分、嵌入、Qdrant、检索与问答接口。
- MySQL：`knowledge_chunks` + ngram 全文索引。
- Compose：`qdrant` 服务。
- 前端：本班检索与问答页。

## Non-goals

- 不做流式长对话（第 5 课）、交叉编码器重排序、LangChain/LlamaIndex/Dify。
- 不把 Qdrant 或模型网关暴露给浏览器。
- 不把 6333/3306 映射到宿主。
- 不以隐藏按钮作为班级隔离。
