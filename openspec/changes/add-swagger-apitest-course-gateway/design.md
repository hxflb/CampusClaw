## Context

解题助手、Bearer 登录、本班检索和 SSE 已经在仓库里。本变更补任务清单的文档、测试和课程网关接入。登录页保持本系统原有卡片样式，不采用原型的整页分栏。

课件：[第 5 课概述](https://devops.hello1023.com/%E8%AF%BE%E4%BB%B6/%E7%AC%AC5%E8%AF%BE-%E8%AF%BE%E4%BB%B6-%E6%8F%90%E7%A4%BA%E8%AF%8DAI%E6%8A%80%E8%83%BD%E4%B8%8E%E6%B5%81%E5%BC%8F%E4%BC%9A%E8%AF%9D/index.html)。任务清单：[第 5 课](https://devops.hello1023.com/%E8%AF%BE%E7%A8%8B%E4%BB%BB%E5%8A%A1%E6%B8%85%E5%8D%95-%E7%AC%AC5%E8%AF%BE.html)。

## Decisions

### D1 文档描述已经实现的接口

`openapi.json` 只写本仓库真实路径，使用 `bearerAuth`。不把原型里的智能体、笔记、资源夹写进来。Swagger UI 静态放在 web，`/openapi.json` 与 `/swagger/` 不经过登录。

### D2 测试目录按 URL

`apitest/cases/api/login/POST.py` 表示 `POST /api/login`。路径参数用 `_id_` 目录。`run_all.py` 按文件名顺序执行，写出 `apitest/report.html`。该报告列入 `.gitignore`。需要核对库时，通过 `docker compose exec` 查询 MySQL，不把 3306 映射到宿主。

### D3 课程网关

控制台给出的 Base URL 是 `https://ai-gateway.devops.hello1023.com/v1`。代码若发现基址已以 `/v1` 结尾，就直接接 `/chat/completions` 或 `/embeddings`，否则补上 `/v1`。对话模型用控制台的 `course-chat`，嵌入模型用 `course-embedding`。嵌入请求只提交 `model` 和 `input`。`EMBEDDING_DIM` 是向量长度，用来建库并核对返回维度，不放进请求。设置了 `EMBEDDING_DIM` 时它优先于 `VECTOR_SIZE`。`CHAT_API_KEY` 与 `EMBEDDING_API_KEY` 只来自环境变量。缺密钥时不请求网关。已有向量库的维度和配置不一致时，重建该库并用新向量重写已有切片，不改切片正文。

### D4 登录页保持原样式

未登录时仍在系统顶栏下显示原来的登录卡片：标题、用户名、密码，以及「教师 A / 学生 A1 / 学生 B1」三个填入按钮。配色与材料页相同。解题助手和教师上传页的布局见 `align-studio-and-upload`。

## Non-Goals

不实现作业、错题、关怀式对话、异步队列，也不实现原型 OpenAPI 里的 agents、notes、folders、resources。
