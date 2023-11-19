# GameAP Control

Utility for managing [GameAP](https://gameap.ru), [GameAP Daemon](https://github.com/gameap/daemon) and other parts of this.

You can use gameapctl to install, upgrade, inspect and manage GameAP, and view logs.

gameapctl is available for Linux, macOS and Windows.

## Supported OS

Autotests were performed on the following operating systems. 
Other operating systems may work as well, if they can run the required dependencies.

### Debian

| Version       | Supported | Notes                                   |
|---------------|-----------|-----------------------------------------|
| 12 (bookworm) | ✔         | Latest manual test (v0.4.1): 12.11.2023 |
| 11 (bullseye) | ✔         | Latest manual test (v0.4.3): 13.11.2023 |
| 10 (buster)   | ✔         |                                         |
| 9 (stretch)   | ✔         |                                         | 

### Ubuntu

| Version | Supported | Notes                                                              |
|---------|----------|--------------------------------------------------------------------|
| 22.04   | ✔        | Latest manual test (v0.4.1): 12.11.2023                            |
| 20.04   | ✔        | Latest manual test (v0.5.1): 16.11.2023                            |
| 18.04   | ✔        | Latest manual test (v0.5.0): 16.11.2023, used chrooted php package |
| 16.04   | ✔        | Latest manual test (v0.5.6): 16.11.2023, used chrooted php package |

### CentOS

| Version  | Supported | Notes                                  |
|----------|----------|----------------------------------------|
| Stream 9 | ✔        | Lates manual test (v0.6.1): 17.11.2023 |
| Stream 8 | ✔        | Lates manual test (v0.6.2): 17.11.2023 |
| 7        | ✔        | Lates manual test (v0.6.2): 17.11.2023 |