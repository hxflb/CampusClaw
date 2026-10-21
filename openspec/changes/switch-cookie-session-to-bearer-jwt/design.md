## Context

第 3 课用 Cookie `sid` 加 `sessions` 表判定身份。本变更只换凭证的携带方式，班级隔离、角色权限和密码哈希不变。

## Goals / Non-Goals

**Goals:** 登录响应含 JWT；浏览器请求头为 `Authorization: Bearer`；验签使用 `SESSION_SECRET`；登出后该令牌立即失效；角色与班级从用户表读取。

**Non-Goals:** 不改 OAuth、刷新令牌、多副本。不把密钥交给浏览器。不把令牌放进 URL。

## Decisions

### D1 令牌是 HS256 JWT

载荷固定为 `user_id`、`username`、`role`、`class_id`、`iat`、`exp`、`jti`。`alg` 只接受 `HS256`，签名输入是令牌前两段原文。密钥只用 `SESSION_SECRET`。过期时间沿用 `SESSION_TTL_HOURS`。

### D2 作废仍走 sessions 表

`jti` 写入现有 `sessions.id`。每次请求验签通过后，还要能按 `jti` 和 `user_id` 找到未过期行。登出删除该行。没有 `jti`、签名错误、已过期或已登出都返回 401。

角色和班级用用户表当前值，不用令牌里的声明覆盖，也不采用客户端提交的 `class_id`。

### D3 浏览器保存并附带

登录成功把 `token` 写入 `sessionStorage`。`fetch`（含解题助手 SSE 和原文件下载）设置 `Authorization: Bearer`。普通链接下载带不上该头，所以下载改为带令牌的请求再保存文件。登出先带令牌调用 `POST /api/logout`，再清掉本地令牌。不再设置 `sid` Cookie。

### D4 Nginx 转发 Authorization

`/api/` 显式转发 `Authorization`，避免只转发 Cookie 时令牌到不了 api。

## Risks

- 未登出的令牌在过期前仍然有效；登出只作废当前这枚 `jti`。
- 旧的 `sid` Cookie 不再被接受。已打开的页面需要重新登录。
