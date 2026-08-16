# GameAP Control

Utility for managing [GameAP](https://gameap.ru), [GameAP Daemon](https://github.com/gameap/daemon) and other parts of this.

You can use gameapctl to install, upgrade, inspect and manage GameAP, and view logs.

gameapctl is available for Linux, macOS and Windows.

## Rootless installation (user scope)

On Linux both the panel and the daemon can be installed without root, into the current
user's home directory, managed by systemd user units:

```bash
gameapctl panel install --scope=user --host=<host> --database=sqlite
gameapctl daemon install --scope=user --connect=grpc://<host>:31718/<setup-key>
```

The scope is recorded at install time, so the other commands (`start`, `stop`, `restart`,
`status`, `upgrade`, `uninstall`, `change-password`, `letsencrypt`) pick it up automatically.
Pass `--scope=user` explicitly if the state file in `~/.gameapctl` was lost.

### Requirements

* Linux with systemd.
* A real login session, so that `systemctl --user` can reach the user bus: connect with
  `ssh user@host` or `machinectl shell user@`, not with `su` or `sudo -u`.
* Lingering, so that the services survive logout and start at boot. The installer only
  attempts to enable it and warns when that is denied (common over SSH, where polkit may
  refuse it). Check with `loginctl show-user $USER --property=Linger`; if it prints
  `Linger=no`, run `sudo loginctl enable-linger $USER`.

### File layout

| | system scope | user scope |
|---|---|---|
| Panel config | `/etc/gameap/config.env` | `~/.config/gameap/config.env` |
| Panel data | `/var/lib/gameap` | `~/.local/share/gameap` |
| Panel binary | `/usr/bin/gameap` | `~/.local/bin/gameap` |
| Panel unit | `/etc/systemd/system/gameap.service` | `~/.config/systemd/user/gameap.service` |
| Daemon config | `/etc/gameap-daemon/gameap-daemon.yaml` | `~/.config/gameap-daemon/gameap-daemon.yaml` |
| Daemon work dir | `/srv/gameap` | `~/gameap` |
| Daemon binary | `/usr/bin/gameap-daemon` | `~/.local/bin/gameap-daemon` |
| Daemon unit | `/etc/systemd/system/gameap-daemon.service` | `~/.config/systemd/user/gameap-daemon.service` |

The daemon work dir can be changed at install time with `gameapctl daemon install --work-path=<dir>`
(an absolute path; defaults to `/srv/gameap` in system scope, `~/gameap` in user scope,
`C:\gameap` on Windows).

Note that `~/.local/bin` is frequently missing from `PATH` in non-login shells. The services
are unaffected because the units use absolute paths, but to run `gameap` by name add it:
`export PATH="$HOME/.local/bin:$PATH"`.

### Limitations

| Limitation | Reason |
|---|---|
| Ports below 1024 (80, 443) are normally unavailable; the panel defaults to 8025 | A systemd user unit cannot be granted `CAP_NET_BIND_SERVICE`. The installer probes the port rather than rejecting anything below 1024, so low ports still work where an administrator lowered `net.ipv4.ip_unprivileged_port_start` |
| No database server is installed; SQLite is the default | `apt`/`dnf` and system services require root. `--database=mysql\|postgres` is only accepted for an existing server, described by `--database-host`, `--database-name`, `--database-username` and `--database-password` (plus `--database-port` for a non-default port) |
| System packages are not installed for the panel | It needs none of them: downloads, archive extraction, SQLite and password hashing are all in-process. Building with `--github` still needs `git`, `go` and `npm` preinstalled |
| The daemon has prerequisites of its own | `curl` and `gpg` must be preinstalled (plus `tmux` or `docker` if the process manager is overridden to one of them). SteamCMD additionally needs the 32-bit libraries `lib32gcc`, `lib32stdc++6` and `lib32z1` on a 64-bit system; the installer only warns about all of these |
| Let's Encrypt `http-01` is unavailable with `--scope=user`; use `--challenge=dns-01` | The challenge requires port 80; a system-scope install can use `http-01` when port 80 is publicly reachable |
| The `gameap` system user and group are not created | Everything runs as the current user |

## Supported OS

Autotests were performed on the following operating systems. 
Other operating systems may work as well, if they can run the required dependencies.

### Windows

| Version     | Supported | Notes                                    |
|-------------|-----------|------------------------------------------|
| Server 2025 | ✔         | Latest manual test (v0.20.4): 26.11.2024 |
| Server 2022 | ✔         | Latest manual test (v0.9.1): 02.03.2024  |
| Server 2019 | ✔         | Latest manual test (v0.9.3): 02.03.2024  |
| Server 2016 | ✔         | Latest manual test (v0.9.3): 10.03.2024  |
| 11          | ✔         | Latest manual test (v0.20.4): 26.11.2025 |
| 10          | ✔         | Latest manual test (v0.10.0): 26.05.2024 |

### Debian

| Version       | Supported | Notes                                    |
|---------------|-----------|------------------------------------------|
| 13 (trixie)   | ✔         |                                          |
| 12 (bookworm) | ✔         | Latest manual test (v0.4.1): 12.11.2023  |
| 11 (bullseye) | ✔         | Latest manual test (v0.4.3): 13.11.2023  |
| 10 (buster)   | ✔         | Latest manual test (v0.10.0): 25.05.2024 |
| 9 (stretch)   | ✔         |                                          | 

### Ubuntu

| Version | Supported | Notes                                                              |
|---------|-----------|--------------------------------------------------------------------|
| 24.04   | ✔         | Latest manual test (v0.10.0): 15.05.2024                           |
| 22.04   | ✔         | Latest manual test (v0.4.1): 12.11.2023                            |
| 20.04   | ✔         | Latest manual test (v0.5.1): 16.11.2023                            |
| 18.04   | ✔         | Latest manual test (v0.5.0): 16.11.2023, used chrooted php package |
| 16.04   | ✔         | Latest manual test (v0.5.6): 16.11.2023, used chrooted php package |

### CentOS

| Version   | Supported | Notes                                    |
|-----------|-----------|------------------------------------------|
| Stream 10 | ✔         | Latest manual test (v0.10.4): 06.11.2025 |
| Stream 9  | ✔         | Latest manual test (v0.6.1): 17.11.2023  |
| Stream 8  | ✔         | Latest manual test (v0.6.2): 17.11.2023  |
| 7         | ✔         | Latest manual test (v0.6.2): 17.11.2023  |

### AlmaLinux

| Version | Supported | Notes                                    Z |
|---------|-----------|--------------------------------------------|
| 9       | ✔         | Latest manual test (v0.6.10): 12.02.2024   |

### Amazon Linux

| Version | Supported | Notes                                                                                                                                      |
|---------|-----------|--------------------------------------------------------------------------------------------------------------------------------------------|
| 2023    | ⚠️        | Latest manual test (v0.7.1): 12.02.2024<br/>Web part tested with SQLite Database<br/>Amazon Linux 2023 no longer ships any i686 user space |

### Rocky Linux

| Version | Supported | Notes                                                                                                                                    |
|---------|-----------|------------------------------------------------------------------------------------------------------------------------------------------|
| 9.3     | ⚠️        | Latest manual test (v0.10.0): 15.05.2024<br/>Web part tested with MySQL Database<br/>Rocky Linux 9.3 no longer ships any i686 user space |


### Fedora

| Version | Supported | Notes                                    |
|---------|-----------|------------------------------------------|
| 43      | ✔         | Latest manual test (v0.20.6): 26.11.2024 |