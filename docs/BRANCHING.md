# Hookgram 分支策略

## 分支定义

- `main`：受保护稳定分支，只能由 PR 合并而来，不做日常直接开发。
- `dev`：日常开发分支。
- `feature/*`：功能分支，从 `dev` 派生。
- `release/*`：发布收口分支，必要时从 `dev` 派生。
- `hotfix/*`：紧急修复分支，修复后合并回 `main` 和 `dev`。

## 日常流程

1. 本地开发提交到 `dev`，提交遵循 Conventional Commits。
2. 推送 `dev`，触发 CI。
3. CI 通过后，在 GitHub 页面发起 PR：`dev` -> `main`。
4. PR CI 通过并审批后合并到 `main`。
5. 如需发版，在 `main` 最新提交上创建新的 `v*` tag。
6. Release workflow 由 tag 触发，自动测试、构建、打包并发布 GitHub Release。
7. 发布后继续回到 `dev` 开发。

## 版本规则

- Git tag 是唯一版本号来源。
- 不使用 `VERSION` 文件。
- 发布 tag 必须是新的 `vX.Y.Z` 或 `vX.Y.Z-rc.N`。
- 二进制注入版本号时去掉前缀 `v`。
- 日常/CI 构建版本由 `git describe --tags --always --dirty` 派生。
- 不使用日期、随机字符串、`latest`、`final` 或 `release-final`。
- 不覆盖已有 tag。

## main 规则

- 不直接在 `main` 提交。
- 不 force push `main`。
- 不在 `main` 上继续日常开发。
- 不手工上传 Release 资产。
- 发布只通过新的 `v*` tag 触发 Release workflow。

## 日常开发示例

```bash
git checkout dev
git checkout -b feature/check-update
```

## 发布示例

```bash
git checkout main
git pull
git tag v0.1.0
git push origin v0.1.0
```

也可以在 GitHub 页面 Draft a new release，Choose a tag 处输入新 tag，Target 选择 `main`，Publish 后由 Actions 自动构建并上传资产。