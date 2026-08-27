package gameap

const (
	// DefaultPanelPort is the panel HTTP port in system scope.
	DefaultPanelPort = "80"

	// DefaultUserScopePanelPort is the panel HTTP port in user scope: an unprivileged
	// process cannot bind port 80 and there is no way to grant it CAP_NET_BIND_SERVICE.
	DefaultUserScopePanelPort = "8025"
)
