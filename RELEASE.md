# Hookgram v0.1.0-rc.1 发布说明

当前版本：`v0.1.0-rc.1`  
版本定位：Release Candidate

该版本用于真实环境验收、GitHub 开源发布准备和后续小范围迭代基线。

## 版本号规则

- Release Candidate：`v0.1.0-rc.1`、`v0.1.0-rc.2`
- 正式版：`v0.1.0`、`v0.1.1`、`v0.2.0`、`v1.0.0`
- Git tag 必须与版本号一致，例如 `v0.1.0-rc.1`
- 不使用随机字符串、日期主版本号或其它不规则命名。

## Release 资产命名

```text
hookgram-v0.1.0-rc.1-windows-amd64.zip
hookgram-v0.1.0-rc.1-windows-arm64.zip
hookgram-v0.1.0-rc.1-linux-amd64.tar.gz
hookgram-v0.1.0-rc.1-linux-arm64.tar.gz
hookgram-v0.1.0-rc.1-checksums.txt
```

压缩包内部二进制文件：

- Windows：`hookgram.exe`
- Linux：`hookgram`

## 已完成能力

- 无配置启动，自动创建配置和 SQLite 数据库。
- 首次初始化管理员账号、Telegram Bot Token、API Proxy、Base URL。
- Telegram Bot long polling，支持 `/start`、`/help`、`/list`、`/add`、`/del`、`/rename`、`/url`、`/usage`。
- Webhook GET / POST 推送，支持 JSON、纯文本、表单、Markdown、HTML 和 `fields`。
- Web 管理端查看 Bot 用户、Token、推送记录和系统设置。
- 前端构建产物通过 Go embed 内嵌进可执行文件。
- 默认 SQLite 使用 `github.com/glebarez/sqlite`，为纯 Go GORM SQLite 驱动，适合跨平台构建。

## 发布工程化

- 新增 GitHub Actions CI：测试、gofmt、go mod tidy、原生 SQL 扫描、前端构建、Go build、最小启动检查。
- 新增 GitHub Actions Release：tag 或手动触发，多平台构建、压缩、checksums、GitHub Release 上传。
- 新增 Linux 一键安装脚本：下载 Release 包、安装到 `/opt/hookgram`、创建 `hookgram.service`、启动服务。
- Linux 安装脚本支持 `--uninstall` 保留数据卸载、`--purge` 彻底卸载和 `--dry-run` 预览操作。
- 新增 `/api/version`：返回版本、commit、构建时间和平台。
- 管理端系统设置页展示当前版本信息。
- 为后续 Web 管理端检查更新和程序自更新预留版本结构；当前未实现自动升级。

## 支持目标

Release 工作流构建：

- `windows/amd64`
- `windows/arm64`
- `linux/amd64`
- `linux/arm64`
- `linux/386`
- `linux/arm/v7`

暂不支持 `windows/386`：本地交叉编译验证中 `modernc.org/sqlite` 在该目标下缺少生成符号，不能作为真实可用资产发布。

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

## 验收状态

- 真实 Telegram live 投递测试：未执行，原因：本地 `data/config.yaml` 中缺少真实 Telegram Bot Token。
- MySQL/MariaDB/PostgreSQL 实库验证：未执行，原因：本轮聚焦默认 SQLite 和发布工程化。
- Linux 一键安装脚本：依赖 GitHub Release 资产存在。
- Windows 双击启动：需要真实桌面环境验证。

## 已知限制

- `v0.1.0-rc.1` 仍是 RC，不建议直接作为无人值守生产版本。
- systemd 安装仅支持 Linux，不支持 Windows。
- `--purge` 会删除 `/var/lib/hookgram` 下的配置和数据，执行前需要确认或显式传入 `--yes`。
- Session 为内存态，服务重启后需要重新登录。
- `/url` 不返回完整 Token，因为系统不保存 Webhook Token 明文。
- 自更新功能当前只预留版本信息和发布结构，未实现自动升级。
