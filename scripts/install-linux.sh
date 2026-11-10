#!/usr/bin/env bash
# install-linux.sh：校验、部署、systemd 注册、启动/更新与状态检查。
# 从脚本自身位置定位根目录，不依赖调用者工作目录。
#
#   make build
#   sudo env APP_ADMIN_PASSWORD='your-password' ./scripts/install-linux.sh
#
# 布局：程序 /opt/nwatch，配置 /opt/nwatch/configs，数据 /var/lib/nwatch，
# 专用非 root 用户 nwatch，生产默认 profile=pro。
# 已有的 configs/ 与环境文件不会被覆盖；更新只替换二进制并重启。

if [ -z "${BASH_VERSION:-}" ]; then
  echo "请用 bash 运行这个脚本" >&2
  exit 1
fi

set -euo pipefail

PREFIX=/opt/nwatch
DATA_DIR=/var/lib/nwatch
SERVICE_NAME=nwatch
RUN_USER=nwatch
UNIT_DIR=/etc/systemd/system
BINARY=
CONFIGS_SRC=
ENV_SRC=
START=1
ENABLE=1
DRY_RUN=0
SYSTEMCTL=${SYSTEMCTL_OVERRIDE:-systemctl}
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)

usage() {
  cat <<EOF
用法：${0} [选项]

把 nwatch 安装为 systemd 服务。需要 root，且系统以 systemd 运行。
已有的配置与环境文件不会被覆盖。

选项：
  --prefix DIR     安装目录，默认 /opt/nwatch
  --data-dir DIR   数据目录，默认 /var/lib/nwatch
  --binary PATH    nwatch 二进制；默认查找 ./nwatch、仓库根目录与 release/
  --configs DIR    配置目录源；默认仓库 configs/
  --env PATH       环境变量模板；仅在目标不存在时复制
  --name NAME      systemd 服务名，默认 nwatch
  --user NAME      运行用户，默认 nwatch
  --unit-dir DIR   单元目录，默认 /etc/systemd/system
  --no-enable      不设置开机启动
  --no-start       不启动；服务已在运行时仍会重启以加载新二进制
  --dry-run        只打印将要执行的操作
  -h, --help       显示说明

管理员密码取自环境变量 APP_ADMIN_PASSWORD（兼容旧名 NMONITOR_PASSWORD）。
EOF
}

die() { printf '错误：%s\n' "$*" >&2; exit 1; }
log() { printf '%s\n' "$*"; }

run() {
  printf '+'
  printf ' %q' "$@"
  printf '\n'
  if [ "$DRY_RUN" -eq 0 ]; then
    "$@"
  fi
}

ensure_dir() {
  local dir=$1 mode=$2 owner=$3
  if [ -d "$dir" ]; then
    return 0
  fi
  run install -d -m "$mode" -o "$owner" -g "$owner" "$dir"
}

parse_args() {
  while [ $# -gt 0 ]; do
    case $1 in
      -h|--help) usage; exit 0 ;;
      --no-start) START=0; shift ;;
      --no-enable) ENABLE=0; shift ;;
      --dry-run) DRY_RUN=1; shift ;;
      --prefix|--data-dir|--binary|--configs|--env|--name|--user|--unit-dir)
        [ $# -ge 2 ] && [ -n "${2:-}" ] && [ "${2:-}" != "--" ] || die "$1 需要一个值"
        case $1 in
          --prefix) PREFIX=$2 ;;
          --data-dir) DATA_DIR=$2 ;;
          --binary) BINARY=$2 ;;
          --configs) CONFIGS_SRC=$2 ;;
          --env) ENV_SRC=$2 ;;
          --name) SERVICE_NAME=$2 ;;
          --user) RUN_USER=$2 ;;
          --unit-dir) UNIT_DIR=$2 ;;
        esac
        shift 2 ;;
      *)
        die "未知参数：$1" ;;
    esac
  done
}

validate_options() {
  PREFIX=${PREFIX%/}; DATA_DIR=${DATA_DIR%/}; UNIT_DIR=${UNIT_DIR%/}
  [ "$(uname -s)" = "Linux" ] || die "只支持 Linux"
  case "$PREFIX" in /*) ;; *) die "安装目录必须是绝对路径" ;; esac
  [ "$PREFIX" != "/" ] || die "安装目录不能是根目录"
  case "$DATA_DIR" in /*) ;; *) die "数据目录必须是绝对路径" ;; esac
  case "$SERVICE_NAME" in
    [!A-Za-z0-9]*) die "服务名要以字母或数字开头" ;;
    *[!A-Za-z0-9._-]*) die "服务名只能包含字母、数字、点、下划线和连字符" ;;
  esac
  if [ "${#SERVICE_NAME}" -gt 200 ]; then
    die "服务名过长"
  fi
}

require_runtime() {
  [ "$(id -u)" -eq 0 ] || die "请使用 root 运行：sudo env APP_ADMIN_PASSWORD='你的密码' $0"
  command -v "$SYSTEMCTL" >/dev/null 2>&1 || die "未找到 systemctl，需要 systemd"
  command -v install >/dev/null 2>&1 || die "未找到 install 命令"
  if [ "$DRY_RUN" -eq 0 ]; then
    [ -f /run/systemd/system ] || die "系统未以 systemd 运行"
  fi
}

resolve_inputs() {
  if [ -n "$BINARY" ]; then
    [ -f "$BINARY" ] || die "找不到二进制：$BINARY"
  else
    local candidate
    for candidate in "$PWD/nwatch" "$REPO_ROOT/nwatch" "$REPO_ROOT"/release/*/nwatch; do
      if [ -f "$candidate" ]; then
        BINARY=$candidate
        break
      fi
    done
    [ -n "$BINARY" ] || die "找不到 nwatch 二进制。请先构建，或使用 --binary 指定路径"
  fi
  if [ -n "$CONFIGS_SRC" ]; then
    [ -d "$CONFIGS_SRC" ] || die "找不到配置目录：$CONFIGS_SRC"
  else
    CONFIGS_SRC=$REPO_ROOT/configs
    [ -d "$CONFIGS_SRC" ] || die "找不到仓库配置目录：$CONFIGS_SRC"
  fi
  if [ -n "$ENV_SRC" ]; then
    [ -f "$ENV_SRC" ] || die "找不到环境变量模板：$ENV_SRC"
  else
    ENV_SRC=$REPO_ROOT/deploy/nwatch.env.example
    [ -f "$ENV_SRC" ] || die "找不到环境变量模板：$ENV_SRC"
  fi
}

