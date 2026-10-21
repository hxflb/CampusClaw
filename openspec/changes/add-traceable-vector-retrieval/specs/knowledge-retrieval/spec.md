## Purpose

在已登录用户的本班材料中，按关键字、向量或混合模式检索可回溯切片；无依据时不得编造出处或调用生成模型。

## ADDED Requirements

### Requirement: 切分与双库入库

上传或重建索引后，系统 MUST 将知识库正文切为切片写入 `knowledge_chunks`，并将对应向量写入 Qdrant 集合 `campusclaw_chunks`（余弦）；向量点 id MUST 等于 `knowledge_chunks.id`。payload MUST NOT 含切片正文。未指定策略时 MUST 使用 `auto`（约 800 字、重叠 80 字）。种子材料启动补齐 MUST 使用 `auto`。

#### Scenario: 上传后产生切片与向量

- **WHEN** 教师成功上传合法 txt/md
- **THEN** MySQL MUST 新增至少一条 `index_status=ready` 的切片（嵌入成功时）
- **AND** Qdrant 中 MUST 存在相同 id 的点
- **AND** `knowledge_entries.body_text` MUST 仍为原文

#### Scenario: 嵌入失败不写残缺向量

- **WHEN** 嵌入调用失败
- **THEN** 材料与知识库原文 MUST 保留
- **AND** 对应切片 MUST 标记 failed
- **AND** Qdrant MUST NOT 写入该切片的不完整点

#### Scenario: 重建索引替换旧切片

- **WHEN** 教师对已有材料按新策略重建索引
- **THEN** 旧切片行与旧向量点 MUST 删除后再写入新切片
- **AND** 未重建的材料切片 MUST 保持不变

### Requirement: 三种检索模式

系统 MUST 提供 `keyword`、`vector`、`hybrid`（默认 hybrid）。空查询 MUST 返回 400。keyword MUST 只查 MySQL ngram 全文且不产生问句向量。vector MUST 丢弃余弦相似度低于 0.35 的候选。hybrid MUST 在合并前丢掉关键词得分不足本路最高分四成的切片，再对两路剩余名次做 RRF（k=60）。keyword 模式本身仍返回全文检索的原始结果。hits MUST 含材料标题、切片序号、字符区间与摘录（摘录取自 MySQL `chunk_text`）。

#### Scenario: 关键字命中原文词

- **WHEN** 已登录用户以 keyword 检索本班原文中出现的词
- **THEN** hits 非空且摘录来自本班切片
- **AND** 响应 MUST NOT 包含问句向量（或明确不调用嵌入）

#### Scenario: 向量过滤低分

- **WHEN** vector 模式得到余弦低于 0.35 的候选
- **THEN** 这些候选 MUST NOT 出现在 hits 中

#### Scenario: 混合检索丢掉过弱的关键词命中

- **WHEN** 问句在关键词一路的最高分来自相关材料，另一材料的关键词得分不足该最高分的四成，且向量一路没有命中它
- **THEN** hybrid 的 hits MUST NOT 包含那篇低分材料

#### Scenario: 无依据返回空命中

- **WHEN** 所选模式过滤后无候选
- **THEN** HTTP 200 且 hits 为空（或等价空列表）
- **AND** MUST NOT 用低相关切片凑数

#### Scenario: Qdrant 不可用时降级

- **WHEN** Qdrant 不可用
- **THEN** keyword MUST 仍可返回正文
- **AND** vector 与 hybrid MUST 返回 503

### Requirement: 检索侧班级隔离

检索与问答的班级 MUST 仅来自登录会话。请求中的 `class_id` MUST 丢弃。关键字与向量路径及回表 MUST 均按会话班级过滤，且仅 `index_status=ready`。跨班检索 MUST 表现为 200 且 hits 为空，MUST NOT 用 403 暗示他班存在资料。

#### Scenario: 请求体伪造班级无效

- **WHEN** A 班用户检索并在 JSON 中携带 B 班 `class_id`，使用仅存在于 B 班的词
- **THEN** 响应 MUST 为 200 且 hits 为空

#### Scenario: 跨班详情仍为 404

- **WHEN** A 班用户打开 B 班材料详情
- **THEN** MUST 仍为 HTTP 404（第 3 课规则）

### Requirement: 问答有依据才生成

`POST /api/ask` MUST 先对本班做 hybrid 并取前 4 条。无命中 MUST 返回「资料中未找到相关内容」，citations 为空，MUST NOT 调用对话网关。有命中时回答正文 MUST 直接回答问题，并用 `[n]` 指向依据；切片原文 MUST 只出现在 citations，不得整段贴进回答正文。`[n]` MUST 与 citations 顺序一致。客户端 `system` MUST 被丢弃。

#### Scenario: 无关问题不调用生成

- **WHEN** 用户询问本班材料不可能出现的内容（如天气）
- **THEN** 200，正文为固定未找到文案，citations 为空

#### Scenario: 有依据时标注出处

- **WHEN** 本班材料中确有依据
- **THEN** 回答含 `[1]` 等标注且与 citations 列表顺序一致

#### Scenario: 回答不重复贴出资料原文

- **WHEN** 本班材料中确有依据
- **THEN** 回答正文是针对问题的短句，citations 中仍保留切片摘录
