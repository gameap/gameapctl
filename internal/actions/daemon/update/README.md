# Daemon Upgrade

Implements `gameapctl daemon upgrade` (aliases: `update`, `u`).

Two independent flows live here:

1. **Binary upgrade** — replace the `gameap-daemon` binary with a newer version
   (either a GitHub release or a build from source).
2. **Switch to gRPC** (`--switch-to-grpc`) — migrate an installed daemon from the
   legacy HTTP/binn protocol to the gRPC bidirectional stream protocol introduced
   in GameAP v4.2.

The two flows are mutually exclusive: when `--switch-to-grpc` is set, `Handle`
delegates to `HandleSwitchToGRPC` and returns immediately.

## Flags

| Flag | Type | Description |
|------|------|-------------|
| `--github` | bool | Build daemon from GitHub source instead of downloading a release. |
| `--branch` | string | GitHub branch to build (hidden flag, defaults to `master`). |
| `--version` | string | Specific release tag (e.g. `4.0.0`, `4.0.0beta1`). Empty = latest stable. |
| `--switch-to-grpc` | bool | Migrate config from legacy protocol to gRPC. |
| `--grpc-address` | string | Override gRPC server address. Default: derived from `api_host`, port `31718`. |

`--version` is mutually exclusive with `--github` and `--branch`.

## Binary upgrade

Two sub-flows: **release** and **GitHub source**. Both share the same
stop/backup/replace/start/rollback skeleton; they differ in how the new binary
is produced.

### State reuse

Before resolving flags, the upgrader loads `DaemonInstallState` (written during
`daemon install`). When `--version` is not set:

* If `--github` is not set but the previous install used GitHub, GitHub mode is
  reactivated.
* If `--branch` is empty, the previously used branch is reused.

This keeps `daemon upgrade` consistent with how the daemon was originally
installed without forcing the operator to re-supply the same flags.

### Release flow (default)

1. Resolve `gameap-daemon` binary path via `exec.LookPath`.
2. Query GitHub Releases API
   (`https://api.github.com/repos/gameap/daemon/releases`) for a matching tag,
   filtered by `runtime.GOOS` / `runtime.GOARCH`. If `--version` carries a
   pre-release suffix, pre-releases are allowed.
3. Download the release archive into a temp directory.
4. **Stop** the daemon and verify the process is gone.
5. **Backup** the current binary to `$TMPDIR/gameap-daemon-backup`.
6. **Apply** the new binary in place via `selfupdate.Apply`. On failure,
   revert from backup and abort.
7. **Start** the daemon and wait for the process to come up. If it doesn't,
   revert from backup and start the old version.
8. Persist the resolved tag back into `DaemonInstallState.Version`.

### GitHub source flow (`--github`)

1. Resolve `gameap-daemon` binary path (falls back to
   `gameap.DefaultDaemonFilePath` if not on `PATH`).
2. **Backup** existing binary if present.
3. **Stop** the daemon.
4. Load the OS package manager and call `SetupDaemonFromGithub`, which:
   * Installs `git` and `go` if missing.
   * Clones `gameap/daemon` at the requested branch into a temp dir.
   * Runs `go build` and writes the binary to `gameap.DefaultDaemonFilePath`.
5. On build failure, revert from backup and try to start the old binary.
6. **Start** the daemon. On failure, revert from backup and restart.

The GitHub flow does not update `DaemonInstallState.Version` because there is
no release tag.

## `--switch-to-grpc`

Migrates a registered daemon from the legacy HTTP/binn protocol to gRPC.

### When to use it

The legacy protocol uses HTTP polling and a TCP listener on the daemon side
(`listen_ip` / `listen_port`). gRPC mode replaces that with a single
bidirectional stream the daemon opens to the panel on
`<api_host>:31718`. Switching:

