# GitHub 分支保护建议

## main 保护

在 GitHub 仓库页面进入：

```text
Settings -> Branches -> Branch protection rules
```

新增规则保护：

```text
main
```

推荐启用：

- Require a pull request before merging
- Require status checks to pass before merging
- Require branches to be up to date before merging
- Block force pushes
- Restrict deletions

## 分支约束

- `main` 不允许直接 push。
- `main` 禁止 force push。
- `dev` 可以直接 push，但必须通过 CI。
- `dev` -> `main` 通过 PR 合并。
- Release 不由 `main` push 触发，而由新的 `v*` tag 触发。
- 版本号以 Git tag 为唯一来源，不使用版本文件。

## Actions 权限

`release.yml` 需要：

```yaml
permissions:
  contents: write
```

该权限用于通过 GitHub Actions 创建或更新 GitHub Release 并上传资产。

如果仓库启用了更严格的 tag protection，需要允许维护者创建 `v*` tag；如需让 Actions 使用更高权限 token，也应使用受控的 release token。