#!/usr/bin/env bash
# Build the universal macOS app, refresh the .dmg for teammates and install
# the app in /Applications (moved, not copied: exactly one Leme.app on the Mac).
#   scripts/install-mac.sh
set -euo pipefail
cd "$(dirname "$0")/.."
WAILS=${WAILS:-$(go env GOPATH)/bin/wails}
VERSION=$(python3 -c 'import json;print(json.load(open("wails.json"))["info"]["productVersion"])')
DMG="dist/mac/Leme-${VERSION}-macos-universal.dmg"

# Local builds report "<version>-dev": no update notice on startup (CI releases use the tag).
"$WAILS" build -clean -trimpath -platform darwin/universal -ldflags "-s -w -X main.version=${VERSION}-dev"

mkdir -p dist/mac
STAGE=$(mktemp -d)
cp -R build/bin/Leme.app "$STAGE/"
cp THIRD_PARTY_NOTICES.txt ../../LICENSE "$STAGE/" 2>/dev/null || true
ln -s /Applications "$STAGE/Applications"
hdiutil create -volname Leme -srcfolder "$STAGE" -fs HFS+ -size 300m -format UDZO -ov "$DMG" >/dev/null
rm -rf "$STAGE"
hdiutil verify "$DMG" >/dev/null

# Quit a running copy so the new one is what opens next (and the pre-rename app).
osascript -e 'tell application "Leme" to quit' >/dev/null 2>&1 || true
osascript -e 'tell application "btu" to quit' >/dev/null 2>&1 || true
rm -rf /Applications/Leme.app /Applications/btu.app
mv build/bin/Leme.app /Applications/Leme.app

echo "installed: /Applications/Leme.app"
echo "dmg:       $DMG"
mdfind "kMDItemFSName == 'Leme.app' || kMDItemFSName == 'btu.app'" 2>/dev/null | sed 's/^/copy on disk: /'
