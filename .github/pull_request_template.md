## What

<!-- What does this change? One topic per PR. -->

## Why

<!-- The problem it solves. Link the issue: Fixes #123 -->

## How it was tested

<!-- Commands run and the scenario tried. Use a local cluster (kind, minikube, k3d), never production. -->

- [ ] `go vet ./... && go test -race ./...` (in `apps/desktop`)
- [ ] `npm run typecheck` (in `apps/desktop/frontend`)
- [ ] `scripts/security-check.sh`
- [ ] Screenshots in light and dark themes (UI changes only)

## Notes for reviewers

<!-- Anything risky, follow-ups, or decisions worth discussing. -->
