#!/usr/bin/env bash
set -euo pipefail

REPO="akiiya/hookgram"
SERVICE_NAME="hookgram"
RUN_USER="hookgram"
RUN_GROUP="hookgram"
INSTALL_DIR="/opt/hookgram"
DATA_DIR="/var/lib/hookgram"
CONFIG_FILE="${DATA_DIR}/config.yaml"
UNIT_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

ACTION="install"
DRY_RUN="${HOOKGRAM_DRY_RUN:-0}"
YES="${HOOKGRAM_YES:-0}"
TMP_DIR=""

log() {
  printf '%s\n' "$*"
}

fail() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Hookgram Linux installer

Usage:
bash install-linux.sh [options]

Options:
--uninstall    卸载程序，保留 /var/lib/hookgram 数据
--purge        彻底卸载程序、配置和数据
--yes          与 --purge 配合使用，跳过确认
--dry-run      仅打印将执行的操作，不修改系统
-h, --help     显示帮助

Environment:
HOOKGRAM_VERSION=v0.1.0-rc.1
HOOKGRAM_DRY_RUN=1
HOOKGRAM_YES=1

Examples:
bash install-linux.sh
HOOKGRAM_VERSION=v0.1.0-rc.1 bash install-linux.sh
bash install-linux.sh --uninstall
bash install-linux.sh --purge
bash install-linux.sh --purge --yes
EOF
}

is_dry_run() {
  case "${DRY_RUN:-0}" in
    1 | true | TRUE | yes | YES) return 0 ;;
    *) return 1 ;;
  esac
}

is_yes() {
  case "${YES:-0}" in
    1 | true | TRUE | yes | YES) return 0 ;;
    *) return 1 ;;
  esac
}

parse_args() {
  local arg
  for arg in "$@"; do
    case "$arg" in
      --uninstall)
        [ "$ACTION" = "install" ] || fail "--uninstall 不能与 --purge 同时使用"
        ACTION="uninstall"
        ;;
      --purge)
        [ "$ACTION" = "install" ] || fail "--purge 不能与 --uninstall 同时使用"
        ACTION="purge"
        ;;
      --yes)
        YES="1"
        ;;
      --dry-run)
        DRY_RUN="1"
        ;;
      -h | --help)
        usage
        exit 0
        ;;
      *)
        printf '未知参数：%s\n\n' "$arg" >&2
        usage >&2
        exit 2
        ;;
    esac
  done
}

cleanup() {
  if [ -n "${TMP_DIR:-}" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
}

trap cleanup EXIT

need_root() {
  if is_dry_run; then
    log "[dry-run] 检查 root 权限"
    return
  fi
  if [ "$(id -u)" -ne 0 ]; then
    fail "请使用 root 权限执行，例如：curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install-linux.sh | sudo bash"
  fi
}

has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

require_cmd() {
  local name="$1"
  if is_dry_run; then
    log "[dry-run] 检查命令：${name}"
    return
  fi
  has_cmd "$name" || fail "需要 ${name}"
}

require_systemd() {
  if is_dry_run; then
    log "[dry-run] 检查 systemd / systemctl"
    return
  fi
  has_cmd systemctl || fail "需要 systemd / systemctl"
  [ -d /run/systemd/system ] || fail "当前系统未运行 systemd"
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
  cat > "$UNIT_FILE" <<EOF
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

safe_remove_dir() {
  local dir="$1"
  case "$dir" in
    "$INSTALL_DIR" | "$DATA_DIR") ;;
    *) fail "拒绝删除非固定目录：${dir}" ;;
  esac

  if is_dry_run; then
    log "[dry-run] rm -rf ${dir}"
    return
  fi
  rm -rf "$dir"
}

stop_disable_service() {
  if is_dry_run; then
    log "[dry-run] systemctl stop ${SERVICE_NAME}.service || true"
    log "[dry-run] systemctl disable ${SERVICE_NAME}.service || true"
    return
  fi
  if has_cmd systemctl; then
    systemctl stop "${SERVICE_NAME}.service" >/dev/null 2>&1 || true
    systemctl disable "${SERVICE_NAME}.service" >/dev/null 2>&1 || true
  fi
}

remove_systemd_unit() {
  if is_dry_run; then
    log "[dry-run] rm -f ${UNIT_FILE}"
    return
  fi
  rm -f "$UNIT_FILE"
}

reload_systemd() {
  if is_dry_run; then
    log "[dry-run] systemctl daemon-reload"
    log "[dry-run] systemctl reset-failed ${SERVICE_NAME} || true"
    return
  fi
  if has_cmd systemctl; then
    systemctl daemon-reload >/dev/null 2>&1 || true
    systemctl reset-failed "$SERVICE_NAME" >/dev/null 2>&1 || true
  fi
}

