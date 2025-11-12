# Uninstall GameAP Panel

## Linux

* Check databaseWasInstalled in ~/.gameapctl/panel_install_state.json, 
  if true - call removing package for used 
* Stop gameap services:
  ```
    systemctl stop gameap
    systemctl disable gameap
    rm /etc/systemd/system/gameap.service
  ```
* Stop gameap-daemon service (if installed):
  ```
    systemctl stop gameap-daemon
    systemctl disable gameap-daemon
    rm /etc/systemd/system/gameap-daemon.service
  ```
* Remove gameap files and directories:
  * Remove config directory, default /etc/gameap 
  * Remove data directory, default /var/lib/gameap
  * Remove binary files, default /usr/bin/gameap

## Windows
* Check databaseWasInstalled in ~/.gameapctl/panel_install_state.json, 
  if true - remove database

    ```
    net stop MariaDB
    wmic product where "name like '%MariaDB%'" get name,identifyingnumber
    msiexec /x {IDENTIFYING_NUMBER} /qn
    ```

* Remove GameAP service (for version 4)
    ```
    sc stop "GameAP
    sc delete "GameAP"
    ```

* Remove panel service

    ```
    sc stop "GameAP Daemon"
    sc delete "GameAP Daemon"
    ```

* Remove gameap files and directories:
    * Remove binary files C:\gameap\web\gameap.exe
    * Remove default data and config path C:\gameap\web

* Remove PHP
  * Remove php-fpm service
    ```
    sc stop php-fpm
    sc delete php-fpm
    ```
  * Remove PHP directory C:\php
  
* Remove nginx service

    ```
    sc stop nginx
    sc delete nginx
    ```
  
* Remove root gameap directory C:\gameap