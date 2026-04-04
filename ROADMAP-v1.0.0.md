# GameAPCTL v1.0.0 Roadmap

This document outlines the requirements and milestones for the stable v1.0.0 release.

## 1. Security

### 1.1. SQL Injection in Database Setup (Critical)

SQL queries in MySQL/PostgreSQL initialization use string concatenation:

- `internal/actions/panel/install/mysql.go` (lines 162, 172, 208-220, 288-299)
- `internal/actions/panel/install/pgsql.go`

**Required:** Use parameterized queries or proper escaping for all user-supplied values (username, password, database name).

### 1.2. Credential Exposure in send-logs (Critical)

`panel_install_state.json` stores `DBPassword`, `DBRootPassword`, and `AdminPassword` in plaintext. The `send-logs` command (`internal/actions/sendlogs/send_logs.go:223-248`) transmits them without redaction.

**Required:** Redact sensitive fields before sending logs.

### 1.3. Self-update Without Verification (High)

`internal/actions/selfupdate/self_update.go` applies downloaded binaries without checksum or signature validation.

**Required:** Verify SHA256 checksum from the GitHub release. Optionally support GPG signature verification.

## 2. Package Manager

### 2.1. Move Hardcoded Logic to YAML Configurations

Currently hardcoded in Go source:

- PHP versions and extensions in `pkg/package_manager/apt.go:653-762` (16 packages, versions 7.3-8.4)
- Windows `php-extensions` preprocessor in `pkg/package_manager/windows.go:1355-1468`
- Repository definitions (ondrej/php PPA, sury.org, remi) embedded in code

**Required:** Declarative YAML configuration for all platforms covering package lists, repositories, extensions, and version selection.

### 2.2. Extend YAML Configuration Schema

The current schema does not support:

- Version pinning
- Declarative repository/PPA definitions
- PHP extension configuration
- Conditional version selection (e.g., if php >= 8.2 use package X)
- Post-install verification (confirm the package actually installed)

### 2.3. Improve Error Handling

- Linux managers silently skip unknown packages
- Windows manager logs but continues on unknown packages (`windows.go:443`)
- No dependency validation before installation
- No rollback mechanism on installation failure

**Required:** Explicit errors for missing packages, pre-install dependency validation, and at minimum logging of partial installation state for manual recovery.

### 2.4. Additional Package Manager Backends

Consider adding support for:

- **pacman** (Arch Linux)
- **zypper** (openSUSE/SLES)
- **brew** (macOS, for development environments)
- **chocolatey/winget** (Windows, as an alternative to manual download-and-install)

## 3. Cross-Platform Stability

### 3.1. Windows

- `installWinSWService` and `installServyService` exist but are unused — remove or integrate
- Only Shawl service wrapper is active, no mechanism to select alternatives
- Firewall rule duplicate detection is insufficient

### 3.2. ARM64 Support

- Chroot fallback is AMD64-only
- ARM64 YAML configs exist only for APT (`default_arm64.yaml`)
- No ARM64 support in Windows or DNF/YUM configurations

### 3.3. macOS

Detected in `os_info` but no package manager implementation exists. Either add support (via brew) or explicitly exclude with a clear error message.

### 3.4. Supported Environments for v1.0.0

**Tier 1** (full support, CI coverage):

- Debian 12, 13
- Ubuntu 22.04, 24.04
- CentOS Stream 9, 10
- AlmaLinux 9, Rocky Linux 9
- Windows Server 2022, 2025
- Windows 10, 11

**Tier 2** (best-effort):

- Fedora (latest)
- Amazon Linux 2023
- Older Debian/Ubuntu via chroot fallback

## 4. Reliability and Error Handling

### 4.1. Network Operations

- `release_finder.go:92` uses `http.Get()` without context or timeout
- Most HTTP calls lack timeouts (except daemon setup)
- No retry logic for package downloads (retry exists only in release finder)
- No checksum validation for downloaded files

**Required:** Context-aware HTTP clients with configurable timeouts, retry with backoff for all downloads, checksum verification where checksums are available.

### 4.2. Swallowed Errors

15+ locations where errors are suppressed via `_ =` or only logged:

- `send_logs.go` — file close/write errors
- `daemon_install.go` — key file close errors
- Multiple `defer f.Close()` without error checking

**Required:** Audit all suppressed errors. Propagate where meaningful, document where intentionally ignored.

### 4.3. Race Conditions

Check-then-use patterns without atomic operations (e.g., check file exists then create directory).

### 4.4. Panic in Production Code

`pkg/package_manager/env_path_windows.go:23` calls `panic()` on package load failure.

**Required:** Replace with proper error propagation.

## 5. Testing

### 5.1. Current Coverage: ~8.6% (13 of 152 Go files)

Priority areas to cover:

| Component | Priority | Current State |
|-----------|----------|---------------|
| Panel install v3/v4 | Critical | No tests |
| Daemon install flow | Critical | Config parsing only |
| Windows package manager | High | Minimal unit tests |
| Self-update | High | No tests |
| Database setup (MySQL/PG) | High | No tests |
| send-logs | Medium | No tests |
| Service management | Medium | Windows only |
| Network utilities | Medium | Validation only |

### 5.2. Integration Tests

- Add CI for CentOS/AlmaLinux/Rocky (currently Debian + Ubuntu only)
- Add database tests on Windows
- Add full lifecycle smoke tests: install -> start -> status -> stop -> uninstall

### 5.3. Target: minimum 50% coverage of critical paths for v1.0.0

## 6. Use Cases

### 6.1. Core Scenarios (must work reliably)

- **Fresh install:** `gameapctl panel install` on a clean server — zero to working panel
- **Upgrade:** `gameapctl panel upgrade` — update without data loss
- **Daemon deploy:** `gameapctl daemon install --host X --token Y` — connect to existing panel
- **Uninstall:** `gameapctl panel uninstall --with-daemon --with-data` — full cleanup
- **Self-update:** `gameapctl self-update` — update the tool itself
- **Diagnostics:** `gameapctl send-logs` — send logs to support

### 6.2. Missing Scenarios to Consider

- **Database migration:** switch between SQLite, PostgreSQL, and MySQL
- **Backup/Restore:** back up panel configuration and data
- **Health check:** `gameapctl panel check` / `gameapctl daemon check` — verify all components
- **Config management:** `gameapctl panel config set/get` — manage configuration without manual file editing
- **Multi-daemon:** manage multiple daemons from a single panel

### 6.3. UX Improvements

- Progress indicators for long-running operations (download, installation)
- Dry-run mode: `--dry-run` — show what would be installed without executing
- Verbose/quiet modes for different levels of detail
- Colored output with `--no-color` opt-out

## 7. Code Quality

- **65 nolint directives** — review each one, replace with a fix or a documented justification
- **`windows.go` is 1659 lines** — split into logical modules (service management, download, PHP setup, etc.)
- **Panel install v3/v4 are 37-38KB each** — extract shared logic

## 8. Release Plan

| Phase | Scope | Version |
|-------|-------|---------|
| 1 | Security fixes (SQL injection, send-logs redaction, self-update checksum) | 0.x.y |
| 2 | Package manager: YAML declarations instead of hardcoded logic, error handling | 0.x.y |
| 3 | Tests for critical paths (panel/daemon install, package manager) | 0.x.y |
| 4 | Panic to error, swallowed errors, timeouts, retry | 0.x.y |
| 5 | New use cases (health check, dry-run, backup) | RC |
| 6 | Stabilization, documentation, final audit | **1.0.0** |