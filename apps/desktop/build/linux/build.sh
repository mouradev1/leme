#!/usr/bin/env bash
# Runs INSIDE the builder container (see scripts/build-linux.sh). Source is
# mounted at /src; outputs go to /src/dist/linux. The host's node_modules
# (macOS binaries) is never reused.
set -euo pipefail
ARCH=$(dpkg --print-architecture)
VERSION=${VERSION:-0.1.0}
OUT=/src/dist/linux
mkdir -p "$OUT"

rm -rf /work && mkdir -p /work
tar -C /src --exclude=./frontend/node_modules --exclude=./build/bin --exclude=./dist -cf - . | tar -C /work -xf -
cd /work

wails build -clean -trimpath -tags webkit2_41 -ldflags "-s -w -X main.version=${VERSION}" -o leme

# 1) Standalone binary: run it once and it installs itself in the app menu.
cp build/bin/leme "$OUT/leme-linux-$ARCH"
bash scripts/third-party-notices.sh >/dev/null || echo "warning: could not regenerate third-party notices"
# 2) .deb for Ubuntu/Debian/Mint (double-click to install; apt pulls deps).
bash build/linux/package-deb.sh build/bin/leme "$VERSION" "$ARCH" "$OUT"
file "$OUT/leme-linux-$ARCH"
