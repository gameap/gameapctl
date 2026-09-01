package letsencrypt

import (
	"path/filepath"
	"runtime"

	"github.com/gameap/gameapctl/pkg/configenv"
	"github.com/gameap/gameapctl/pkg/panel"
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
	panel.ACMEEnabledKey,
	panel.ACMEChallengeTypeKey,
	panel.ACMEEmailKey,
	panel.ACMEDomainsKey,
	"ACME_DIRECTORY_URL",
	panel.ACMEDNSProviderKey,
	"ACME_RENEWAL_THRESHOLD",
	"ACME_RENEWAL_CHECK_INTERVAL",
	"ACME_PROPAGATION_TIMEOUT",
	"ACME_STORAGE_PATH",
}

const (
	ChallengeHTTP01 = panel.ACMEChallengeHTTP01
	ChallengeDNS01  = panel.ACMEChallengeDNS01
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

// DisableUpdates builds the config.env changes that switch ACME off: the flag is
// kept and set to false so that the file still documents the decision, every
// other key the subcommand owns is removed.
func DisableUpdates() map[string]string {
	updates := map[string]string{
		panel.ACMEEnabledKey: "false",
	}

	for _, key := range envKeysOwned {
		if key == panel.ACMEEnabledKey {
			continue
		}

		updates[key] = configenv.RemoveMarker
	}

	return updates
}
