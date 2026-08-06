//go:build linux || darwin

package panel

import (
	"bytes"
	"text/template"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/pkg/errors"
)

type SystemdUnitConfig struct {
	Scope            string
	User             string
	Group            string
	WorkingDirectory string
	ExecStart        string
	EnvironmentFile  string
	ReadWritePaths   string
}

func renderSystemdUnit(data SystemdUnitConfig) ([]byte, error) {
	unitTemplate := systemdUnitTemplate
	if data.Scope == gameap.ScopeUser {
		unitTemplate = systemdUserUnitTemplate
	}

	tmpl, err := template.New("systemd.unit").Parse(unitTemplate)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse systemd unit template")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, errors.WithMessage(err, "failed to execute systemd unit template")
	}

	return buf.Bytes(), nil
}

const systemdUnitTemplate = `[Unit]
Description=GameAP - Game Server Control Panel
Documentation=https://docs.gameap.com
After=network.target network-online.target postgresql.service mysql.service mariadb.service
Wants=network-online.target
Requires=network.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
User={{.User}}
Group={{.Group}}

# Working directory
WorkingDirectory={{.WorkingDirectory}}

ExecStart={{.ExecStart}}

# Allow binding to privileged ports
AmbientCapabilities=CAP_NET_BIND_SERVICE

# Graceful stop
ExecStop=/bin/kill -TERM $MAINPID
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30

# Restart policy
Restart=always
RestartSec=5

# Environment configuration
EnvironmentFile={{.EnvironmentFile}}

RuntimeDirectory=gameap
PIDFile=/run/gameap/gameap.pid

# Filesystem permissions
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths={{.ReadWritePaths}}

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gameap

[Install]
WantedBy=multi-user.target
`

// A systemd --user manager rejects User=/Group=, cannot raise ambient capabilities,
// and does not know network.target. ProtectHome would hide the binary, the config and
// the data, all of which live under $HOME; the remaining sandboxing directives need a
// mount namespace that is unavailable wherever unprivileged user namespaces are off.
const systemdUserUnitTemplate = `[Unit]
Description=GameAP - Game Server Control Panel
Documentation=https://docs.gameap.com

[Service]
Type=simple

# Working directory
WorkingDirectory={{.WorkingDirectory}}

ExecStart={{.ExecStart}}

# Graceful stop
ExecStop=/bin/kill -TERM $MAINPID
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30

# Restart policy
Restart=always
RestartSec=5

# Environment configuration
EnvironmentFile={{.EnvironmentFile}}

RuntimeDirectory=gameap

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gameap

[Install]
WantedBy=default.target
`
