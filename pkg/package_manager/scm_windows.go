//go:build windows

package packagemanager

import (
	"log"
	"math"
	"strings"
	"time"

	"github.com/gameap/gameapctl/pkg/oscore"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

// windowsServiceConfig describes a service to register in the Windows service control manager.
type windowsServiceConfig struct {
	Name            string
	ExecutablePath  string
	Arguments       []string
	Account         string
	Password        string
	RecoveryActions []mgr.RecoveryAction
	ResetPeriod     time.Duration
}

// createWindowsService registers a new auto start service.
//
// The service control manager API is used instead of the sc command because sc passes the
// account name to the very same API but reports failures as localized text with an exit code,
// and because the API escapes the binary path and its arguments on its own.
func createWindowsService(cfg windowsServiceConfig) error {
	manager, err := mgr.Connect()
	if err != nil {
		return errors.Wrap(err, "failed to connect to service control manager")
	}
	defer func() {
		_ = manager.Disconnect()
	}()

	account := oscore.NormalizeWindowsServiceAccount(cfg.Account)

	log.Printf(
		"Creating service '%s' for account '%s'\n%s %s\n",
		cfg.Name, account, cfg.ExecutablePath, strings.Join(cfg.Arguments, " "),
	)

	svc, err := manager.CreateService(cfg.Name, cfg.ExecutablePath, mgr.Config{
		DisplayName:      cfg.Name,
		StartType:        mgr.StartAutomatic,
		ErrorControl:     mgr.ErrorNormal,
		ServiceStartName: account,
		Password:         cfg.Password,
	}, cfg.Arguments...)
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_SERVICE_ACCOUNT) {
			log.Printf(
				"Service control manager rejected account '%s'. "+
					"Well-known accounts must be specified exactly as '%s', "+
					"localized names are not supported\n",
				account, oscore.WindowsNetworkServiceAccount,
			)
		}

		return errors.Wrapf(err, "failed to create service '%s'", cfg.Name)
	}
	defer func() {
		_ = svc.Close()
	}()

	if len(cfg.RecoveryActions) == 0 {
		return nil
	}

	resetPeriod := cfg.ResetPeriod.Seconds()
	if resetPeriod < 0 || resetPeriod > math.MaxUint32 {
		return errors.Errorf("invalid failure reset period for service '%s'", cfg.Name)
	}

	err = svc.SetRecoveryActions(cfg.RecoveryActions, uint32(resetPeriod))
	if err != nil {
		return errors.Wrapf(err, "failed to configure failure actions for service '%s'", cfg.Name)
	}

	return nil
}

// windowsServiceBinaryPath returns the command line the service was registered with.
func windowsServiceBinaryPath(serviceName string) (string, error) {
	manager, err := mgr.Connect()
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to service control manager")
	}
	defer func() {
		_ = manager.Disconnect()
	}()

	svc, err := manager.OpenService(serviceName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open service '%s'", serviceName)
	}
	defer func() {
		_ = svc.Close()
	}()

	config, err := svc.Config()
	if err != nil {
		return "", errors.Wrapf(err, "failed to read config of service '%s'", serviceName)
	}

	return config.BinaryPathName, nil
}
