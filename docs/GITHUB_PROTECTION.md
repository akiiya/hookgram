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
- `dev` 可以直接 push，但必须通过 CI。
- Release 由 `main` 上的 GitHub Actions 自动完成。
- 版本号由 `VERSION` 文件控制。
- 每次合并到 `main` 前必须确认 `VERSION` 尚未发布。

## Actions 权限

`release.yml` 需要：

```yaml
permissions:
  contents: write
```

该权限用于创建 `v*` tag 和 GitHub Release。

如果仓库启用了更严格的 tag protection，需要允许 GitHub Actions 创建 `v*` tag，或改用受控的 release token。
