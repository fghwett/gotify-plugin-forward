#!/usr/bin/env bash
# 校验编译出的插件与目标 gotify/server 镜像的共享依赖版本一致。
# Go plugin 要求宿主与插件中相同 import path 的包完全一致（版本 + 内容哈希），
# 任一共享依赖不一致都会导致插件加载失败。
#
# 用法: scripts/verify-compat.sh <gotify-server镜像标签> [so文件...] [--keep]
# 示例: scripts/verify-compat.sh 3.1.0 build/gotify-plugin-forward-linux-amd64.so

set -euo pipefail

# sort/join 的比较必须用字节序（C locale），否则含 '-' 的模块名排序不一致
export LC_ALL=C

IMAGE_TAG="${1:?用法: verify-compat.sh <gotify-server镜像标签> [so文件...]}"
shift
KEEP=0
SO_FILES=()
for arg in "$@"; do
  if [ "$arg" = "--keep" ]; then KEEP=1; else SO_FILES+=("$arg"); fi
done
[ ${#SO_FILES[@]} -gt 0 ] || { echo "错误: 未指定 so 文件" >&2; exit 2; }

WORKDIR=$(mktemp -d)
trap '[ $KEEP -eq 1 ] || rm -rf "$WORKDIR"' EXIT

# 提取官方镜像内 gotify 二进制的依赖版本表
echo "==> 提取 gotify/server:${IMAGE_TAG} 二进制 buildinfo..."
if ! docker image inspect "gotify/server:${IMAGE_TAG}" >/dev/null 2>&1; then
  docker pull "gotify/server:${IMAGE_TAG}" >/dev/null
fi
docker run --rm --entrypoint cat "gotify/server:${IMAGE_TAG}" /app/gotify-app > "$WORKDIR/gotify-app"

# 依赖表格式: "module version"。
# go version -m 的 replace 信息是独立的 "=>" 行，需覆盖 dep 行的原始版本。
parse_deps() {
  awk '$1=="dep" {ver[$2]=$3} $1=="=>" {ver[$2]=$3} END {for (m in ver) print m, ver[m]}'
}

go version -m "$WORKDIR/gotify-app" | parse_deps | sort > "$WORKDIR/gotify.deps"

FAIL=0
for so in "${SO_FILES[@]}"; do
  echo "==> 校验 ${so}"
  go version -m "$so" | parse_deps | sort > "$WORKDIR/plugin.deps"

  # 只比较两边共享的依赖（插件私有依赖不在 gotify 二进制中，不参与校验）
  MISMATCH=$(join "$WORKDIR/gotify.deps" "$WORKDIR/plugin.deps" \
    | awk '$2 != $3 {print "    " $1 ": gotify=" $2 " plugin=" $3}')
  if [ -n "$MISMATCH" ]; then
    echo "    共享依赖版本不一致:" >&2
    echo "$MISMATCH" >&2
    echo "    请更新 go.mod 中的 require/replace 使其与镜像一致" >&2
    FAIL=1
  else
    SHARED=$(join "$WORKDIR/gotify.deps" "$WORKDIR/plugin.deps" | wc -l)
    echo "    通过（共享依赖 ${SHARED} 个全部一致）"
  fi
done

[ $FAIL -eq 0 ] && echo "==> 兼容性校验全部通过" || exit 1
