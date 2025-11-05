# Uninstall GameAP Panel

## Windows
* Check databaseWasInstalled in ~/.gameapctl/panel_install_state.json, 
  if true - remove database

    ```
    net stop MariaDB
    wmic product where "name like '%MariaDB%'" get name,identifyingnumber
    msiexec /x {IDENTIFYING_NUMBER} /qn
    ```

* Remove panel service

    ```
    sc stop "GameAP Daemon"
    sc delete "GameAP Daemon"
    ```

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