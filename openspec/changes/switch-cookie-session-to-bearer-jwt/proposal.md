## Why

材料页在浏览器网络面板里应能看到 `Authorization: Bearer`，令牌本身带上用户、角色和班级。当前身份只在 HttpOnly Cookie `sid` 里，请求头不会出现这一行。

## What Changes

- `POST /api/login` 成功后返回 HS256 JWT，不再下发会话 Cookie。
- 前端把令牌放在 `sessionStorage`，访问受保护 API（含 SSE 与原文件下载）时带上 `Authorization: Bearer`。
- 服务端验签并检查过期时间；`jti` 记在 `sessions` 表，登出删除后同一令牌立即 401。
- 角色和班级仍以用户表为准。请求体或查询里的 `class_id` 不能冒充会话班级。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-upload`：登录凭证从 Cookie 会话改为 Bearer JWT，登出立即作废。

## Impact

- api：登录、登出、`require`。
- web：`api.ts` 附带令牌；材料原文件改为带令牌下载。
- Nginx：把 `Authorization` 转到 api。
- 验收脚本改为读取登录响应里的 `token`。
