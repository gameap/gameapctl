# DNF Package Manager Configuration

This directory contains package configuration files for the DNF package manager used on RHEL-based distributions 
(CentOS, AlmaLinux, Rocky Linux, Fedora).

## Configuration Files Structure

### `default.yaml` 
Main configuration file for DNF package manager settings. 
It includes parameters such as repository definitions, package lists, and installation options.

### `default_{architecture}.yaml` 

Architecture-specific overrides for the default configuration.

Replace `architecture` with the system architecture (e.g., `amd64`, `arm64`).
These files provide additional customization for package management based on the system architecture.
Values in these files override those in `default.yaml` when applicable.

For example:
- `default_amd64.yaml` for AMD64 architecture
- `default_arm64.yaml` for ARM64 architecture

### `{distname}_{distversion}.yaml` 

Files are specific to each supported distribution and version.

Replace `distname` with the name of the distribution (e.g., `centos`, `almalinux`, `rocky`, `fedora`) 
and `distversion` with the version number (e.g., `7`, `8`, `9`, `36`, `37`). 
These files allow for customization of package management settings based on the specific OS and version.
Values in these files override those in `default.yaml` when applicable.

For example:
- `centos_7.yaml` for CentOS 7
- `almalinux_8.yaml` for AlmaLinux 8
- `rocky_9.yaml` for Rocky Linux 9
- `fedora_36.yaml` for Fedora 36

### `{distname}_{distversion}_{architecture}.yaml` 

Architecture-specific configurations.

Replace `architecture` with the system architecture (e.g., `amd64`, `arm64`).
These files provide further customization for package management based on the system architecture.
Values in these files override those in both `default.yaml` and `{distname}_{distversion}.yaml` when applicable.

For example:
- `centos_7_amd64.yaml` for CentOS 7 on AMD64 architecture
- `almalinux_8_arm64.yaml` for AlmaLinux 8 on ARM64