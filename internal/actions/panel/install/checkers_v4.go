package install

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/gameap/gameapctl/pkg/fixer"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func filterAndCheckHostV4(state panelInstallStateV4) (panelInstallStateV4, error) {
	state.Host = strings.TrimSpace(state.Host)
	state.Host = strings.TrimPrefix(state.Host, "http://")
	state.Host = strings.TrimPrefix(state.Host, "https://")
	state.Host = strings.TrimRight(state.Host, "/?&")

	if state.Port == "" {
		state.Port = defaultPortForScope(state.Scope)
	}

	if strings.ContainsAny(state.Host, "/?&") {
		return state, errors.New("invalid host")
	}

	if host, port, err := net.SplitHostPort(state.Host); err == nil {
		state.Host = host
		state.Port = port
		state.PortInput = port
	}

	if !utils.IsIPv4(state.Host) && !utils.IsIPv6(state.Host) {
		if ip, err := chooseIPFromHost(state.Host); err == nil {
			state.HostIP = ip
		} else if !errors.As(err, new(*net.DNSError)) {
			return state, errors.WithMessage(err, "failed to choose IP from host")
		}
	} else {
		state.HostIP = state.Host
	}

	return state, nil
}

func checkPortAvailabilityV4(ctx context.Context, state panelInstallStateV4) (panelInstallStateV4, error) {
	if state.Port == "" {
		state.Port = defaultPortForScope(state.Scope)
	}

	if existingPanelDetected(ctx, state) {
		fmt.Println("Existing GameAP panel detected on port", state.Port)

		return state, nil
	}

	listenAddr := resolveListenAddress(state.Host, state.Port)

	err := utils.CheckPortAvailability(listenAddr, state.Port)
	if err == nil {
		return state, nil
	}

	freePort, freePortFound := utils.FindAvailablePort(listenAddr, state.Port)
	if freePortFound && freePort == state.Port {
		// The port was released between the two probes.
		return state, nil
	}

	if freePortFound && state.PortInput == "" {
		message := fmt.Sprintf("Port %s is already in use, port %s will be used instead.", state.Port, freePort)
		fmt.Println(message)
		log.Println(message)

		state.Port = freePort

		return state, nil
	}

	// Probing rather than comparing against 1024: the port is bindable when the
	// administrator lowered net.ipv4.ip_unprivileged_port_start.
	if state.Scope == gameap.ScopeUser && errors.Is(err, syscall.EACCES) {
		return state, errors.Errorf(
			"port %s cannot be bound by an unprivileged process; "+
				"use a port >= 1024 (default for user scope: %s), put a reverse proxy in front, "+
				"or install with --scope=system",
			state.Port, defaultPortForScope(gameap.ScopeUser),
		)
	}

	text := fmt.Sprintf(
		"Port %s is already in use. "+
			"You can specify other available port. "+
			"Further installation may fail.", state.Port,
	)
	if freePortFound {
		text += fmt.Sprintf(" Port %s is free, you can re-run the installation with --port=%s.", freePort, freePort)
	}

	warningErr := warningV4(ctx, state, text)
	if warningErr != nil {
		return state, warningErr
	}

	return state, nil
}

const loopbackHost = "127.0.0.1"

// existingPanelDetected reports whether the panel of a previous installation already
// answers on the port. This is common during re-installation, and such a port must not be
// treated as occupied. A previous installation of the same scope on the very same port is
// required as well: any local service can answer with 200, and trusting one of those would
// leave the panel configured for a port it cannot bind.
//
// The previous panel listens on its configured HTTP_HOST, usually the public address rather
// than loopback, so that address is probed first. A panel that already redirects plain HTTP
// to HTTPS is not recognised here.
func existingPanelDetected(ctx context.Context, state panelInstallStateV4) bool {
	prev, ok := previousInstallation(ctx, state.Scope)
	if !ok || prev.Port != state.Port {
		return false
	}

	for _, host := range panelProbeHosts(prev) {
		if checkPanelHealth(ctx, host, state.Port, panelProbeTimeout) == nil {
			return true
		}
	}

	return false
}

