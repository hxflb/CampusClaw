## ADDED Requirements

### Requirement: 课程网关作为对话与嵌入上游

当配置了对话基址和密钥时，系统 MUST 把聊天补全发到课程网关的 OpenAI 兼容路径。基址 MUST 接受 `https://ai-gateway.devops.hello1023.com` 与以 `/v1` 结尾的控制台 Base URL，且 MUST NOT 拼出 `/v1/v1/`。密钥 MUST 只从环境变量读取。未配置密钥时 MUST NOT 调用网关，并继续使用本地回答。

#### Scenario: 带 /v1 的基址只出现一次

- **WHEN** `CHAT_BASE_URL` 为 `https://ai-gateway.devops.hello1023.com/v1`
- **THEN** 聊天请求的路径以 `/v1/chat/completions` 结尾且不包含 `/v1/v1/`

#### Scenario: 嵌入维度与控制台一致

- **WHEN** `EMBEDDING_DIM` 为 2048，且嵌入接口返回的向量长度不是 2048
- **THEN** 该切片不写入向量库

#### Scenario: 向量库维度变化后重写

- **WHEN** 已有向量库的维度与 `EMBEDDING_DIM` 不同，服务启动
- **THEN** 向量库按新维度重建，已有切片用新向量重写，切片正文不变
