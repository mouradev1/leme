#!/usr/bin/env bash
# Security audit (offline-safe: only queries public vuln DBs/registries with
# package names+versions; never touches any cluster).
#   scripts/security-check.sh
set -uo pipefail
cd "$(dirname "$0")/.."
GOVULN=${GOVULN:-$(go env GOPATH)/bin/govulncheck}
fail=0

echo "== Go: known vulnerabilities (code paths actually reachable) =="
"$GOVULN" ./... || fail=1

echo "== Go: module integrity vs go.sum =="
go mod verify || fail=1

echo "== npm: known vulnerabilities =="
(cd frontend && npm audit) || fail=1

echo "== npm: registry signatures / provenance =="
(cd frontend && npm audit signatures) || fail=1

echo "== Code: TLS verification never disabled =="
if grep -rn "InsecureSkipVerify" --include='*.go' internal main.go; then fail=1; else echo ok; fi

echo "== Code: no raw HTML / eval in the UI =="
if grep -rn "v-html\|innerHTML\|insertAdjacentHTML\|eval(\|new Function" frontend/src; then fail=1; else echo ok; fi

echo "== Bundle: CSP present, no source maps =="
grep -q "Content-Security-Policy" frontend/index.html && echo "CSP ok" || fail=1
if find frontend/dist -name '*.map' | grep -q .; then echo "source maps found"; fail=1; else echo "no source maps"; fi

exit $fail
