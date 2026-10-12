#!/usr/bin/env bash
# 把 NMonitor 安装为 systemd 服务。
#
#   make build
#   sudo env NMONITOR_PASSWORD='your-password' ./deploy/install-service.sh
#
# 已有的 config.yaml 和 nmonitor.env 不会被覆盖。
# 再次运行会更新二进制和单元；服务若已在运行，会重启以加载新程序。

if [ -z "${BASH_VERSION:-}" ]; then
  echo "请用 bash 运行这个脚本" >&2
  exit 1
fi

set -euo pipefail

PREFIX=/opt/nmonitor
SERVICE_NAME=nmonitor
UNIT_DIR=/etc/systemd/system
BINARY=
CONFIG=
ENV_SRC=
START=1
ENABLE=1
DRY_RUN=0
# 测试可以把 NMONITOR_SYSTEMCTL 指到 systemctl 替身。
SYSTEMCTL=${NMONITOR_SYSTEMCTL:-systemctl}
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
TMP_FILES=()

usage() {
  cat <<EOF
用法：${0} [选项]

把 nmonitor 安装为 systemd 服务。需要 root，且系统以 systemd 运行。
已有的 config.yaml 和 nmonitor.env 不会被覆盖。

选项：
  --prefix DIR     安装目录，默认 /opt/nmonitor
  --binary PATH    nmonitor 二进制；默认查找 ./nmonitor 和仓库根目录
  --config PATH    配置文件；仅在目标不存在时复制
  --env PATH       环境变量模板；仅在目标不存在时复制为安装目录下的 nmonitor.env
  --name NAME      systemd 服务名，默认 nmonitor
  --unit-dir DIR   单元目录，默认 /etc/systemd/system
  --no-enable      不设置开机启动
  --no-start       不启动。服务已经在运行时仍会重启，以加载新二进制
  --dry-run        只打印将要执行的操作
  -h, --help       显示说明

管理员密码取自环境变量 NMONITOR_PASSWORD。在终端里直接运行、且目标环境文件还不存在时，会提示输入密码。
非交互运行且未设置该变量时，先写入示例值 change-me。

示例：
  make build
  sudo env NMONITOR_PASSWORD='your-password' ./deploy/install-service.sh
EOF
}

die() {
  printf '错误：%s\n' "$*" >&2
  exit 1
}

log() {
  printf '%s\n' "$*"
}

cleanup() {
  if [[ ${#TMP_FILES[@]} -gt 0 ]]; then
    rm -f "${TMP_FILES[@]}"
  fi
}
trap cleanup EXIT

run() {
  printf '+'
  printf ' %q' "$@"
  printf '\n'
  if [[ $DRY_RUN -eq 0 ]]; then
    "$@"
  fi
}

ensure_dir() {
  local dir=$1 mode=$2
  if [[ -d $dir ]]; then
    return 0
  fi
  run install -d -m "$mode" -o root -g root "$dir"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case $1 in
      -h|--help)
        usage
        exit 0
        ;;
      --no-start)
        START=0
        shift
        ;;
      --no-enable)
        ENABLE=0
        shift
        ;;
      --dry-run)
        DRY_RUN=1
        shift
        ;;
      --prefix|--binary|--config|--env|--name|--unit-dir)
        if [[ $# -lt 2 || -z ${2:-} || ${2:-} == --* ]]; then
          die "$1 需要一个值"
        fi
        case $1 in
          --prefix) PREFIX=$2 ;;
          --binary) BINARY=$2 ;;
          --config) CONFIG=$2 ;;
          --env) ENV_SRC=$2 ;;
          --name) SERVICE_NAME=$2 ;;
          --unit-dir) UNIT_DIR=$2 ;;
        esac
        shift 2
        ;;
      --prefix=*|--binary=*|--config=*|--env=*|--name=*|--unit-dir=*)
        local key=${1%%=*} value=${1#*=}
        if [[ -z $value ]]; then
          die "$key 需要一个值"
        fi
        case $key in
          --prefix) PREFIX=$value ;;
          --binary) BINARY=$value ;;
          --config) CONFIG=$value ;;
          --env) ENV_SRC=$value ;;
          --name) SERVICE_NAME=$value ;;
          --unit-dir) UNIT_DIR=$value ;;
        esac
        shift
        ;;
      *)
        die "未知参数：$1"
        ;;
    esac
  done
}

