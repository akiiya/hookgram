# Hookgram v0.1.0-rc.4 发布说明

当前开发目标：`v0.1.0-rc.4`

最新公开版本：`v0.1.0-rc.3`

版本定位：Release Candidate

`v0.1.0-rc.4` 目标是重构发布流程：日常开发只推 `dev`，用户在 GitHub 页面将变更合并到 `main` 后，由 GitHub Actions 自动完成测试、验证、tag、构建和 Release 发布。

## 版本号规则

- `VERSION` 是唯一版本号来源。
- Release Candidate：`v0.1.0-rc.1`、`v0.1.0-rc.2`、`v0.1.0-rc.3`、`v0.1.0-rc.4`
- 正式版：`v0.1.0`、`v0.1.1`、`v0.2.0`、`v1.0.0`
- `main` 发布 workflow 根据 `VERSION` 创建同名 Git tag。
- 如果 `VERSION` 对应 tag 指向其它 commit，发布 workflow 会失败，必须先升级 `VERSION`。
- 如果 tag 已在当前 `main` commit 上，发布 workflow 会复用该 tag 并继续创建或更新 GitHub Release。
- 不使用随机字符串、日期主版本号、`latest`、`final` 或其它不规则命名。

## Release 资产命名

假设 `VERSION=v0.1.0-rc.4`：

```text
hookgram-v0.1.0-rc.4-windows-amd64.zip
hookgram-v0.1.0-rc.4-windows-arm64.zip
hookgram-v0.1.0-rc.4-linux-amd64.tar.gz
hookgram-v0.1.0-rc.4-linux-arm64.tar.gz
hookgram-v0.1.0-rc.4-linux-386.tar.gz
hookgram-v0.1.0-rc.4-linux-armv7.tar.gz
hookgram-v0.1.0-rc.4-checksums.txt
```

压缩包内部二进制文件：

- Windows：`hookgram.exe`
- Linux：`hookgram`

`windows/386` 暂不发布资产：`modernc.org/sqlite` 在该目标交叉编译存在生成符号限制。

## 发布工程化

- `ci.yml`：`dev` push 和指向 `main` 的 PR 只跑必要 CI，不创建 tag，不发布 Release。
- `release.yml`：`main` push 后读取 `VERSION`，执行完整测试、前端构建、Go 测试、SQL 扫描、smoke、多平台构建、checksums、tag 创建或复用、GitHub Release 创建或更新。
- Release 构建通过 ldflags 注入 `version`、`commit`、`buildDate`。
- `/api/version` 返回版本、commit、构建时间和运行平台。
- 包含 `-rc.` 的版本自动标记为 prerelease。

## 沿用能力

- Linux 一键安装脚本下载 Release 包、安装到 `/opt/hookgram`、创建 `hookgram.service` 并启动服务。
- 安装脚本继续支持 `--uninstall`、`--purge`、`--purge --yes` 和 `--dry-run`。
- Webhook GET / POST、Telegram Bot 命令、Web 管理端、SQLite 默认存储和版本接口继续沿用。

## 默认地址

```text
http://127.0.0.1:8787
```

首次初始化：

```text
http://127.0.0.1:8787/setup
```

版本接口：

```text
http://127.0.0.1:8787/api/version
```

## 已知限制

- `v0.1.0-rc.4` 仍是 RC，不建议直接作为无人值守生产版本。
- systemd 安装仅支持 Linux，不支持 Windows。
- `--purge` 会删除 `/var/lib/hookgram` 下的配置和数据，执行前需要确认或显式传入 `--yes`。
- Session 为内存态，服务重启后需要重新登录。
- `/url` 不返回完整 Token，因为系统不保存 Webhook Token 明文。
- MySQL、MariaDB、PostgreSQL 配置入口已保留，但仍需实库验收。
- 自更新功能当前只预留版本信息和发布结构，未实现自动升级。
