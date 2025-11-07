# APT Package Manager Configuration

This directory contains package configuration files for the APT package manager used on Debian-based distributions
(Debian, Ubuntu).

## Configuration Files Structure

### `default.yaml`
Main configuration file for APT package manager settings.
It includes package aliases and replacement definitions that apply across all Debian-based distributions.

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

Replace `distname` with the name of the distribution (e.g., `debian`, `ubuntu`).
These files allow for customization of package management settings based on the specific OS.
Values in these files override those in `default.yaml` when applicable.

For example:
- `debian.yaml` for Debian
- `ubuntu.yaml` for Ubuntu

### `{distname}_{codename}.yaml`

Files are specific to each supported distribution and version.

Replace `distname` with the name of the distribution (e.g., `debian`, `ubuntu`)
and `codename` with the version codename (e.g., `bookworm`, `jammy`, `focal`).
These files allow for customization of package management settings based on the specific OS and version.
Values in these files override those in `default.yaml` and `{distname}.yaml` when applicable.

For example:
- `debian_bookworm.yaml` for Debian Bookworm
- `debian_bullseye.yaml` for Debian Bullseye
- `ubuntu_jammy.yaml` for Ubuntu 22.04 (Jammy Jellyfish)
- `ubuntu_focal.yaml` for Ubuntu 20.04 (Focal Fossa)

### `{distname}_{codename}_{architecture}.yaml`

Architecture-specific configurations for specific distributions and versions.

Replace `architecture` with the system architecture (e.g., `amd64`, `arm64`).
These files provide further customization for package management based on the system architecture.
Values in these files override those in `default.yaml`, `{distname}.yaml`, and `{distname}_{codename}.yaml` when applicable.

For example:
- `debian_bookworm_arm64.yaml` for Debian Bookworm on ARM64 architecture
- `ubuntu_focal_arm64.yaml` for Ubuntu 20.04 on ARM64 architecture

## Package Configuration Format

Each YAML file contains a list of package configurations with the following structure:

```yaml
packages:
  - name: package-name
    replace-with: [actual-package-1, actual-package-2]
    virtual-package: true  # Optional: marks this as a virtual package
    pre-install:           # Optional: commands to run before installation
      - command1
      - command2
    post-install:          # Optional: commands to run after installation
      - command1
      - command2
```

### Fields:

- `name`: The package name or alias to match
- `replace-with`: Array of actual package names to install instead
- `virtual-package`: (Optional) Boolean indicating if this is a virtual package
- `pre-install`: (Optional) Array of shell commands to execute before installation
- `post-install`: (Optional) Array of shell commands to execute after installation

## Examples

### Simple Package Replacement

```yaml
packages:
  - name: lib32gcc
    replace-with: [lib32gcc-s1]
```

### Virtual Package with Multiple Replacements

```yaml
packages:
  - name: php-extensions
    virtual-package: true
    replace-with: [php-bcmath, php-gd, php-mbstring, php-mysql, php-xml]
```

### Package with Post-Install Commands

```yaml
packages:
  - name: postgresql
    replace-with: [postgresql, postgresql-contrib]
    post-install:
      - systemctl enable postgresql
      - systemctl start postgresql
```

## Loading Order

Configuration files are loaded in the following order, with later files overriding earlier ones:

1. `default.yaml`
2. `default_{architecture}.yaml` (if architecture is specified)
3. `{distname}.yaml` (if distribution is specified)
4. `{distname}_{codename}.yaml` (if distribution and version are specified)
5. `{distname}_{codename}_{architecture}.yaml` (if all parameters are specified)

This hierarchical approach allows for:
- General defaults that apply to all systems
- Architecture-specific overrides
- Distribution-specific overrides
- Version-specific overrides
- Fine-grained control for specific distribution/version/architecture combinations