validate_options() {
  PREFIX=${PREFIX%/}
  UNIT_DIR=${UNIT_DIR%/}

  [[ $(uname -s) == Linux ]] || die "只支持 Linux"
  [[ -n $PREFIX && $PREFIX == /* ]] || die "安装目录必须是绝对路径"
  [[ $PREFIX != / ]] || die "安装目录不能是根目录"
  [[ $PREFIX != *[[:space:]]* ]] || die "安装目录不能包含空白"
  [[ -n $UNIT_DIR && $UNIT_DIR == /* ]] || die "单元目录必须是绝对路径"
  [[ -n $SERVICE_NAME ]] || die "服务名不能为空"
  [[ ${#SERVICE_NAME} -le 200 ]] || die "服务名过长"

  local part
  local -a parts
  IFS=/ read -ra parts <<< "$PREFIX"
  for part in "${parts[@]}"; do
    [[ $part != .. ]] || die "安装目录不能包含 .."
  done
  IFS=/ read -ra parts <<< "$UNIT_DIR"
  for part in "${parts[@]}"; do
    [[ $part != .. ]] || die "单元目录不能包含 .."
  done

  case $SERVICE_NAME in
    [A-Za-z0-9]*) ;;
    *) die "服务名要以字母或数字开头" ;;
  esac
  case $SERVICE_NAME in
    *[!A-Za-z0-9._-]*) die "服务名只能包含字母、数字、点、下划线和连字符" ;;
  esac

  if [[ -n ${NMONITOR_PASSWORD-} ]]; then
    case $NMONITOR_PASSWORD in
      *$'\n'*|*$'\r'*) die "NMONITOR_PASSWORD 不能包含换行" ;;
    esac
  fi
}

require_template() {
  [[ -f $SCRIPT_DIR/nmonitor.service ]] || die "缺少单元模板：$SCRIPT_DIR/nmonitor.service"
  local key line
  for key in WorkingDirectory ExecStart EnvironmentFile; do
    local found=0
    while IFS= read -r line || [[ -n $line ]]; do
      if [[ $line == "$key="* ]]; then
        found=1
        break
      fi
    done < "$SCRIPT_DIR/nmonitor.service"
    [[ $found -eq 1 ]] || die "单元模板缺少 ${key}="
  done
}

require_runtime() {
  require_template
  if [[ $DRY_RUN -eq 1 ]]; then
    return 0
  fi
  [[ $(id -u) -eq 0 ]] || die "请使用 root 运行：sudo env NMONITOR_PASSWORD='你的密码' $0"
  command -v "$SYSTEMCTL" >/dev/null 2>&1 || die "未找到 systemctl，需要 systemd"
  command -v install >/dev/null 2>&1 || die "未找到 install 命令"
}

resolve_paths() {
  if [[ -n $BINARY ]]; then
    [[ -f $BINARY ]] || die "找不到二进制：$BINARY"
  else
    local candidate
    for candidate in "$PWD/nmonitor" "$REPO_ROOT/nmonitor"; do
      if [[ -f $candidate ]]; then
        BINARY=$candidate
        break
      fi
    done
    [[ -n $BINARY ]] || die "找不到 nmonitor 二进制。请先执行 make build，或使用 --binary 指定路径"
  fi

  if [[ -n $CONFIG ]]; then
    [[ -f $CONFIG ]] || die "找不到配置：$CONFIG"
  else
    local candidate
    for candidate in "$PWD/config.yaml" "$REPO_ROOT/config.yaml"; do
      if [[ -f $candidate ]]; then
        CONFIG=$candidate
        break
      fi
    done
    if [[ -z $CONFIG && ! -f $PREFIX/config.yaml ]]; then
      die "找不到 config.yaml。请使用 --config 指定"
    fi
  fi

  if [[ -n $ENV_SRC ]]; then
    [[ -f $ENV_SRC ]] || die "找不到环境变量模板：$ENV_SRC"
  else
    ENV_SRC=$SCRIPT_DIR/nmonitor.env.example
  fi
  if [[ ! -f $PREFIX/nmonitor.env && ! -f $ENV_SRC ]]; then
    die "找不到环境变量模板：$ENV_SRC"
  fi
}

maybe_prompt_password() {
  [[ $DRY_RUN -eq 0 ]] || return 0
  [[ -e $PREFIX/nmonitor.env ]] && return 0
  [[ -n ${NMONITOR_PASSWORD-} ]] && return 0
  [[ -t 0 ]] || return 0

  local first second
  if ! read -r -s -p "设置管理员密码（直接回车则使用示例值 change-me）：" first; then
    printf '\n'
    die "未读到密码"
  fi
  printf '\n'
  if [[ -z $first ]]; then
    return 0
  fi
  if ! read -r -s -p "再输入一次：" second; then
    printf '\n'
    die "未读到密码"
  fi
  printf '\n'
  [[ $first == "$second" ]] || die "两次输入的密码不一致"
  case $first in
    *$'\n'*|*$'\r'*) die "密码不能包含换行" ;;
  esac
  NMONITOR_PASSWORD=$first
}

same_file() {
  [[ -e $1 && -e $2 ]] || return 1
  [[ $(stat -c '%d:%i' "$1") == $(stat -c '%d:%i' "$2") ]]
}

format_env_assignment() {
  local value=$1 escaped
  case $value in
    *[!A-Za-z0-9_./:@+-]*)
      escaped=${value//\\/\\\\}
      escaped=${escaped//\"/\\\"}
      printf 'NMONITOR_PASSWORD="%s"\n' "$escaped"
      ;;
    *)
      printf 'NMONITOR_PASSWORD=%s\n' "$value"
      ;;
  esac
}

install_tree() {
  local dest_bin=$PREFIX/nmonitor
  local dest_config=$PREFIX/config.yaml
  local dest_env=$PREFIX/nmonitor.env

  ensure_dir "$PREFIX" 0755
  ensure_dir "$PREFIX/data" 0755
  ensure_dir "$PREFIX/data/backups" 0755

  if same_file "$BINARY" "$dest_bin"; then
    log "二进制已就位：$dest_bin"
  else
    local staged=$PREFIX/nmonitor.new
    if [[ $DRY_RUN -eq 0 ]]; then
      TMP_FILES+=("$staged")
    fi
    run install -m 0755 -o root -g root "$BINARY" "$staged"
    run mv -f "$staged" "$dest_bin"
  fi

  if [[ -e $dest_config ]]; then
    log "保留已有配置：$dest_config"
  else
    [[ -n $CONFIG ]] || die "找不到 config.yaml。请使用 --config 指定"
    run install -m 0644 -o root -g root "$CONFIG" "$dest_config"
  fi

  if [[ -e $dest_env ]]; then
    log "保留已有环境文件：$dest_env"
  elif [[ -n ${NMONITOR_PASSWORD-} ]]; then
    write_env_file "$dest_env"
  else
    run install -m 0600 -o root -g root "$ENV_SRC" "$dest_env"
  fi

  if [[ $DRY_RUN -eq 0 && -f $dest_env ]] && grep -q '^NMONITOR_PASSWORD=change-me$' "$dest_env"; then
    log "请编辑 $dest_env，把 NMONITOR_PASSWORD 从示例值 change-me 改成实际密码"
  fi
}

write_env_file() {
  local dest=$1 tmp found=0 line
  if [[ $DRY_RUN -eq 1 ]]; then
    log "+ 写入 $dest（含 NMONITOR_PASSWORD，不打印密码）"
    return 0
  fi
  tmp=$(mktemp)
  TMP_FILES+=("$tmp")
  chmod 0600 "$tmp"
  while IFS= read -r line || [[ -n $line ]]; do
    if [[ $line == NMONITOR_PASSWORD=* || $line == export\ NMONITOR_PASSWORD=* ]]; then
      format_env_assignment "$NMONITOR_PASSWORD"
      found=1
    else
      printf '%s\n' "$line"
    fi
  done < "$ENV_SRC" > "$tmp"
  if [[ $found -eq 0 ]]; then
    format_env_assignment "$NMONITOR_PASSWORD" >> "$tmp"
  fi
  install -m 0600 -o root -g root "$tmp" "$dest"
  log "已写入环境文件：$dest"
}

render_unit() {
  local line
  while IFS= read -r line || [[ -n $line ]]; do
    case $line in
      WorkingDirectory=*) printf 'WorkingDirectory=%s\n' "$PREFIX" ;;
      ExecStart=*) printf 'ExecStart=%s/nmonitor\n' "$PREFIX" ;;
      EnvironmentFile=*) printf 'EnvironmentFile=-%s/nmonitor.env\n' "$PREFIX" ;;
      *) printf '%s\n' "$line" ;;
    esac
  done < "$SCRIPT_DIR/nmonitor.service"
}

write_unit() {
  local unit_path=$UNIT_DIR/$SERVICE_NAME.service
  if [[ $DRY_RUN -eq 1 ]]; then
    printf '+ write %q\n' "$unit_path"
    render_unit | sed 's/^/+ | /'
    return 0
  fi
  ensure_dir "$UNIT_DIR" 0755
  local tmp
  tmp=$(mktemp "$UNIT_DIR/.${SERVICE_NAME}.service.XXXXXX")
  TMP_FILES+=("$tmp")
  render_unit > "$tmp"
  chown root:root "$tmp"
  chmod 0644 "$tmp"
  mv -f "$tmp" "$unit_path"
  log "已写入单元：$unit_path"
}

activate_service() {
  local was_active=0
  if [[ $DRY_RUN -eq 0 ]] && "$SYSTEMCTL" is-active --quiet "$SERVICE_NAME"; then
    was_active=1
  fi
  run "$SYSTEMCTL" daemon-reload
  if [[ $ENABLE -eq 1 ]]; then
    run "$SYSTEMCTL" enable "$SERVICE_NAME"
  fi
  if [[ $START -eq 1 || $was_active -eq 1 ]]; then
    run "$SYSTEMCTL" restart "$SERVICE_NAME"
  fi
}

summarize() {
  if [[ $DRY_RUN -eq 1 ]]; then
    log "dry-run：未修改系统"
    return 0
  fi
  log "NMonitor 已安装为 systemd 服务 ${SERVICE_NAME}"
  log "目录：${PREFIX}"
  log "单元：${UNIT_DIR}/${SERVICE_NAME}.service"
  if [[ $START -eq 1 ]]; then
    log "管理台默认地址：http://127.0.0.1:8080（端口以 config.yaml 的 server.port 为准）"
  fi
}

main() {
  parse_args "$@"
  validate_options
  require_runtime
  resolve_paths
  maybe_prompt_password
  install_tree
  write_unit
  activate_service
  summarize
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
  main "$@"
fi
