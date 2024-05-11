package packagemanager

import osinfo "github.com/gameap/gameapctl/pkg/os_info"

const ApachePackage = "apache2"
const ComposerPackage = "composer"
const CurlPackage = "curl"
const GitPackage = "git"
const GnuPGPackage = "gnupg"
const Lib32GCCPackage = "lib32gcc"
const Lib32Stdc6Package = "lib32stdc++6"
const Lib32z1Package = "lib32z1"
const MariaDBServerPackage = "mariadb-server"
const MySQLServerPackage = "mysql-server"
const NPMPackage = "npm"
const NginxPackage = "nginx"
const NodeJSPackage = "nodejs"
const PHPExtensionsPackage = "php-extensions"
const PHPPackage = "php"
const TarPackage = "tar"
const TmuxPackage = "tmux"
const UnzipPackage = "unzip"
const XZUtilsPackage = "xz-utils"

const DistributionDefault = Default
const DistributionDebian = osinfo.DistributionDebian
const DistributionUbuntu = osinfo.DistributionUbuntu
const DistributionCentOS = osinfo.DistributionCentOS
const DistributionAmazon = osinfo.DistributionAmazon
const DistributionAlmaLinux = osinfo.DistributionAlmaLinux
const DistributionWindows = osinfo.DistributionWindows

const CodeNameDefault = Default

const ArchDefault = Default
const ArchAMD64 = "amd64"
const ArchARM64 = "arm64"

const Default = "default"

const packageMarkFile = ".gameap-package"
