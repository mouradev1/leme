# Contributing to Leme

Thanks for your interest! Bug reports, ideas and pull requests are welcome.

## Development setup

Requirements: Go 1.26+, Node.js 20+ and the
[Wails prerequisites](https://wails.io/docs/gettingstarted/installation).

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
cd apps/desktop
wails dev
```

VS Code users: `Cmd/Ctrl+Shift+B` runs `wails dev`; `F5` debugs the Go
backend with the UI served by Vite (see `.vscode/`).

## Before opening a pull request

```bash
cd apps/desktop
go vet ./... && go test -race ./...
(cd frontend && npm run typecheck)
scripts/security-check.sh
```

Please test against a **local** cluster (kind, minikube, k3d), never against
a production cluster.

## Commit messages

Leme follows [Conventional Commits](https://www.conventionalcommits.org/).
Release notes are generated from them, so they matter.

```
<type>(<optional scope>): <summary>

<optional body: what and why, wrapped at 72 columns>

<optional footer: BREAKING CHANGE: ..., Refs #123>
```

**Types**

| Type | Use for |
|---|---|
| `feat` | a new feature for users |
| `fix` | a bug fix |
| `perf` | a performance or memory improvement |
| `refactor` | code change that neither fixes a bug nor adds a feature |
| `docs` | documentation or the project page |
| `test` | adding or fixing tests |
| `build` | build scripts, packaging, dependencies |
| `ci` | GitHub Actions workflows |
| `chore` | maintenance that doesn't fit above |
| `revert` | reverting a previous commit |

**Scopes** (optional): `cluster`, `resources`, `pods`, `logs`, `terminal`,
`metrics`, `helm`, `editor`, `ui`, `theme`, `security`, `linux`, `macos`,
`windows`, `docs`, `release`.

**Rules**

- Summary in the imperative mood, lower case, no trailing period, up to 72
  characters: `fix(logs): keep follow mode after reconnect`.
- One logical change per commit. Formatting-only changes go in their own commit.
- Breaking changes: add `!` after the type (`feat(cluster)!: ...`) and explain
  in a `BREAKING CHANGE:` footer.
- Reference issues in the footer: `Fixes #42`.

Examples:

```
feat(editor): add find and replace to the YAML editor
fix(pods): show previous logs for restarted init containers
perf(resources): coalesce watch deltas per UID before emitting
docs: explain macOS quarantine on first launch
```

Optional local check (rejects malformed messages before they are recorded):

```bash
git config core.hooksPath .githooks
```

## Branches

Branch from `main` and name the branch after the change type:
`feat/<short-description>`, `fix/<issue-number>-<short-description>`,
`docs/...`, `ci/...`. Example: `fix/42-logs-reconnect`.

## Pull requests

- **Title** follows the commit convention (a CI check enforces it); it becomes
  the line in the release notes.
- Keep PRs focused on one topic. Large changes: open an issue first to agree on
  the approach.
- Fill in the PR template: what changed, why, how it was tested.
- UI changes: include screenshots in both light and dark themes.
- CI must be green (tests, type-check, vulnerability scan).
- PRs are squash-merged, so the PR title is the final commit message.

## Versioning

[Semantic Versioning](https://semver.org/): `MAJOR.MINOR.PATCH`. While the
version is `0.x`, minor versions may contain breaking changes. Releases are
created by maintainers by pushing a `vX.Y.Z` tag.

## Guidelines

- **Keep it light.** Memory and CPU are the product. Stream only what's on
  screen, bound every buffer, lazy-load heavy UI (Monaco, xterm).
- **Generic over per-kind code.** Lists use the API server's Table format, so
  new kinds and CRDs work without new code.
- **Never weaken security**: no disabled TLS verification, no remote code in
  the UI, credentials only in private files.
- **UI text** is Brazilian Portuguese, plain and neutral. No emojis in the UI;
  use the SVG icons in `frontend/src/lib/icons.ts`.
- Go code is formatted with `gofmt`; keep comments explaining *why*.

## Project layout

See the repository layout section in [README.md](README.md).

By contributing you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE).
