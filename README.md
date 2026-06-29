# Hookgram

Hookgram 是一个自托管的 Telegram Bot Webhook 消息转发系统。用户通过 Bot 创建自己的 Webhook Token，外部系统调用对应 URL 后，消息会转发到该 Telegram 用户；管理员可在 Web 管理端完成初始化、配置、用户管理、Token 管理和推送记录查看。

当前开发目标：`v0.1.0-rc.3`

最新公开版本：`v0.1.0-rc.2`

默认地址：`http://127.0.0.1:8787`  
首次初始化：`http://127.0.0.1:8787/setup`

## 核心功能

- Telegram Bot 命令创建和管理 Webhook Token。
- `GET /w/{token}` 与 `POST /w/{token}` 消息推送。
- 支持 JSON、纯文本、表单、Markdown、HTML 和结构化 `fields`。
- Web 管理端：初始化、登录、系统设置、Bot 用户、Token、推送记录。
- 默认 SQLite，保留 MySQL、MariaDB、PostgreSQL 配置入口。
- 前端通过 Go embed 内嵌进单个可执行文件。
- `/api/version` 暴露版本、commit、构建时间和运行平台。

## 快速开始

```powershell
.\scripts\build-windows.ps1
.\dist\hookgram.exe
```

打开：

```text
http://127.0.0.1:8787/setup
```

## Windows 运行

从 GitHub Releases 下载对应 Windows 压缩包，解压后运行：

```powershell
.\hookgram.exe
```

本地构建输出：

```text
dist/hookgram.exe
```

## Linux 一键安装

```bash
curl -fsSL https://raw.githubusercontent.com/akiiya/hookgram/main/scripts/install-linux.sh | sudo bash
```

指定版本：

```bash
curl -fsSL https://raw.githubusercontent.com/akiiya/hookgram/main/scripts/install-linux.sh | sudo env HOOKGRAM_VERSION=v0.1.0-rc.3 bash
```

脚本会安装到 `/opt/hookgram`，数据目录为 `/var/lib/hookgram`，并创建 `hookgram.service`。

卸载程序，保留配置和数据：

```bash
curl -fsSL https://raw.githubusercontent.com/akiiya/hookgram/main/scripts/install-linux.sh | sudo bash -s -- --uninstall
```

彻底卸载，删除配置和数据：

```bash
curl -fsSL https://raw.githubusercontent.com/akiiya/hookgram/main/scripts/install-linux.sh | sudo bash -s -- --purge
```

跳过确认彻底卸载：

```bash
curl -fsSL https://raw.githubusercontent.com/akiiya/hookgram/main/scripts/install-linux.sh | sudo bash -s -- --purge --yes
```

`--uninstall` 会保留 `/var/lib/hookgram`；`--purge` 会删除 `/var/lib/hookgram`，请谨慎使用。

## Linux 手动安装

在 GitHub Releases 下载对应系统和架构的二进制压缩包：

```text
https://github.com/akiiya/hookgram/releases
```

示例：

```bash
tar -xzf hookgram-v0.1.0-rc.3-linux-amd64.tar.gz
sudo install -m 0755 hookgram /opt/hookgram/hookgram
HOOKGRAM_DATA_DIR=/var/lib/hookgram HOOKGRAM_CONFIG=/var/lib/hookgram/config.yaml /opt/hookgram/hookgram
```

## 首次初始化

首次启动后访问：

```text
http://127.0.0.1:8787/setup
```

初始化会创建管理员账号，并写入 Telegram Bot Token、Telegram API Proxy、Base URL。Bot Token 可先留空，之后在“系统设置”中补充。

## Webhook 用法

GET 适合轻量消息：

```bash
curl "http://127.0.0.1:8787/w/wh_xxx?title=测试&text=Hello&level=success"
```

POST 适合正式和结构化消息：

```bash
curl -X POST "http://127.0.0.1:8787/w/wh_xxx" \
  -H "Content-Type: application/json" \
  -d '{"title":"部署完成","text":"Hello from Hookgram","format":"markdown","level":"success","source":"deploy-system","fields":{"环境":"prod","服务":"api-server"}}'
```

PowerShell：

