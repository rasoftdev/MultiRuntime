#!/usr/bin/env bash
# mrt remote installer: downloads the repo, builds, installs into ~/.mrt and removes all temporary files.
#
#   curl -fsSL https://raw.githubusercontent.com/rasoftdev/MultiRuntime/main/get.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/rasoftdev/MultiRuntime/main/get.sh | bash -s -- --write-rc
#   curl -fsSL https://raw.githubusercontent.com/rasoftdev/MultiRuntime/main/get.sh | bash -s -- uninstall
#
# No sudo needed: everything goes to your home ($MRT_HOME, default ~/.mrt).
# Updating = running the same command again (installed PHP versions are kept).
# Optional variables: MRT_REPO (git URL), MRT_REF (branch/tag, default: main), MRT_HOME.
set -euo pipefail

REPO="${MRT_REPO:-https://github.com/rasoftdev/MultiRuntime.git}"
REF="${MRT_REF:-main}"
GO_VERSION="1.24.4"   # only used when the system has no Go >= 1.22
HOME_DIR="${MRT_HOME:-$HOME/.mrt}"

say() { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(uname -s)" = "Linux" ] || die "mrt only runs on Linux (WSL included)."
if [ "$(id -u)" -eq 0 ] && [ -z "${MRT_ALLOW_ROOT:-}" ]; then
  die "Do not run as root: mrt installs into your own home. Remove sudo (or set MRT_ALLOW_ROOT=1)."
fi

if [ "${1:-}" = "uninstall" ]; then
  say "Uninstalling mrt from $HOME_DIR (this also deletes every PHP version installed by mrt)..."
  rm -rf "$HOME_DIR"
  for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
    [ -f "$rc" ] || continue
    if grep -qE 'mrt env|# mrt \(multi runtime manager\)' "$rc"; then
      tmp="$(mktemp)"
      grep -vE 'mrt env|# mrt \(multi runtime manager\)' "$rc" > "$tmp" || true
      cat "$tmp" > "$rc"; rm -f "$tmp"
      say "Cleaned $rc"
    fi
  done
  say "Done. Open a new shell to drop the PATH entries."
  exit 0
fi

case "$(uname -m)" in
  x86_64|amd64)  ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "Unsupported architecture: $(uname -m)" ;;
esac

command -v git  >/dev/null || die "git is missing (apt install git / dnf install git)."
command -v curl >/dev/null || die "curl is missing."

# Everything temporary lives here and is always deleted on exit (success or failure).
WORK="$(mktemp -d "${TMPDIR:-/tmp}/mrt-build.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

say "Downloading the repository ($REF)..."
git clone --quiet --depth 1 --branch "$REF" "$REPO" "$WORK/repo"

# Go: use the system one if recent enough; otherwise a temporary copy inside $WORK.
need_go=1
if command -v go >/dev/null; then
  have="$(go env GOVERSION 2>/dev/null | sed 's/^go//')"
  [ -n "$have" ] && [ "$(printf '%s\n1.22\n' "$have" | sort -V | head -1)" = "1.22" ] && need_go=0
fi
if [ "$need_go" -eq 1 ]; then
  say "Downloading a temporary Go $GO_VERSION (it will be removed when finished)..."
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" | tar -C "$WORK" -xz
  export PATH="$WORK/go/bin:$PATH"
fi
export GOCACHE="$WORK/gocache" GOPATH="$WORK/gopath" GOTOOLCHAIN=local CGO_ENABLED=0

say "Building..."
mkdir -p "$WORK/repo/dist"
( cd "$WORK/repo" && go build -trimpath -ldflags "-s -w" -o "dist/mrt-linux-$ARCH" ./cmd/mrt )

if [ "${MRT_BUILD_ONLY:-}" = "1" ]; then
  say "Build OK (test mode; nothing is installed)."
  "$WORK/repo/dist/mrt-linux-$ARCH" version
  exit 0
fi

say "Installing into $HOME_DIR..."
MRT_HOME="$HOME_DIR" bash "$WORK/repo/scripts/install.sh" "$@"

say "Cleaning up temporary files..."
# (the trap removes $WORK on exit)
cat <<'EOF'

Installed. To use it now:   eval "$(~/.mrt/bin/mrt env)"
Try:                        mrt php install 8.3   |   mrt php local 7.4
Compiling PHP 7.4/8.x from source needs: gcc make perl pkg-config xz (and autoconf m4 for 'ext add').
EOF
