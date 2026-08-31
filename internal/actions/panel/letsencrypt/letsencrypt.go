package letsencrypt

import (
	"path/filepath"
	"runtime"
)

const (
	defaultConfigDirUnix    = "/etc/gameap"
	defaultConfigDirWindows = "C:\\gameap\\web"
	configFileName          = "config.env"
)

// envKeysOwned lists every config.env key that the letsencrypt subcommand
// rewrites. Anything not in this list is preserved verbatim. DNS-provider-
// specific credentials that the operator supplies live alongside these and
// are merged in at write time.
var envKeysOwned = []string{
	"ACME_ENABLED",
	"ACME_CHALLENGE_TYPE",
	"ACME_EMAIL",
	"ACME_DOMAINS",
	"ACME_DIRECTORY_URL",
	"ACME_DNS_PROVIDER",
	"ACME_RENEWAL_THRESHOLD",
	"ACME_RENEWAL_CHECK_INTERVAL",
	"ACME_PROPAGATION_TIMEOUT",
	"ACME_STORAGE_PATH",
}

const (
	ChallengeHTTP01 = "http-01"
	ChallengeDNS01  = "dns-01"
)

func ConfigPath() string {
	var dir string
	if runtime.GOOS == "windows" {
		dir = defaultConfigDirWindows
	} else {
		dir = defaultConfigDirUnix
	}

	return filepath.Join(dir, configFileName)
}
