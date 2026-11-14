#!/bin/sh
# Cross-compiles mrt for Linux amd64 and arm64 into dist/.
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
for arch in amd64 arm64; do
  GOOS=linux GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "dist/mrt-linux-$arch" ./cmd/mrt
  echo "built dist/mrt-linux-$arch"
done
