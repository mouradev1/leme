# Security

Leme holds access to Kubernetes clusters, often production ones. Security
reports are taken seriously and handled with priority.

## Reporting a vulnerability

Please **do not open a public issue**. Use GitHub's private reporting instead:
*Security > Report a vulnerability* on this repository. Include the version
(shown at the bottom of the sidebar), your OS, and steps to reproduce.

You'll get an acknowledgement within a few days. Fixes are released as a new
version with a note in the release changelog.

## What Leme does with your data

- **No telemetry.** Leme does not collect usage data, has no accounts and
  talks to no server other than the Kubernetes API servers you open.
- **Credentials stay on your machine.** Clusters come from your kubeconfig.
  Configs pasted into the app are stored in a private per-user folder
  (directory `0700`, files `0600`) and deleted when removed in the app.
  The local terminal uses a temporary single-context kubeconfig in the same
  kind of private folder, removed when the shell exits and swept at startup.
- **Isolated UI.** A strict Content-Security-Policy prevents the interface
  from loading remote code or sending data anywhere; all cluster traffic goes
  through the Go backend.
- **TLS is always verified.** Leme never disables certificate verification.
- **Secret values** are only fetched when you open a Secret, are hidden until
  you reveal them, and are never logged.

## Supply chain

Every change runs `govulncheck`, `go mod verify`, `npm audit` and
`npm audit signatures` in CI. Dependencies are updated weekly by Dependabot.
Release binaries are built by GitHub Actions and published with SHA-256
checksums.

Run the same audit locally:

```bash
apps/desktop/scripts/security-check.sh
```
