//go:build linux || darwin

package gameap

const DefaultWebInstallationPath = "/var/www/gameap"
const DefaultWorkPath = "/srv/gameap"

// GameAP v4.

const DefaultConfigFilePath = "/etc/gameap/config.env"
const DefaultDataPath = "/var/lib/gameap"
const DefaultBinaryPath = "/usr/bin/gameap"

// Daemon.

const DefaultDaemonCertPath = "/etc/gameap-daemon/certs"
const DefaultToolsPath = "/srv/gameap"
const DefaultSteamCMDPath = "/srv/gameap/steamcmd"
const DefaultDaemonFilePath = "/usr/bin/gameap-daemon"
const DefaultDaemonConfigFilePath = "/etc/gameap-daemon/gameap-daemon.yaml"
const DefaultOutputLogPath = "/var/log/gameap-daemon/output.log"
