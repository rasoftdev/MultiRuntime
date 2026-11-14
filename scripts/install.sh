#!/bin/sh
# Installs mrt (and the php/composer shims) into ~/.mrt.
#
#   ./scripts/install.sh                 use the binary in ../dist for this machine
#   MRT_URL=https://host/mrt-linux-amd64 ./scripts/install.sh   download it instead
#   ./scripts/install.sh --write-rc      also add the PATH line to ~/.bashrc / ~/.zshrc
set -eu

case "$(uname -m)" in
  x86_64|amd64)  arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac
case "$(uname -s)" in
  Linux)  os=linux ;;
  Darwin) os=darwin ;;   # experimental: prebuilt PHP binaries only (8.0+)
  *) echo "mrt supports Linux and, experimentally, macOS" >&2; exit 1 ;;
esac

here=$(cd "$(dirname "$0")" && pwd)
bin="$here/../dist/mrt-$os-$arch"
tmp=""
if [ -n "${MRT_URL:-}" ]; then
  tmp=$(mktemp)
  trap 'rm -f "$tmp"' EXIT
  curl -fsSL "$MRT_URL" -o "$tmp"
  bin=$tmp
fi
[ -f "$bin" ] || { echo "binary not found: $bin (run scripts/build.sh or set MRT_URL)" >&2; exit 1; }
chmod +x "$bin"
"$bin" setup

if [ "${1:-}" = "--write-rc" ]; then
  line='eval "$(~/.mrt/bin/mrt env)"'
  for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
    [ -f "$rc" ] || continue
    grep -qF "$line" "$rc" || printf '\n# mrt (multi runtime manager)\n%s\n' "$line" >> "$rc"
    echo "updated $rc"
  done
fi
