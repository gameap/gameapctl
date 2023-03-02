//go:build linux || darwin
// +build linux darwin

package actions

const defaultWebInstallationPath = "/var/www/gameap"
const defaultWorkPath = "/srv/gameap"

// Daemon.
const defaultDaemonCertPath = "/etc/gameap-daemon/certs"
const defaultToolsPath = "/srv/gameap"
const defaultSteamCMDPath = "/srv/gameap/steamcmd"
const defaultDaemonFilePath = "/usr/bin/gameap-daemon"
const defaultDaemonConfigFilePath = "/etc/gameap-daemon/gameap-daemon.yaml"
const defaultOutputLogPath = "/var/log/gameap-daemon/output.log"
