package shellquote

import (
	"runtime"
	"strings"

	"github.com/gopherclass/go-shellquote"
)

func Split(input string) (words []string, err error) {
	// Escape backslashes on Windows
	// Without it shellquote.Split will split command without backslashes
	// C:\gameap\steamcmd\steamcmd.exe -> ["C:gameapsteamcmdsteamcmd.exe"]
	// Should be ["C:\\gameap\\steamcmd\\steamcmd.exe"]
	if runtime.GOOS == "windows" {
		input = strings.ReplaceAll(input, "\\", "\\\\")
	}

	return shellquote.Split(input)
}
