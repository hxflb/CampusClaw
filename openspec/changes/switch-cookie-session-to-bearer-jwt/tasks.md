## 1. 令牌

- [x] 1.1 用 `SESSION_SECRET` 签发与校验 HS256 JWT，拒绝其他算法和篡改
- [x] 1.2 登录返回 `token` 并写入 `sessions.jti`；登出删除该行；不再下发 `sid` Cookie

## 2. 前端与代理

- [x] 2.1 受保护请求、SSE 和原文件下载携带 `Authorization: Bearer`
- [x] 2.2 Nginx 将 `Authorization` 转发到 api

## 3. 验收

- [x] 3.1 无令牌访问材料为 401；学生带令牌只能看到本班；登出后原令牌 401
