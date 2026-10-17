#!/usr/bin/env bash
# Builds the Linux app from macOS (or anywhere with Docker).
#   scripts/build-linux.sh            # amd64 (most PCs/servers)
#   scripts/build-linux.sh arm64      # ARM (Raspberry Pi 5, Ampere, etc.)
# Outputs in dist/linux/: leme-linux-<arch> (self-installing binary) and leme_<ver>_<arch>.deb
set -euo pipefail
ARCH=${1:-amd64}
cd "$(dirname "$0")/.."
# Rebuild the builder image only when its Dockerfile changes: avoids hitting
# Docker Hub on every build (and failing when the registry is flaky).
HASH=$(shasum build/linux/Dockerfile | cut -c1-12)
IMAGE="leme-linux-builder:$ARCH-$HASH"
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  docker build --platform "linux/$ARCH" -t "$IMAGE" build/linux
fi
docker run --rm --platform "linux/$ARCH" \
  -v "$PWD":/src \
  -v "leme-gomod-$ARCH":/root/go/pkg/mod \
  -v leme-npm:/root/.npm \
  -e VERSION="${VERSION:-$(python3 -c 'import json;print(json.load(open("wails.json"))["info"]["productVersion"])')}" \
  "$IMAGE" bash /src/build/linux/build.sh
ls -lh dist/linux
