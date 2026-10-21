# 教师管理本班智能体

## Why

学生界面不需要接口文档。教师要在自己的班级里管理智能体：名称、角色提示词、是否启用，以及勾选技能和本班材料。

## What Changes

- 学生导航不显示「接口文档」。教师仍可打开。
- 教师增加班级管理页，分页为学生、资料、智能体、技能。智能体页可新建和保存。
- 服务端按登录用户的班级读写智能体。学生不能创建或修改。未启用的智能体对学生不可见。
- 解题助手使用已启用智能体的提示词。勾选「解题引导」时不给最终答案。勾选了材料时，检索只留这些材料。

## Impact

- `backend/internal/store/store.go`、`backend/internal/server/agents.go`、`tutor.go`
- `frontend/src/pages/Admin.tsx`、`Ask.tsx`、`App.tsx`、`api.ts`
