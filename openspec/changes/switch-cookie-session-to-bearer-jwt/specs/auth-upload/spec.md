## MODIFIED Requirements

### Requirement: 用户登录

系统 MUST 提供教师（teacher）、学生（student）两类角色的账号密码登录。登录成功后 MUST 返回 HS256 JWT，并在服务端登记该令牌的 `jti`。受保护 API MUST 从 `Authorization: Bearer` 读取令牌。未登录用户访问受保护页面 MUST 被引导到登录页，且 MUST NOT 泄露任何业务数据。系统 MUST NOT 把访问令牌写入 Cookie。

#### Scenario: 教师与学生登录成功

- **WHEN** 用户使用有效账号与密码发起登录（预置账号含教师 A、学生 A1、学生 B1）
- **THEN** 响应 MUST 含 `token`
- **AND** 令牌载荷 MUST 含 `user_id`、`username`、`role`、`class_id`、`iat`、`exp`、`jti`
- **AND** 使用该令牌调用 `GET /api/me` MUST 返回同一角色与班级

#### Scenario: 材料请求携带 Bearer

- **WHEN** 学生 A1 登录后请求 `GET /api/materials`，请求头为 `Authorization: Bearer` 加上登录返回的令牌
- **THEN** 响应 MUST 为 HTTP 200
- **AND** 返回材料 MUST 都属于该学生所在班级

#### Scenario: 错误密码登录失败

- **WHEN** 用户提供存在用户名但错误密码
- **THEN** 登录 MUST 失败
- **AND** 系统 MUST NOT 返回 `token`
- **AND** 响应 MUST NOT 包含密码哈希

#### Scenario: 未登录访问受保护页面

- **WHEN** 未携带有效 Bearer 令牌的用户请求受保护页面（如材料列表）或受保护 API
- **THEN** 对页面请求 MUST 重定向到登录页
- **AND** 对 API 请求 MUST 返回 HTTP 401
- **AND** 响应 MUST NOT 包含任何本班或他班材料标题、正文或文件路径

#### Scenario: 登出后令牌立即失效

- **WHEN** 已登录用户调用 `POST /api/logout` 后，再用同一令牌访问 `GET /api/materials`
- **THEN** MUST 返回 HTTP 401

#### Scenario: 伪造或篡改令牌被拒绝

- **WHEN** 请求携带签名错误、算法不是 HS256、或已过期的令牌
- **THEN** MUST 返回 HTTP 401
- **AND** MUST NOT 返回材料内容

### Requirement: 密码哈希与会话密钥

系统 MUST 使用单向密码哈希算法存储用户密码，禁止明文。JWT 签名密钥 `SESSION_SECRET` MUST 仅通过服务端环境变量或 Compose 注入，MUST NOT 硬编码在源码或提交到版本库，MUST NOT 下发给浏览器。

#### Scenario: 库中无明文密码

- **WHEN** 直接查询用户表中密码相关字段（如 `password_hash`）
- **THEN** 字段值 MUST NOT 等于任何已知预置账号的明文口令
- **AND** MUST 可识别为哈希格式（如 bcrypt `$2` 前缀或 argon2 标识）

#### Scenario: 登录校验使用哈希比较

- **WHEN** 用户使用正确明文密码登录
- **THEN** 系统 MUST 通过哈希验证通过并签发令牌
- **WHEN** 同一用户使用错误密码登录
- **THEN** 验证 MUST 失败且不签发令牌

#### Scenario: 缺少会话密钥时服务不得静默使用默认值

- **WHEN** 生产/Compose 配置未提供会话签名必需的环境变量
- **THEN** 应用启动 MUST 失败；`.env.example` MUST 列出所需变量名

## ADDED Requirements

### Requirement: 身份以用户表为准

验签通过后，系统 MUST 用令牌中的 `user_id` 读取用户表里的角色和班级，并以此做授权与班级过滤。令牌中的 `role`、`class_id` 以及请求里的 `class_id` MUST NOT 单独决定学生能看到哪个班。

#### Scenario: 请求里的班级不能覆盖令牌用户

- **WHEN** 学生 A1 携带自己的 Bearer 令牌，并在材料列表请求中指定 B 班 `class_id`
- **THEN** 返回材料 MUST 仍只属于 A 班
