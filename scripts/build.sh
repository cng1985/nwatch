#!/usr/bin/env bash
# 在 Linux 上构建 NMonitor。前端先编译并嵌入二进制。
# 用法:
#   ./scripts/build.sh            当前架构的 Linux 程序
#   ./scripts/build.sh windows    交叉编译 Windows 程序
#   ./scripts/build.sh all        Linux 和 Windows 都构建
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TARGET="${1:-linux}"

require() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[build] 缺少命令: $1" >&2
    exit 1
  fi
}

host_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    i386|i686) echo 386 ;;
    *)
      echo "[build] 无法识别的架构: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

build_web() {
  echo "[build] 构建前端"
  (
    cd web
    if [[ -f package-lock.json ]]; then
      npm ci
    else
      npm install
    fi
    npm run build
  )
}

build_go() {
  local goos="$1"
  local goarch="$2"
  local outdir="dist/${goos}-${goarch}"
  local outfile="nmonitor"
  if [[ "$goos" == "windows" ]]; then
    outfile="nmonitor.exe"
  fi
  mkdir -p "$outdir"
  echo "[build] 构建 ${goos}/${goarch} -> ${outdir}/${outfile}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "-s -w" -o "${outdir}/${outfile}" ./cmd/server
  cp -f config.yaml "${outdir}/config.yaml"
  echo "[build] 完成 ${outdir}/${outfile}"
}

require go
require npm

ARCH="$(host_arch)"
build_web

case "$TARGET" in
  linux)
    build_go linux "$ARCH"
    ;;
  windows|cmd)
    build_go windows "$ARCH"
    ;;
  all)
    build_go linux "$ARCH"
    build_go windows "$ARCH"
    ;;
  *)
    echo "用法: $0 [linux|windows|all]" >&2
    exit 1
    ;;
esac

echo "[build] 产物目录: ${ROOT}/dist"