```powershell
curl.exe -X POST "http://127.0.0.1:8787/w/wh_xxx" `
  -H "Content-Type: application/json" `
  -d "{\"title\":\"部署完成\",\"text\":\"Hello from Hookgram\",\"format\":\"markdown\",\"level\":\"success\",\"source\":\"deploy-system\",\"fields\":{\"环境\":\"prod\",\"服务\":\"api-server\"}}"
```

成功响应：

```json
{"ok":true,"message":"sent"}
```

## Bot 命令

```text
/start
/help
/list
/add [别名]
/del <别名或token前缀>
/rename <旧别名> <新别名>
/url <别名>
/usage <别名>
```

`/add` 创建成功后会返回完整 Token 和 Webhook URL。完整 Token 只显示一次，数据库只保存哈希和前缀。

## 管理端

- 总览：Bot 用户、Token、今日推送、成功/失败统计。
- Bot 用户：查看用户、Token 和推送记录。
- 系统设置：Base URL、Telegram Bot Token、API Proxy、运行信息。
- 关于信息：当前版本来自 `/api/version`。

## 配置

默认配置文件：

```text
data/config.yaml
```

Linux systemd 推荐环境变量：

```bash
HOOKGRAM_DATA_DIR=/var/lib/hookgram
HOOKGRAM_CONFIG=/var/lib/hookgram/config.yaml
```

路径优先级：

1. `HOOKGRAM_CONFIG`
2. `HOOKGRAM_DATA_DIR/config.yaml`
3. `data/config.yaml`

## 安全说明

- Webhook Token 完整值只在创建成功时显示一次。
- 数据库只保存 `token_hash` 和 `token_prefix`。
- Telegram Bot Token 在 API 返回中会脱敏。
- Webhook 日志会隐藏 URL 中的完整 Token。
- 管理端 Session 为内存态，服务重启后需要重新登录。

## GitHub Actions 与 Release

- `VERSION` 是唯一版本号来源。
- `ci.yml`：`dev` push、指向 `main` 的 PR 只跑 CI，不发布。
- `release.yml`：`main` 被合并后自动读取 `VERSION`，完成测试、验证、打 tag、构建多平台二进制、生成 checksums、创建 GitHub Release。
- 不需要人工打 tag，也不需要人工创建 Release。
- 发布下一个版本时，在 `dev` 修改 `VERSION`，例如 `v0.1.0-rc.3` -> `v0.1.0-rc.4`，CI 通过后合并到 `main`。
- 如果 `VERSION` 对应 tag 指向其它 commit，发布 workflow 会失败并提示升级版本号；如果 tag 已在当前 `main` commit 上，则会复用该 tag 并补齐或更新 Release。

资产命名：

```text
hookgram-v0.1.0-rc.3-windows-amd64.zip
hookgram-v0.1.0-rc.3-windows-arm64.zip
hookgram-v0.1.0-rc.3-linux-amd64.tar.gz
hookgram-v0.1.0-rc.3-linux-arm64.tar.gz
hookgram-v0.1.0-rc.3-linux-386.tar.gz
hookgram-v0.1.0-rc.3-linux-armv7.tar.gz
hookgram-v0.1.0-rc.3-checksums.txt
```

## 分支策略

- `main`：保护分支，只接受 PR 或 GitHub 页面 merge；合并后自动发布。
- `dev`：日常开发，push 后只跑 CI。
- `feature/*`：功能开发。
- `release/*`：发布收口。
- `hotfix/*`：紧急修复。

详见 [docs/BRANCHING.md](docs/BRANCHING.md)。

## 检查更新预留

当前版本可通过：

```text
GET /api/version
```

后续 Web 管理端“检查更新”和程序自更新可基于 GitHub Release API、规范化资产命名和 checksums 实现。本版本只预留版本元信息和发布结构，不实现自动升级。

## 已知限制

- `v0.1.0-rc.3` 是下一轮 Release Candidate 目标，适合验收和小范围自托管试用。
- Linux 一键安装依赖对应 GitHub Release 资产已经存在。
- systemd 安装仅支持 Linux，不支持 Windows。
- `windows/386` 暂不发布资产，原因是当前纯 Go SQLite 依赖在该目标下交叉编译失败。
- MySQL、MariaDB、PostgreSQL 配置入口已保留，但仍需实库验收。
- 自更新当前未实现。
