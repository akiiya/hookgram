#!/usr/bin/env bash
set -euo pipefail

REPO="akiiya/hookgram"
SERVICE_NAME="hookgram"
RUN_USER="hookgram"
RUN_GROUP="hookgram"
INSTALL_DIR="/opt/hookgram"
DATA_DIR="/var/lib/hookgram"
CONFIG_FILE="${DATA_DIR}/config.yaml"

log() {
  printf '%s\n' "$*"
}

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

need_root() {
  if [ "$(id -u)" -ne 0 ]; then
    fail "请使用 root 权限执行，例如：curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install-linux.sh | sudo bash"
  fi
}

has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

download() {
  local url="$1"
  local output="$2"
  if has_cmd curl; then
    curl -fL "$url" -o "$output"
  elif has_cmd wget; then
    wget -O "$output" "$url"
  else
    fail "需要 curl 或 wget"
  fi
}

download_stdout() {
  local url="$1"
  if has_cmd curl; then
    curl -fsSL "$url"
  elif has_cmd wget; then
    wget -qO- "$url"
  else
    fail "需要 curl 或 wget"
  fi
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) printf 'amd64' ;;
    aarch64 | arm64) printf 'arm64' ;;
    armv7l | armv7) printf 'armv7' ;;
    i386 | i686) printf '386' ;;
    *) fail "暂不支持当前架构：$(uname -m)" ;;
  esac
}

latest_version() {
  local json version
  json="$(download_stdout "https://api.github.com/repos/${REPO}/releases")"
  version="$(printf '%s' "$json" | grep -m1 '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')"
  [ -n "$version" ] || fail "未找到可用 GitHub Release"
  printf '%s' "$version"
}

validate_version() {
  if ! printf '%s' "$1" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$'; then
    fail "版本号不符合规范：$1"
  fi
}

random_hex() {
  od -An -tx1 -N32 /dev/urandom | tr -d ' \n'
}

write_default_config() {
  if [ -f "$CONFIG_FILE" ]; then
    log "检测到已有配置文件，保留不覆盖：${CONFIG_FILE}"
    return
  fi

  local session_secret token_hash_secret
  session_secret="$(random_hex)"
  token_hash_secret="$(random_hex)"
  cat > "$CONFIG_FILE" <<EOF
# Hookgram 配置文件
app:
  host: "0.0.0.0"
  port: 8787
  base_url: ""

telegram:
  bot_token: ""
  api_proxy: ""
  command_mode: "polling"

database:
  driver: "sqlite"
  dsn: "${DATA_DIR}/hookgram.db"

security:
  session_secret: "${session_secret}"
  token_hash_secret: "${token_hash_secret}"

log:
  level: "info"
EOF
  chmod 600 "$CONFIG_FILE"
  chown "${RUN_USER}:${RUN_GROUP}" "$CONFIG_FILE"
}

ensure_user() {
  if ! getent group "$RUN_GROUP" >/dev/null 2>&1; then
    if has_cmd groupadd; then
      groupadd --system "$RUN_GROUP"
    else
      fail "需要 groupadd 创建运行用户组"
    fi
  fi
  if id "$RUN_USER" >/dev/null 2>&1; then
    usermod -g "$RUN_GROUP" "$RUN_USER" >/dev/null 2>&1 || true
    return
  fi
  if has_cmd useradd; then
    useradd --system --gid "$RUN_GROUP" --home-dir "$DATA_DIR" --shell /usr/sbin/nologin "$RUN_USER"
  elif has_cmd adduser; then
    adduser --system --home "$DATA_DIR" --shell /usr/sbin/nologin --ingroup "$RUN_GROUP" "$RUN_USER"
  else
    fail "需要 useradd 或 adduser 创建运行用户"
  fi
}

write_systemd_unit() {
  cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Hookgram Telegram Webhook Relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${RUN_USER}
Group=${RUN_GROUP}
Environment=HOOKGRAM_DATA_DIR=${DATA_DIR}
Environment=HOOKGRAM_CONFIG=${CONFIG_FILE}
WorkingDirectory=${DATA_DIR}
ExecStart=${INSTALL_DIR}/hookgram
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
}

server_ip() {
  local ip=""
  if has_cmd hostname; then
    ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
  fi
  if [ -z "$ip" ] && has_cmd ip; then
    ip="$(ip route get 1.1.1.1 2>/dev/null | awk '/src/ {for (i=1;i<=NF;i++) if ($i=="src") {print $(i+1); exit}}' || true)"
  fi
  printf '%s' "$ip"
}

main() {
  need_root
  has_cmd tar || fail "需要 tar"
  has_cmd systemctl || fail "需要 systemd / systemctl"
  [ -d /run/systemd/system ] || fail "当前系统未运行 systemd"

  local arch version asset url tmp
  arch="$(detect_arch)"
  version="${HOOKGRAM_VERSION:-}"
  if [ -z "$version" ]; then
    version="$(latest_version)"
  fi
  validate_version "$version"

  asset="hookgram-${version}-linux-${arch}.tar.gz"
  url="https://github.com/${REPO}/releases/download/${version}/${asset}"

  if systemctl list-unit-files "${SERVICE_NAME}.service" >/dev/null 2>&1; then
    log "检测到已存在 ${SERVICE_NAME}.service，将执行升级/重装并保留现有配置。"
    systemctl stop "${SERVICE_NAME}.service" >/dev/null 2>&1 || true
  fi

  ensure_user
  mkdir -p "$INSTALL_DIR" "$DATA_DIR"
  chown "${RUN_USER}:${RUN_GROUP}" "$DATA_DIR"

  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  log "下载 ${url}"
  download "$url" "${tmp}/${asset}"
  mkdir -p "${tmp}/pkg"
  tar -xzf "${tmp}/${asset}" -C "${tmp}/pkg"
  [ -f "${tmp}/pkg/hookgram" ] || fail "Release 包中未找到 hookgram 可执行文件"
  install -m 0755 "${tmp}/pkg/hookgram" "${INSTALL_DIR}/hookgram"

  write_default_config
  write_systemd_unit

  systemctl daemon-reload
  systemctl enable "${SERVICE_NAME}.service" >/dev/null
  systemctl start "${SERVICE_NAME}.service"
  systemctl is-active --quiet "${SERVICE_NAME}.service" || fail "服务启动失败，请执行 journalctl -u hookgram -f 查看日志"

  local ip
  ip="$(server_ip)"
  log ""
  log "Hookgram 已安装并启动。"
  log ""
  if [ -n "$ip" ]; then
    log "访问地址："
    log "http://${ip}:8787"
    log ""
    log "首次初始化："
    log "http://${ip}:8787/setup"
  else
    log "请在浏览器访问：http://<你的服务器IP>:8787/setup"
  fi
  log ""
  log "查看服务状态："
  log "systemctl status hookgram"
  log ""
  log "查看日志："
  log "journalctl -u hookgram -f"
  log ""
  log "配置文件："
  log "${CONFIG_FILE}"
}

main "$@"
