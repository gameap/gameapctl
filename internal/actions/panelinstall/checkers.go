package panelinstall

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func filterAndCheckHost(state panelInstallState) (panelInstallState, error) {
	if idx := strings.Index(state.Host, "http://"); idx >= 0 {
		state.Host = state.Host[7:]
	} else if idx = strings.Index(state.Host, "https://"); idx >= 0 {
		state.Host = state.Host[8:]
	}

	if state.Port == "" {
		state.Port = "80"
	}

	state.Host = strings.TrimRight(state.Host, "/?&")

	var invalidChars = []int32{'/', '?', '&'}
	for _, s := range state.Host {
		if utils.Contains(invalidChars, s) {
			return state, errors.New("invalid host")
		}
	}

	host, port, err := net.SplitHostPort(state.Host)
	if err != nil { //nolint:revive
		// Do nothing
	} else {
		state.Host = host
		state.Port = port
	}

	if utils.IsIPv4(state.Host) || utils.IsIPv6(state.Host) {
		state.HostIP = state.Host
	} else {
		ip, err := chooseIPFromHost(state.Host)
		var dnsErr *net.DNSError
		if err != nil && errors.As(err, &dnsErr) {
			// Do nothing
			return state, nil
		}
		if err != nil {
			return state, errors.WithMessage(err, "failed to choose IP from host")
		}

		state.HostIP = ip
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
		nginxConfPath, err := packagemanager.ConfigForDistro(ctx, packagemanager.NginxPackage, "nginx_conf")
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

	gameapConfPath, err := packagemanager.ConfigForDistro(ctx, packagemanager.NginxPackage, "gameap_host_conf")
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

//nolint:funlen
func checkHTTPHostAvailability(ctx context.Context, state panelInstallState) (panelInstallState, error) {
	_, err := net.LookupIP(state.Host)
	var dnsErr *net.DNSError
	if err != nil && errors.As(err, &dnsErr) {
		err = warning(ctx, state,
			fmt.Sprintf(
				"Failed to resolve host: %s. "+
					"Check that it is correct, without any typos. "+
					"Further installation may fail.", state.Host, //nolint:goconst
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
	}
	url := "http://" + state.Host + ":" + state.Port       //nolint:goconst
	req, err := http.NewRequest(http.MethodHead, url, nil) //nolint:noctx
	if err != nil {
		return state, err
	}
	resp, err := client.Do(req)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
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

	err = resp.Body.Close()
	if err != nil {
		fmt.Println("Failed to close a response body: ", err)
	}

	return state, nil
}