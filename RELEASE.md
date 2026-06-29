# Hookgram 发布说明

当前公开版本：`v0.1.0-rc.3`

版本定位：Release Candidate

## 版本号规则

- 版本号以 Git tag 为唯一来源，不使用 `VERSION` 文件。
- Release Candidate：`v0.1.0-rc.1`、`v0.1.0-rc.2`、`v0.1.0-rc.3`。
- 正式版：`v0.1.0`、`v0.1.1`、`v0.2.0`、`v1.0.0`。
- 发版 tag 必须是新的 `vX.Y.Z` 或 `vX.Y.Z-rc.N`。
- 二进制内注入的版本号会去掉前缀 `v`。
- 日常/CI 构建使用 `git describe --tags --always --dirty` 派生版本。
- 不使用日期、随机字符串、`latest`、`final` 或其它不规则命名。

## Release 资产命名

假设发布 tag 为 `v0.1.0`：

```text
hookgram_0.1.0_linux_amd64.tar.gz
hookgram_0.1.0_linux_arm64.tar.gz
hookgram_0.1.0_linux_386.tar.gz
hookgram_0.1.0_linux_armv7.tar.gz
hookgram_0.1.0_windows_amd64.zip
hookgram_0.1.0_windows_arm64.zip
SHA256SUMS
```

压缩包内部二进制文件：

- Windows：`hookgram.exe`
- Linux：`hookgram`

`windows/386` 暂不发布资产，原因是当前纯 Go SQLite 依赖在该目标下交叉编译失败。

## 发布工程化

- `ci.yml`：`dev` push 和所有 PR 只跑 CI，不创建 tag，不发布 Release。
- `release.yml`：仅 `v*` tag push 或 `workflow_dispatch` 触发发布。
- Release workflow 执行前端构建、`go vet ./...`、`go test ./...`、多平台构建、压缩打包和 `SHA256SUMS` 生成。
- Release 构建通过 ldflags 注入 `hookgram/internal/version.Version`。
- `/api/version` 返回当前版本和运行平台。
- GitHub Release 说明由 GitHub 自动生成。

## 人工发布流程

1. 在 `dev` 用 Conventional Commits 开发。
2. 提 PR 到 `main`，等待 CI 通过并合并。
3. 创建新的发布 tag：

```bash
git checkout main
git pull
git tag v0.1.0
git push origin v0.1.0
```

也可以在 GitHub 页面 Draft a new release，Choose a tag 处输入新 tag，Target 选择 `main`，Publish 后由 Actions 自动构建并上传资产。

重发同一版本请使用 `workflow_dispatch` 指定 tag 重跑，或新建更高版本 tag。不要复用或覆盖已发布 tag。

## 沿用能力

- Linux 一键安装脚本下载 Release 包、安装到 `/opt/hookgram`、创建 `hookgram.service` 并启动服务。
- 安装脚本支持 `--uninstall`、`--purge`、`--purge --yes` 和 `--dry-run`。
- Webhook GET / POST、Telegram Bot 命令、Web 管理端、SQLite 默认存储和版本接口继续沿用。

## 已知限制

- systemd 安装仅支持 Linux，不支持 Windows。
- `--purge` 会删除 `/var/lib/hookgram` 下的配置和数据，执行前需要确认或显式传入 `--yes`。
- Session 为内存态，服务重启后需要重新登录。
- `/url` 不返回完整 Token，因为系统不保存 Webhook Token 明文。
- MySQL、MariaDB、PostgreSQL 配置入口已保留，但仍需实库验收。
- 自更新功能当前仅预留版本接口和发布结构，未实现自动升级。