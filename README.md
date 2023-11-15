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

| Version     | Supported | Notes                                                                                           |
|-------------|----------|-------------------------------------------------------------------------------------------------|
| 22.04       | ✔        | Latest manual test (v0.4.1): 12.11.2023                                                         |
| 20.04       | ✔        |                                                                                                 |
| 18.04       | ✖        | Latest manual test (v0.4.5): 14.11.2023, web panel installation is not supported due to php 7.2 |
