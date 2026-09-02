package https

import (
	"context"
	"io/fs"
	"log"
	"net"
	"os"

	panelletsencrypt "github.com/gameap/gameapctl/internal/actions/panel/letsencrypt"
	panelpkg "github.com/gameap/gameapctl/internal/pkg/panel"
	"github.com/gameap/gameapctl/pkg/configenv"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/panel"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func Disable(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	paths, err := panelpkg.ResolveScope(ctx, cliCtx.String("scope"))
	if err != nil {
		return err
	}

	if err = panelpkg.CheckBinaryInstalled(paths); err != nil {
		return err
	}

	configPath := paths.ConfigFilePath
	log.Printf("Reading config from: %s\n", configPath)

	lines, values, err := configenv.Read(configPath)
	if err != nil {
		return err
	}

	if panel.EffectiveCertSource(values) == panel.CertSourceNone {
		log.Println("HTTPS is already disabled.")

		return nil
	}

	// Nothing this command writes touches HTTP_HOST or HTTP_BIND_IP, so the
	// values read above still describe where the restarted panel will listen.
	bind := panel.ResolveBindAddress(ctx, values)

	if err = configenv.Update(configPath, lines, disableUpdates(values)); err != nil {
		return errors.WithMessage(err, "failed to write config")
	}

	log.Println("config.env updated. Restarting gameap ...")

	if err = panel.Restart(ctx, panel.Options{Scope: paths.Scope}); err != nil {
		return errors.WithMessage(err, "failed to restart gameap")
	}

	if err = waitForHTTP(ctx, bind, values); err != nil {
		return err
	}

	if cliCtx.Bool("purge") {
		purgeIssuedCertificate(paths, values)
	}

	log.Println("HTTPS is disabled, the panel serves plain HTTP only.")

	return nil
}

func disableUpdates(values map[string]string) map[string]string {
	updates := panel.TLSDisableUpdates()

	if !panel.ACMEEnabled(values) {
		return updates
	}

	log.Println("ACME is enabled, switching it off as well.")

	for key, value := range panelletsencrypt.DisableUpdates() {
		updates[key] = value
	}

	return updates
}

func waitForHTTP(ctx context.Context, bind panel.BindAddress, values map[string]string) error {
	httpPort := panel.ConfigValue(values, panel.HTTPPortKey)
	if httpPort == "" {
		httpPort = gameap.DefaultPanelPort
	}

	hosts := bind.ProbeHosts()

	err := waitFor(ctx, healthInterval, func() error {
		return panel.ProbeEach(hosts, func(host string) error {
			// The health check reports a bad response without saying which
			// address gave it, and here there may be more than one.
			return errors.WithMessage(
				panelpkg.CheckInstallationV4(ctx, host, httpPort, false),
				net.JoinHostPort(host, httpPort),
			)
		})
	})
	if err != nil {
		return errors.WithMessagef(err, "the panel is not answering on HTTP port %s", httpPort)
	}

	return nil
}

// purgeIssuedCertificate removes only the material this command issued: a
// certificate the operator pointed at with --cert belongs to the operator.
func purgeIssuedCertificate(paths gameap.PanelPaths, values map[string]string) {
	certPath, keyPath := certPaths(paths)

	if configured := panel.ConfigValue(values, panel.TLSCertFileKey); configured != "" && configured != certPath {
		log.Printf("Keeping %s, it was not issued by gameapctl.\n", configured)

		return
	}

	for _, path := range []string{certPath, keyPath} {
		err := os.Remove(path)

		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			log.Println(errors.WithMessagef(err, "failed to remove %s", path))
		default:
			log.Printf("Removed %s\n", path)
		}
	}
}