delete_run_user() {
  if is_dry_run; then
    log "[dry-run] userdel ${RUN_USER} || true"
    return
  fi
  if id "$RUN_USER" >/dev/null 2>&1; then
    userdel "$RUN_USER" >/dev/null 2>&1 || true
  fi
}

confirm_purge() {
  if is_dry_run || is_yes; then
    return
  fi

  local answer=""
  cat <<EOF
即将彻底删除 Hookgram，包括：

* 程序目录：${INSTALL_DIR}
* 数据目录：${DATA_DIR}
* 配置文件：${CONFIG_FILE}
* SQLite 数据库和所有运行数据

请输入 PURGE 确认彻底删除：
EOF

  if [ -r /dev/tty ]; then
    read -r answer </dev/tty || true
  else
    read -r answer || true
  fi

  if [ "$answer" != "PURGE" ]; then
    log "已取消彻底卸载"
    exit 1
  fi
}

print_install_plan() {
  local version="$1"
  local asset="$2"
  local url="$3"
  log "[dry-run] 将安装 Hookgram ${version}"
  log "[dry-run] 将下载：${url}"
  log "[dry-run] 将使用资产：${asset}"
  log "[dry-run] 将创建/确认用户：${RUN_USER}"
  log "[dry-run] 将创建程序目录：${INSTALL_DIR}"
  log "[dry-run] 将创建数据目录：${DATA_DIR}"
  log "[dry-run] 若配置不存在，将写入：${CONFIG_FILE}"
  log "[dry-run] 将写入 systemd 服务：${UNIT_FILE}"
  log "[dry-run] 将启用并启动：${SERVICE_NAME}.service"
}

install_hookgram() {
  need_root

  local arch version asset url
  arch="$(detect_arch)"
  version="${HOOKGRAM_VERSION:-}"
  if [ -z "$version" ]; then
    if is_dry_run; then
      version="<latest-release>"
    else
      version="$(latest_version)"
    fi
  fi
  if [ "$version" != "<latest-release>" ]; then
    validate_version "$version"
  fi

  asset="hookgram-${version}-linux-${arch}.tar.gz"
  url="https://github.com/${REPO}/releases/download/${version}/${asset}"

  if is_dry_run; then
    print_install_plan "$version" "$asset" "$url"
    return
  fi

  require_cmd tar
  require_systemd

  if systemctl list-unit-files "${SERVICE_NAME}.service" >/dev/null 2>&1; then
    log "检测到已存在 ${SERVICE_NAME}.service，将执行升级/重装并保留现有配置。"
    systemctl stop "${SERVICE_NAME}.service" >/dev/null 2>&1 || true
  fi

  ensure_user
  mkdir -p "$INSTALL_DIR" "$DATA_DIR"
  chown "${RUN_USER}:${RUN_GROUP}" "$DATA_DIR"

  TMP_DIR="$(mktemp -d)" || fail "创建临时目录失败"
  [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ] || fail "创建临时目录失败"

  log "下载 ${url}"
  download "$url" "${TMP_DIR}/${asset}"
  mkdir -p "${TMP_DIR}/pkg"
  tar -xzf "${TMP_DIR}/${asset}" -C "${TMP_DIR}/pkg"
  [ -f "${TMP_DIR}/pkg/hookgram" ] || fail "Release 包中未找到 hookgram 可执行文件"
  install -m 0755 "${TMP_DIR}/pkg/hookgram" "${INSTALL_DIR}/hookgram"

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

uninstall_hookgram() {
  need_root
  stop_disable_service
  remove_systemd_unit
  reload_systemd
  safe_remove_dir "$INSTALL_DIR"

  log ""
  log "Hookgram 程序已卸载。"
  log ""
  log "已删除："
  log ""
  log "* systemd 服务：${SERVICE_NAME}.service"
  log "* 程序目录：${INSTALL_DIR}"
  log ""
  log "已保留："
  log ""
  log "* 数据目录：${DATA_DIR}"
  log "* 配置文件：${CONFIG_FILE}"
  log ""
  log "如需彻底删除配置和数据，请执行："
  log "bash install-linux.sh --purge"
}

purge_hookgram() {
  need_root
  confirm_purge
  stop_disable_service
  remove_systemd_unit
  reload_systemd
  safe_remove_dir "$INSTALL_DIR"
  safe_remove_dir "$DATA_DIR"
  delete_run_user

  log ""
  log "Hookgram 已彻底卸载。"
  log ""
  log "已删除："
  log ""
  log "* systemd 服务：${SERVICE_NAME}.service"
  log "* 程序目录：${INSTALL_DIR}"
  log "* 数据目录：${DATA_DIR}"
  log "* 系统用户：${RUN_USER}"
}

main() {
  parse_args "$@"
  case "$ACTION" in
    install) install_hookgram ;;
    uninstall) uninstall_hookgram ;;
    purge) purge_hookgram ;;
    *) fail "未知动作：${ACTION}" ;;
  esac
}

main "$@"
