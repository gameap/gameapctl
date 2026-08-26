package app

import (
	"regexp"
	"strings"
)

const maskedValue = "***"

var secretArgPattern = regexp.MustCompile(`(?i)(password|secret|token|key)`)

// maskedArgs hides the values of the secret flags, so the command line can be
// written into the log file that is sent to GameAP support.
func maskedArgs(args []string) []string {
	masked := make([]string, 0, len(args))
	maskNext := false

	for _, arg := range args {
		if maskNext {
			masked = append(masked, maskedValue)
			maskNext = false

			continue
		}

		if !strings.HasPrefix(arg, "-") || !isSecretArg(arg) {
			masked = append(masked, arg)

			continue
		}

		name, _, found := strings.Cut(arg, "=")
		if found {
			masked = append(masked, name+"="+maskedValue)

			continue
		}

		masked = append(masked, arg)
		maskNext = true
	}

	return masked
}

// isSecretArg reports whether the flag value must be hidden. Besides the flags
// matching the secret pattern, --connect carries the setup key in the URL and
// --env carries DNS provider credentials.
func isSecretArg(arg string) bool {
	if secretArgPattern.MatchString(arg) {
		return true
	}

	name, _, _ := strings.Cut(arg, "=")

	switch strings.TrimLeft(name, "-") {
	case "connect", "env":
		return true
	}

	return false
}
