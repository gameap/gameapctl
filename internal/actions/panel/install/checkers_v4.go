package install

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gameap/gameapctl/pkg/fixer"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func filterAndCheckHostV4(state panelInstallStateV4) (panelInstallStateV4, error) {
	state.Host = strings.TrimPrefix(state.Host, "http://")
	state.Host = strings.TrimPrefix(state.Host, "https://")
	state.Host = strings.TrimRight(state.Host, "/?&")

	if state.Port == "" {
		state.Port = "80"
	}

	if strings.ContainsAny(state.Host, "/?&") {
		return state, errors.New("invalid host")
	}

	if host, port, err := net.SplitHostPort(state.Host); err == nil {
		state.Host = host
		state.Port = port
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
		state.Port = "80"
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(state.Host, state.Port))
	if err != nil {
		warningErr := warningV4(ctx, state,
			fmt.Sprintf(
				"Port %s is already in use. "+
					"You can specify other available port. "+
					"Further installation may fail.", state.Port,
			),
		)
		if warningErr != nil {
			return state, warningErr
		}
	} else {
		err = listener.Close()
		if err != nil {
			return state, errors.WithMessage(err, "failed to close listener")
		}
	}

	return state, nil
}

func checkHTTPHostAvailabilityV4(ctx context.Context, state panelInstallStateV4) (panelInstallStateV4, error) {
	if state.Host == "localhost" || strings.HasPrefix(state.Host, "127.") {
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

		err = warningV4(ctx, state,
			fmt.Sprintf(
				"Host %s:%s is already in use. "+
					"You can specify other available port. "+
					"Further installation may fail.", state.Host, state.Port,
			),
		)
		if err != nil {
			return state, err
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