* Removes the inbound listener — the daemon no longer needs an open port.
* Reuses the existing daemon certificates (mTLS).
* Requires the panel side to have `GRPC_ENABLED=true` and the gRPC port open.

The daemon is briefly unavailable during the switch, so run during a
maintenance window.

### Flow

```
load config
  ├── grpc.enabled already true? → exit (idempotent)
  ├── derive grpc address from api_host (or use --grpc-address)
  ├── validate api_key, ds_id, certificate paths
preflight
  ├── TCP probe: dial gRPC address
  └── TLS probe: mTLS handshake (CA + client cert/key)
        └── on cert auth error → tell user to reinstall via grpc://
ensure daemon binary exists
backup config → <cfg>.bak.<ts>
mutate config
  ├── set  grpc.enabled = true, grpc.address = <addr>
  └── del  api_host, listen_ip, listen_port
restart daemon (stop + start + grace + process check)
verify panel revoked legacy credentials
  └── poll legacy GET /gdaemon_api/get_token (Bearer api_key)
       expecting HTTP 409 + body containing
       "HTTP API is disabled for this node"
       up to 10 attempts × 1s
on any failure → rollback (stop, restore config, start)
on success    → persist DaemonInstallState.GRPCEnabled = true
```

### Pre-flight checks

The two probes run **before** any config is touched. Their goal is to fail loud
when the panel is misconfigured (port closed, GRPC disabled, wrong panel),
rather than discovering the problem after the daemon has already been
restarted with a broken config.

* `tcpDial` (`CheckGRPCConnectivity`) — confirms the port is reachable.
* `tlsProbe` — performs a real mTLS handshake using the daemon's CA, client
  cert, and key. Detects `x509.UnknownAuthorityError` /
  `CertificateInvalidError` / `bad certificate` and produces a targeted error
  message recommending `gameapctl daemon install --connect=grpc://...`,
  because such errors typically mean the daemon was registered against a
  different panel.

### Address derivation

If `--grpc-address` is not provided:

* `api_host` is parsed (a missing scheme is treated as `http://`).
* The hostname is extracted and combined with `gameap.DefaultGRPCPort`
  (`31718`).
* Any path component on `api_host` is dropped.

Example: `https://panel.example.com/subpath` → `panel.example.com:31718`.

This mirrors the daemon's own `config.GRPCAddress()` logic so both sides agree
on the target.

### Verification

A successful TCP/TLS handshake doesn't prove the panel actually accepted the
daemon's gRPC registration. The post-restart verification step closes that gap
by asking the panel directly: "is the legacy HTTP API still issuing tokens for
this daemon?".

After gRPC registration the panel responds to the legacy token endpoint with
`409 Conflict` and a body containing `HTTP API is disabled for this node`.
Anything else (200, other 4xx, 5xx, transport failure) is treated as
"not yet revoked" and retried. After
`verificationPollMaxAttempts` (10) attempts spaced
`verificationPollInterval` (1s) apart, the switch is rolled back.

### Rollback

Any failure between the config mutation and the verification step triggers a
rollback:

1. Stop the daemon.
2. Restore the config from the timestamped backup
   (`<cfg>.bak.<ts>`).
3. Start the daemon with the legacy config.

If the rollback itself fails (restore or restart), the error is wrapped as
`CRITICAL:` and surfaced to the operator — manual intervention is required.

### State

On success, `DaemonInstallState.GRPCEnabled` is set to `true` so subsequent
`gameapctl` runs know the daemon is in gRPC mode. A missing state file is
non-fatal — the switch still completes; only the state record is skipped.

## Files

* `daemon_update.go` — binary upgrade flow (release + GitHub).
* `switch_to_grpc.go` — `--switch-to-grpc` flow with injected dependencies
  (`switchDeps`) so the orchestration is unit-testable without a real daemon
  or panel.
* `switch_to_grpc_test.go` — tests for the switch flow, including pre-flight
  failures, rollback paths, and the legacy revocation HTTP contract.
