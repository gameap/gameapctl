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

		if !strings.HasPrefix(arg, "-") || !secretArgPattern.MatchString(arg) {
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
