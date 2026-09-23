//go:build windows

package packagemanager

import (
	"context"
	"os"

	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
)

const servicesLogsPath = servicesConfigPath + `\logs`

// ProtectWorkPath creates C:\gameap when it is missing and restricts it, so that only SYSTEM
// and Administrators can change what is stored there. The services run the executables from
// it, the daemon as LocalSystem, and a directory in the root of a drive inherits an ACL that
// lets every local user modify the files created in it.
//
// Shawl, the service wrapper, creates the log directory of its service in the services log
// directory and exits when it cannot. For the services running as NetworkService that worked
// only thanks to the inherited ACL, so NetworkService is allowed to modify that directory.
func ProtectWorkPath(ctx context.Context) error {
	if err := os.MkdirAll(gameap.DefaultWorkPath, 0755); err != nil {
		return errors.Wrapf(err, "failed to create %s", gameap.DefaultWorkPath)
	}

	if err := oscore.RestrictDirectoryAccess(ctx, gameap.DefaultWorkPath); err != nil {
		return err
	}

	if err := os.MkdirAll(servicesLogsPath, 0755); err != nil {
		return errors.Wrapf(err, "failed to create %s", servicesLogsPath)
	}

	return oscore.GrantModify(ctx, servicesLogsPath, defaultServiceUser)
}
