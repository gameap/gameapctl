package panel

const systemdUnitTemplate = `[Unit]
Description=GameAP - Game Server Control Panel
Documentation=https://docs.gameap.com
After=network.target
Wants=network-online.target
Requires=network.target

[Service]
Type=simple
User={{.User}}
Group={{.Group}}

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
StartLimitInterval=60
StartLimitBurst=3

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
