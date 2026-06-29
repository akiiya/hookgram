# Hookgram 分支策略

## 分支定义

- `main`：保护分支，只能由 PR 或 GitHub 页面 merge 而来，不做日常直接开发。
- `dev`：日常开发分支。
- `feature/*`：功能分支，从 `dev` 派生。
- `release/*`：发布收口分支，必要时从 `dev` 派生。
- `hotfix/*`：紧急修复分支，修复后合并回 `main` 和 `dev`。

## 发布流程

1. 本地开发提交到 `dev`。
2. 推送 `dev`，触发 CI。
3. CI 通过后，用户在 GitHub 页面发起 PR 或将 `dev` merge 到 `main`。
4. `main` push 触发 `release.yml`。
5. `release.yml` 自动完成测试、验证、构建、tag 和 GitHub Release。
6. 发布后继续回到 `dev` 开发。

## 版本规则

- `VERSION` 是唯一版本号来源。
- 每次合并 `main` 前必须升级 `VERSION`。
- 如果 `VERSION` 对应 tag 指向其它 commit，发布 workflow 会失败。
- 如果 tag 已在当前 `main` commit 上，发布 workflow 会复用该 tag 并继续创建或更新 GitHub Release。
- Git tag 必须与 `VERSION` 完全一致，例如 `v0.1.0-rc.4`。
- 不使用日期、随机字符串、`latest`、`final` 或 `release-final`。

## main 规则

- 不直接在 `main` 提交。
- 不在本地手动打包发布。
- 不人工创建 GitHub Release。
- 不覆盖已有 tag。
- `main` 合并后由 GitHub Actions 自动创建 tag 和 Release。

## 日常开发示例

```bash
git checkout dev
git checkout -b feature/check-update
```

## 发布前示例

```bash
git checkout dev
printf "v0.1.0-rc.5\n" > VERSION
git add VERSION
git commit -m "chore: bump version to v0.1.0-rc.5"
git push origin dev
```

随后在 GitHub 页面将 `dev` 或 PR 合并到 `main`。
