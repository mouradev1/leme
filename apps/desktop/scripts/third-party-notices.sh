#!/usr/bin/env bash
# Generates THIRD_PARTY_NOTICES.txt: the license texts and NOTICE files of
# every third-party component shipped inside Leme (Go modules compiled into the
# binary + npm packages bundled into the UI + assets inside them).
#
# MIT/BSD/ISC/Apache-2.0 require these texts to accompany binary
# distributions; Apache-2.0 also requires forwarding NOTICE files; MPL-2.0
# requires telling recipients where the source is. Run before each release
# (the Release workflow does it automatically):
#   scripts/third-party-notices.sh [output-file]
set -euo pipefail
cd "$(dirname "$0")/.."
OUT=${1:-THIRD_PARTY_NOTICES.txt}
GO_LICENSES=github.com/google/go-licenses/v2@v2.0.1
LICENSE_CHECKER=license-checker-rseidelsohn@4.4.2

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

# Go: one directory per module with its LICENSE/COPYING/NOTICE files.
# --ignore leme: our own code (Apache-2.0, see LICENSE at the repo root).
GOFLAGS=-mod=mod go run "$GO_LICENSES" report ./... --ignore leme >"$WORK/go.csv" 2>/dev/null
GOFLAGS=-mod=mod go run "$GO_LICENSES" save ./... --ignore leme --save_path="$WORK/go" --force 2>/dev/null

# npm: production dependencies only (what ends up in the app bundle).
(cd frontend && npx --yes "$LICENSE_CHECKER" --production --json \
  --excludePackages "$(node -p 'const p=require("./package.json");p.name+"@"+p.version')") >"$WORK/npm.json"

python3 - "$WORK" "$OUT" <<'PY'
import csv, json, os, pathlib, sys

work, out = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2])
NAMES = ("license", "licence", "copying", "notice", "unlicense")
parts = []

def section(title):
    parts.append("\n" + "=" * 78 + f"\n{title}\n" + "=" * 78 + "\n")

parts.append("""LEME: THIRD-PARTY NOTICES

Leme is licensed under the Apache License 2.0 (see LICENSE).
It includes the third-party components listed below, each under its own
license. Their copyright notices, license texts and NOTICE files follow.

Components under the Mozilla Public License 2.0 are used unmodified; their
source code is available at the URLs listed next to them.
""")

# ---- Go modules ---------------------------------------------------------------
rows = sorted(csv.reader(open(work / "go.csv")), key=lambda r: r[0])
section(f"PART 1: Go modules compiled into the Leme binary ({len(rows)} packages)")
for pkg, url, lic in rows:
    parts.append(f"  {pkg}  [{lic}]  {url}")

go_root = work / "go"
dirs = {}
for p in sorted(go_root.rglob("*")):
    if p.is_file() and p.name.lower().startswith(NAMES) and "/." not in str(p):
        dirs.setdefault(p.parent, []).append(p)
for d, files in sorted(dirs.items()):
    module = d.relative_to(go_root).as_posix()
    # skip nested copies inside a module already printed (e.g. vendored subdirs)
    section(module)
    for f in sorted(files):
        parts.append(f"--- {f.name} ---\n" + f.read_text(errors="replace").strip() + "\n")

# ---- npm packages -------------------------------------------------------------
npm = json.load(open(work / "npm.json"))
section(f"PART 2: npm packages bundled into the Leme interface ({len(npm)} packages)")
for name, info in sorted(npm.items()):
    parts.append(f"  {name}  [{info.get('licenses')}]  {info.get('repository', '')}")
for name, info in sorted(npm.items()):
    section(name)
    lf = info.get("licenseFile")
    if lf and os.path.basename(lf).lower().startswith(NAMES):
        parts.append(pathlib.Path(lf).read_text(errors="replace").strip() + "\n")
    else:
        parts.append(f"License: {info.get('licenses')} (no license file shipped; see {info.get('repository', 'the package')})\n")
    # Monaco bundles third-party components (incl. CC-BY-4.0 material).
    tpn = pathlib.Path(info.get("path", "")) / "ThirdPartyNotices.txt"
    if tpn.is_file():
        parts.append(f"--- {name}: ThirdPartyNotices.txt ---\n" + tpn.read_text(errors="replace").strip() + "\n")

# ---- assets -------------------------------------------------------------------
section("PART 3: Fonts and icons")
parts.append("""Codicons (icon font used by the Monaco editor)
Copyright (c) Microsoft Corporation. https://github.com/microsoft/vscode-codicons
Icons licensed under Creative Commons Attribution 4.0 International (CC BY 4.0):
https://creativecommons.org/licenses/by/4.0/ . Code licensed under the MIT License.
Used unmodified.

The Leme app icon and logo are original artwork of the Leme project, licensed
with the project under the Apache License 2.0.
""")

out.write_text("\n".join(parts) + "\n")
print(f"wrote {out} ({out.stat().st_size // 1024} KB, {len(rows)} Go packages, {len(npm)} npm packages)")
PY
