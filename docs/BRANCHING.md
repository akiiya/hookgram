# Hookgram 分支策略

## 分支定义

- `main`：稳定主分支，只接受合并，不做日常直接开发。
- `dev`：日常开发分支。
- `feature/*`：功能分支，从 `dev` 派生。
- `release/*`：发布前收口分支。
- `hotfix/*`：紧急修复分支，可在修复后合并回 `main` 和 `dev`。

## 首次推送

第一次将当前项目推送到 `main`。推送成功后立即创建 `dev`，后续开发切换到 `dev`，不再直接在 `main` 开发。

```bash
git branch -M main
git add .
git commit -m "chore: bootstrap Hookgram v0.1.0-rc.1"
git push -u origin main
git checkout -b dev
git push -u origin dev
```

## main 合并规则

- `main` 只能通过 PR 或明确合并从 `dev`、`release/*`、`hotfix/*` 合入。
- `main` 每次合并后触发 GitHub Actions 自动化测试。
- 测试通过后继续构建 Release 候选产物。
- 发布版本使用标准 tag，例如 `v0.1.0-rc.1`。

## 后续开发

```bash
git checkout dev
git checkout -b feature/check-update
```

## 注意事项

- 本轮除非 remote origin 已确认且用户明确授权，否则不要执行 `git push`。
- 无法确认 remote 时，只输出建议命令，不强行推送。
- Git tag 必须与版本号一致，例如 `v0.1.0-rc.1`。
