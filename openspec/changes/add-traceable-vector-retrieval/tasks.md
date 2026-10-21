## 1. 数据与 Compose

- [x] 1.1 增加 `knowledge_chunks`（含 ngram 全文、class_id、index_status、偏移）
- [x] 1.2 Compose 增加 `qdrant`，不映射 6333；集合 `campusclaw_chunks` 余弦
- [x] 1.3 种子材料 auto 切分并索引

## 2. 切分与重建

- [x] 2.1 实现 auto / custom / hierarchy
- [x] 2.2 教师 `POST /api/materials/{id}/reindex` 删旧再建
- [x] 2.3 预处理不改 body_text

## 3. 检索

- [x] 3.1 `POST /api/search` 三种 mode，默认 hybrid，空查询 400
- [x] 3.2 keyword 不调嵌入；vector 阈值 0.35；hybrid RRF k=60
- [x] 3.5 hybrid 丢掉不足关键词最高分四成的切片，避免新资料因偶合进入依据
- [x] 3.3 会话班级过滤；伪造 class_id 无效；跨班词 hits 空
- [x] 3.4 Qdrant 停掉后 keyword 200、vector/hybrid 503 — 按 design 实现，可手工停容器验证

## 4. 问答与前端

- [x] 4.1 `POST /api/ask` 无命中固定文案且不调聊天
- [x] 4.2 有命中 citations 与 [n] 对齐
- [x] 4.5 回答正文直接作答，资料原文只留在出处列表
- [x] 4.3 前端检索/问答页，密钥不出现在浏览器
- [x] 4.4 上传成功后自动切分索引
