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

- **No telemetry.** Leme does not collect usage data and has no accounts. It
  talks only to the Kubernetes API servers you open, plus one optional request:
- **Update notice.** At most once a day (at startup or on first use that day) Leme asks GitHub's public
  releases endpoint (`api.github.com/repos/mouradev1/leme/releases/latest`)
  whether a newer version exists. It sends no data about you, your clusters or
  your usage (User-Agent is just `Leme`); GitHub only sees your IP, as with any
  page visit. Turn it off in *Settings* or in the update dialog.
  It never downloads or installs anything by itself: *Baixar* opens the
  release page in your browser.
- **Credentials stay on your machine.** Clusters come from your kubeconfig.
  Configs pasted into the app are stored in a private per-user folder
  (directory `0700`, files `0600`) and deleted when removed in the app.
  The local terminal uses a temporary single-context kubeconfig in the same
  kind of private folder, removed when the shell exits and swept at startup.
- **Rancher tokens** are kept in the OS password store (macOS Keychain,
  Windows Credential Manager, Secret Service on Linux; a `0600` file only when
  no Secret Service is running). On macOS the token is passed to the
  `security` tool on stdin, never as a command-line argument. Browser logins
  use Rancher's CLI flow: the token is encrypted with a throwaway key pair, so
  only this Leme can read it, and *Log out* revokes it on the server. Only
  `https://` Rancher addresses are accepted.
- **Kubeconfig credential commands.** When a kubeconfig wants to run a program
  to get credentials (`exec` plugins such as kubelogin or aws), Leme shows the
  exact command and runs it only after you allow it, once per command line.
- **Production and read-only clusters.** Clusters marked as production ask for
  the cluster name before risky actions; read-only clusters have every change
  refused in the backend, not just hidden in the UI.
- **Isolated UI.** A strict Content-Security-Policy prevents the interface
  from loading remote code or sending data anywhere; all cluster traffic goes
  through the Go backend.
- **TLS is always verified.** Leme never disables certificate verification.
- **Secret values** are only fetched when you open a Secret, are hidden until
  you reveal them, and are never logged.
- **Node shell** (root shell on a node) is opt-in and asks for confirmation.
  It creates a temporary privileged pod (`kube-system`, labelled
  `leme.dev/node-shell=true`, image `busybox` pinned by digest) pinned to that node, enters the
  host with `nsenter`, and deletes the pod when the terminal closes. As a last
  resort the pod expires after 8 hours. It needs RBAC permission to create
  privileged pods in `kube-system`; clusters that forbid it simply refuse.
  It can be turned off in *Settings*, or pointed to an image in an internal
  registry (it must be pinned by digest).

## Supply chain

Every change runs `govulncheck`, `go mod verify`, `npm audit` and
`npm audit signatures` in CI. GitHub Actions are pinned by commit hash.
Helm charts are only fetched over `https://` or from `oci://` registries. Dependencies are updated weekly by Dependabot.
Release binaries are built by GitHub Actions and published with SHA-256
checksums.

Run the same audit locally:

```bash
apps/desktop/scripts/security-check.sh
```