service_active() {
  "$SYSTEMCTL" is-active --quiet "$SERVICE_NAME" 2>/dev/null
}

install_user() {
  if id "$RUN_USER" >/dev/null 2>&1; then
    return 0
  fi
  run useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin "$RUN_USER"
}

write_env_file() {
  local target=$PREFIX/${SERVICE_NAME}.env
  if [ -f "$target" ]; then
    log "保留已有环境文件：$target"
    return 0
  fi
  if [ -n "${APP_ADMIN_PASSWORD:-}" ]; then
    run install -m 600 /dev/null "$target"
    if [ "$DRY_RUN" -eq 0 ]; then
      cat >"$target" <<EOF
# nwatch 环境变量。此处仅保留本地运维所需项。
APP_PROFILE=pro
APP_ADMIN_PASSWORD=${APP_ADMIN_PASSWORD}
EOF
    fi
  else
    run install -m 600 "$ENV_SRC" "$target"
  fi
}

write_unit() {
  local unit=$UNIT_DIR/${SERVICE_NAME}.service
  local was_active=0
  if service_active; then
    was_active=1
  fi
  if [ "$was_active" -eq 1 ] && [ "$START" -eq 1 ]; then
    # 更新先停写，避免覆盖运行中的二进制。
    run "$SYSTEMCTL" stop "$SERVICE_NAME"
  fi
  run install -m 755 "$BINARY" "$PREFIX/nwatch"
  ensure_dir "$DATA_DIR" 750 "$RUN_USER"
  ensure_dir "$PREFIX/configs" 755 root
  local f
  for f in "$CONFIGS_SRC"/*; do
    [ -e "$f" ] || continue
    local name
    name=$(basename "$f")
    if [ -f "$PREFIX/configs/$name" ]; then
      log "保留已有配置：$PREFIX/configs/$name"
      continue
    fi
    run install -m 644 "$f" "$PREFIX/configs/$name"
  done
  write_env_file
  if [ "$DRY_RUN" -eq 0 ]; then
    cat >"$unit" <<EOF
[Unit]
Description=NWatch 监控服务
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$RUN_USER
Group=$RUN_USER
WorkingDirectory=$PREFIX
EnvironmentFile=$PREFIX/${SERVICE_NAME}.env
ExecStart=$PREFIX/nwatch --config-dir $PREFIX/configs --data-dir $DATA_DIR
Restart=on-failure
RestartSec=5
TimeoutStopSec=30
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=$DATA_DIR
ProtectHome=true

[Install]
WantedBy=multi-user.target
EOF
    chmod 644 "$unit"
  else
    log "dry-run：将写入 $unit"
  fi
  chown_files
  run "$SYSTEMCTL" daemon-reload
  if [ "$was_active" -eq 1 ] && [ "$START" -eq 1 ]; then
    run "$SYSTEMCTL" restart "$SERVICE_NAME"
  elif [ "$ENABLE" -eq 1 ]; then
    if [ "$START" -eq 1 ]; then
      run "$SYSTEMCTL" enable --now "$SERVICE_NAME"
    else
      run "$SYSTEMCTL" enable "$SERVICE_NAME"
    fi
  elif [ "$START" -eq 1 ]; then
    if service_active; then
      run "$SYSTEMCTL" restart "$SERVICE_NAME"
    else
      run "$SYSTEMCTL" start "$SERVICE_NAME"
    fi
  fi
  sleep 1
  if [ "$START" -eq 1 ] && [ "$DRY_RUN" -eq 0 ]; then
    "$SYSTEMCTL" is-active --quiet "$SERVICE_NAME" || die "服务未能进入运行态，查看：systemctl status $SERVICE_NAME 与 journalctl -u $SERVICE_NAME"
    log "服务已运行：$SERVICE_NAME"
  fi
  log "状态查看：systemctl status $SERVICE_NAME"
  log "日志查看：journalctl -u $SERVICE_NAME"
}

chown_files() {
  [ "$DRY_RUN" -eq 0 ] || return 0
  chown "$RUN_USER":"$RUN_USER" "$PREFIX/nwatch" "$DATA_DIR"
}

main() {
  parse_args "$@"
  validate_options
  require_runtime
  resolve_inputs
  install_user
  ensure_dir "$PREFIX" 755 root
  write_unit
  log "安装完成。"
}

main "$@"
