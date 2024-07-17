package panelinstall

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gameap/gameapctl/pkg/fixer"
	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func filterAndCheckHost(state panelInstallState) (panelInstallState, error) {
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

func chooseIPFromHost(host string) (string, error) {
	var result string

	ips, err := net.LookupIP(host)
	if err != nil {
		return "", errors.WithMessage(err, "failed to lookup ip")
	}

	if len(ips) == 0 {
		return "", errors.New("no ip for chosen host")
	}

	for i := range ips {
		if utils.IsIPv4(ips[i].String()) {
			result = ips[i].String()

			break
		}
	}

	if result == "" {
		result = ips[0].String()
	}

	return result, nil
}

func checkPath(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	if utils.IsFileExists(state.Path) {
		err := warning(ctx, state,
			fmt.Sprintf("Directory '%s' already exists. Files will be overwritten. "+
				"The panel installation may also fail.", state.Path),
		)
		if err != nil {
			return state, err
		}
	}

	return state, nil
}

func checkWebServers(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	if state.WebServer == noneWebServer {
		return state, nil
	}

	if state.WebServer == nginxWebServer {
		return checkNginxWebServer(ctx, state)
	}

	if state.WebServer == apacheWebServer {
		return checkApacheWebServer(ctx, state)
	}

	return state, nil
}

func checkNginxWebServer(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	_, err := exec.LookPath("nginx")
	//nolint:nestif
	if err == nil || (err != nil && !errors.Is(err, exec.ErrNotFound)) {
		err = warning(ctx, state,
			"Nginx is already installed. "+
				"The existing nginx configuration may be overwritten. The panel installation may also fail.",
		)
		if err != nil {
			return state, err
		}
	} else {
		var errNotFound packagemanager.NotFoundError
		nginxConfPath, err := packagemanager.ConfigForDistro(ctx, packagemanager.NginxPackage, "nginx_conf")
		if err != nil && errors.As(err, &errNotFound) {
			return state, nil
		}
		if err != nil {
			return state, err
		}
		if utils.IsFileExists(nginxConfPath) {
			err = warning(ctx, state,
				fmt.Sprintf("Nginx configuration file is already exists (%s). ", nginxConfPath)+
					"The existing nginx configuration will be overwritten. "+
					"The panel installation may also fail.",
			)
			if err != nil {
				return state, err
			}
		}
	}

	var errNotFound packagemanager.NotFoundError
	gameapConfPath, err := packagemanager.ConfigForDistro(ctx, packagemanager.NginxPackage, "gameap_host_conf")
	if err != nil && errors.As(err, &errNotFound) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if utils.IsFileExists(gameapConfPath) {
		err = warning(ctx, state,
			fmt.Sprintf("GameAP configuration file for Nginx is already exists (%s). ", gameapConfPath)+
				"The existing nginx configuration will be overwritten.",
		)
		if err != nil {
			return state, err
		}
	}

	return state, nil
}

func checkApacheWebServer(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	_, err := exec.LookPath("apache2")
	if err == nil || (err != nil && !errors.Is(err, exec.ErrNotFound)) {
		err = warning(ctx, state,
			"Apache is already installed. "+
				"The existing apache configuration may be overwritten. The panel installation may also fail.",
		)
		if err != nil {
			return state, err
		}
	}

	gameapConfPath, err := packagemanager.ConfigForDistro(
		ctx,
		packagemanager.ApachePackage,
		"gameap_host_conf",
	)
	if err != nil {
		return state, err
	}
	if utils.IsFileExists(gameapConfPath) {
		err = warning(ctx, state,
			fmt.Sprintf(
				"GameAP configuration file for Apache web server is already exists (%s)", gameapConfPath,
			)+
				"The existing nginx configuration will be overwritten.",
		)
		if err != nil {
			return state, err
		}
	}

	return state, nil
}

func checkPortAvailability(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	if state.Port == "" {
		state.Port = "80"
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(state.Host, state.Port))
	if err != nil {
		err = warning(ctx, state,
			fmt.Sprintf(
				"Port %s is already in use. "+
					"You can specify other available port. "+
					"Further installation may fail.", state.Port,
			),
		)
		if err != nil {
			return state, err
		}
	}
	err = listener.Close()
	if err != nil {
		return state, errors.WithMessage(err, "failed to close listener")
	}

	return state, nil
}

//nolint:funlen
func checkHTTPHostAvailability(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	//nolint:goconst
	if state.Host == "localhost" || strings.HasPrefix(state.Host, "127.") {
		return state, nil
	}

	_, err := net.LookupIP(state.Host)
	var dnsErr *net.DNSError
	if err != nil && errors.As(err, &dnsErr) {
		err = warning(ctx, state,
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

		err = warning(ctx, state,
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

		err = warning(ctx, state,
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

func checkSELinux(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	if runtime.GOOS == "windows" {
		return state, nil
	}

	enabled, err := fixer.IsSELinuxEnabled(ctx)
	if err != nil {
		return state, err
	}

	if enabled {
		err := warningAskForAction(ctx, state,
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
