#!/bin/sh
# Cross-compiles mrt for Linux and macOS (amd64 and arm64) into dist/.
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
for os in linux darwin; do
  for arch in amd64 arm64; do
    GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "dist/mrt-$os-$arch" ./cmd/mrt
    echo "built dist/mrt-$os-$arch"
  done
done