// panelProbeHosts lists the addresses a previous installation may answer on, most
// specific first: the configured host, its resolved IP, then loopback.
func panelProbeHosts(prev gameapctl.PanelInstallState) []string {
	candidates := []string{prev.Host, prev.HostIP, loopbackHost}

	hosts := make([]string, 0, len(candidates))
	for _, host := range candidates {
		if host == "" || slices.Contains(hosts, host) {
			continue
		}

		hosts = append(hosts, host)
	}

	return hosts
}

// previousInstallation loads the state of a previous v4 installation of the same scope.
// A panel of the other scope runs from different paths and is never replaced by this one.
func previousInstallation(ctx context.Context, scope string) (gameapctl.PanelInstallState, bool) {
	prev, err := gameapctl.LoadPanelInstallState(ctx)
	if err != nil || !isPrevStateV4(prev.Version) {
		return gameapctl.PanelInstallState{}, false
	}

	if gameap.ScopeOrDefault(prev.Scope) != gameap.ScopeOrDefault(scope) {
		return gameapctl.PanelInstallState{}, false
	}

	return prev, true
}

func checkHTTPHostAvailabilityV4(ctx context.Context, state panelInstallStateV4) (panelInstallStateV4, error) {
	if state.Host == "localhost" || strings.HasPrefix(state.Host, "127.") ||
		state.Host == "::1" || state.Host == "[::1]" || state.Host == "0.0.0.0" {
		return state, nil
	}

	_, err := net.LookupIP(state.Host)
	var dnsErr *net.DNSError
	if err != nil && errors.As(err, &dnsErr) {
		err = warningV4(ctx, state,
			fmt.Sprintf(
				"Failed to resolve host: %s. "+
					"Check that it is correct, without any typos. "+
					"Further installation may fail.", state.Host,
			),
		)
		if err != nil {
			return state, err
		}
	}

	client := http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 2 * time.Second,
			}).DialContext,
		},
		Timeout: 2 * time.Second,
	}
	url := "http://" + state.Host + ":" + state.Port
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return state, err
	}

	var netErr net.Error
	var sysErr *os.SyscallError

	resp, err := client.Do(req)
	if err != nil &&
		(errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, context.Canceled) ||
			(errors.As(err, &netErr) && netErr.Timeout()) ||
			(errors.As(err, &sysErr) && errors.Is(sysErr.Err, syscall.ECONNREFUSED)) ||
			strings.Contains(err.Error(), "No connection could be made because the target machine actively refused it")) {
		// OK
		return state, nil
	}
	if err != nil {
		fmt.Println("Error: ", err)

		err = warningV4(ctx, state,
			fmt.Sprintf(
				"Failed to check host availability: %s. "+
					"Check that it is correct, without any typos. "+
					"Further installation may fail.", state.Host,
			),
		)
		if err != nil {
			return state, err
		}
	} else {
		err = resp.Body.Close()
		if err != nil {
			fmt.Println("Failed to close a response body: ", err)
		}
	}

	return state, nil
}

func checkSELinuxV4(ctx context.Context, state panelInstallStateV4) (panelInstallStateV4, error) {
	if runtime.GOOS == "windows" {
		return state, nil
	}

	enabled, err := fixer.IsSELinuxEnabled(ctx)
	if err != nil {
		return state, err
	}

	if enabled && state.Scope == gameap.ScopeUser {
		fmt.Println()
		fmt.Println(
			"Warning: SELinux is enabled. gameapctl cannot change SELinux settings in user scope; " +
				"if the panel fails to start, adjust the policy manually.",
		)

		return state, nil
	}

	if enabled {
		err := warningAskForActionV4(ctx, state,
			"SELinux is enabled. "+
				"The panel installation may fail due to the lack of necessary permissions.",
			"Do you want to disable SELinux? (Y/n): ",
			func(ctx context.Context) error {
				err := fixer.DisableSELinux(ctx)
				if err != nil {
					return errors.WithMessage(err, "failed to disable SELinux")
				}

				return nil
			},
		)
		if err != nil {
			return state, err
		}
	}

	return state, nil
}
