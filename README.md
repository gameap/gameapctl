# GameAP Control

Utility for managing [GameAP](https://gameap.ru), [GameAP Daemon](https://github.com/gameap/daemon) and other parts of this.

You can use gameapctl to install, upgrade, inspect and manage GameAP, and view logs.

gameapctl is available for Linux, macOS and Windows.

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