//go:build linux || darwin
// +build linux darwin

package gameap

const DefaultWebInstallationPath = "/var/www/gameap"
const DefaultWorkPath = "/srv/gameap"

// Daemon.
const DefaultDaemonCertPath = "/etc/gameap-daemon/certs"
const DefaultToolsPath = "/srv/gameap"
const DefaultSteamCMDPath = "/srv/gameap/steamcmd"
const DefaultDaemonFilePath = "/usr/bin/gameap-daemon"
const DefaultDaemonConfigFilePath = "/etc/gameap-daemon/gameap-daemon.yaml"
const DefaultOutputLogPath = "/var/log/gameap-daemon/output.log"
