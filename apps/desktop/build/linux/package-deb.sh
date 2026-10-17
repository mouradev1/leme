#!/usr/bin/env bash
# Package a built Leme binary as a .deb (Ubuntu/Debian/Mint).
#   package-deb.sh <binary> <version> <arch:amd64|arm64> <outdir>
# Used by build/linux/build.sh (Docker, local) and by the GitHub release workflow.
set -euo pipefail
BIN=$1 VERSION=${2#v} ARCH=$3 OUT=$4
ICON="$(dirname "$0")/../appicon.png"
mkdir -p "$OUT"

PKG=$(mktemp -d)
install -Dm755 "$BIN" "$PKG/usr/bin/leme"
install -Dm644 "$ICON" "$PKG/usr/share/icons/hicolor/512x512/apps/leme.png"
mkdir -p "$PKG/usr/share/applications" "$PKG/DEBIAN" "$PKG/usr/share/doc/leme"
# Licenses: Leme (Apache-2.0) + every bundled third-party component.
NOTICES="$(dirname "$0")/../../THIRD_PARTY_NOTICES.txt"
{
  echo "Leme"
  echo "License: Apache-2.0 (full text: /usr/share/common-licenses/Apache-2.0)"
  echo
  if [ -f "$NOTICES" ]; then cat "$NOTICES"; else echo "Third-party notices: see THIRD_PARTY_NOTICES.txt in the release."; fi
} > "$PKG/usr/share/doc/leme/copyright"
chmod 644 "$PKG/usr/share/doc/leme/copyright"
cat > "$PKG/usr/share/applications/leme.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Leme
GenericName=Kubernetes IDE
Comment=IDE desktop leve para Kubernetes
Exec=/usr/bin/leme %U
Icon=leme
Terminal=false
Categories=Development;System;
Keywords=kubernetes;k8s;kubectl;helm;
StartupWMClass=leme
EOF
cat > "$PKG/DEBIAN/control" <<EOF
Package: leme
Version: $VERSION
Section: devel
Priority: optional
Architecture: $ARCH
Depends: libgtk-3-0, libwebkit2gtk-4.1-0
Maintainer: Leme contributors <noreply@localhost>
Description: IDE desktop leve para Kubernetes
 Clusters, workloads, logs, shell, metricas,
 editor YAML e Helm, com baixo consumo de memoria.
EOF
dpkg-deb --build --root-owner-group "$PKG" "$OUT/leme_${VERSION}_${ARCH}.deb" >/dev/null
rm -rf "$PKG"
echo "$OUT/leme_${VERSION}_${ARCH}.deb"
