# Pacman Package Manager Configuration

This directory contains package configuration files for the pacman package manager used on the Arch family of
distributions (Arch Linux, CachyOS, Manjaro, EndeavourOS).

Routing: Arch Linux is detected via `osinfo.Distribution == "arch"`; other Arch derivatives (CachyOS, Manjaro,
EndeavourOS) fall through to detection by the presence of the `pacman` binary. In both cases the `pacman`
configuration in this directory is used.

## Configuration Files Structure

### `default.yaml`
Main configuration file for pacman package manager settings.
It includes package aliases and replacement definitions that apply across all Arch-based distributions.

### `default_{architecture}.yaml`

Architecture-specific overrides for the default configuration.

Replace `architecture` with the system architecture (e.g., `amd64`, `arm64`).
These files provide additional customization for package management based on the system architecture.
Values in these files override those in `default.yaml` when applicable.

For example:
- `default_amd64.yaml` for AMD64 architecture
- `default_arm64.yaml` for ARM64 architecture

### `{distname}.yaml`

Files are specific to each supported distribution.

Replace `distname` with the name of the distribution (e.g., `arch`, `cachyos`, `manjaro`, `endeavouros`).
These files allow for customization of package management settings based on the specific OS.
Values in these files override those in `default.yaml` when applicable.

For example:
- `arch.yaml` for Arch Linux
- `cachyos.yaml` for CachyOS

### `{distname}_{distversion}_{architecture}.yaml`

Architecture-specific configurations for specific distributions.

Replace `architecture` with the system architecture (e.g., `amd64`, `arm64`).
These files provide further customization for package management based on the system architecture.
Values in these files override those in both `default.yaml` and `{distname}.yaml` when applicable.

For example:
- `arch_amd64.yaml` for Arch Linux on AMD64 architecture

## Notes

- The `lib32-*` packages (e.g. `lib32-gcc-libs`, `lib32-zlib`) live in the `multilib` repository, which is
  disabled by default on Arch. Their `pre-install` steps enable the `[multilib]` section in
  `/etc/pacman.conf` idempotently and refresh the database (`pacman -Sy`).
- `redis-server` maps to `valkey`: Redis was removed from the official Arch repositories in April 2025 and
  replaced by the drop-in-compatible `valkey` (service name `valkey`).
- `docker`, `go` and `nodejs` use native Arch packages instead of the upstream install scripts/tarballs used
  on Debian/RHEL (the Docker convenience script does not support Arch; Arch's `go`/`nodejs` are current).
