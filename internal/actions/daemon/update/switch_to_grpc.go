package update

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	daemonpkg "github.com/gameap/gameapctl/internal/pkg/daemon"
	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	pkgdaemon "github.com/gameap/gameapctl/pkg/daemon"
	"github.com/gameap/gameapctl/pkg/gameap"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/urfave/cli/v2"
)

const (
	defaultGRPCPort             = "31718"
	verificationPollInterval    = 1 * time.Second
	verificationPollMaxAttempts = 10
	postStartGracePeriod        = 3 * time.Second
	tlsDialTimeout              = 5 * time.Second
	httpRequestTimeout          = 10 * time.Second

	grpcDisabledMarker = "HTTP API is disabled for this node"
)

var errVerificationTimeout = errors.New("panel did not revoke legacy credentials within timeout")

type switchDeps struct {
	cfgPath          string
	explicitGRPCAddr string

	stopDaemon  func(context.Context) error
	startDaemon func(context.Context) error
	findProcess func(context.Context) (*process.Process, error)
	lookPath    func(string) (string, error)

	tcpDial             func(addr string) error
	tlsProbe            func(caFile, certFile, keyFile, addr string) error
	verifyLegacyRevoked func(ctx context.Context, apiHost, apiKey string) error

	sleep     func(time.Duration)
	loadState func(context.Context) (gameapctl.DaemonInstallState, error)
	saveState func(context.Context, gameapctl.DaemonInstallState) error
	printf    func(format string, a ...interface{})
}

func HandleSwitchToGRPC(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	deps := switchDeps{
		cfgPath:             gameap.DefaultDaemonConfigFilePath,
		explicitGRPCAddr:    cliCtx.String("grpc-address"),
		stopDaemon:          stopDaemon,
		startDaemon:         startDaemon,
		findProcess:         pkgdaemon.FindProcess,
		lookPath:            exec.LookPath,
		tcpDial:             daemonpkg.CheckGRPCConnectivity,
		tlsProbe:            realTLSProbe,
		verifyLegacyRevoked: realVerifyLegacyRevoked,
		sleep:               time.Sleep,
		loadState:           gameapctl.LoadDaemonInstallState,
		saveState:           gameapctl.SaveDaemonInstallState,
		printf: func(format string, a ...interface{}) {
			fmt.Printf(format, a...)
		},
	}

	return switchToGRPC(ctx, deps)
}

//nolint:funlen,gocognit,gocyclo,cyclop
func switchToGRPC(ctx context.Context, deps switchDeps) error {
	deps.printf("Switching daemon to gRPC mode...\n")

	cfg, err := daemonpkg.LoadConfig(deps.cfgPath)
	if err != nil {
		return errors.WithMessage(err, "failed to load gRPC configuration")
	}

	if enabled, _, _ := cfg.ReadString("$.grpc.enabled"); enabled == "true" {
		deps.printf("Daemon is already in gRPC mode. Nothing to do.\n")

		return nil
	}

	apiHost, hostOk, err := cfg.ReadString("$.api_host")
	if err != nil {
		return errors.WithMessage(err, "failed to read api_host")
	}

	grpcAddr := deps.explicitGRPCAddr
	if grpcAddr == "" {
		if !hostOk || apiHost == "" {
			return errors.New("api_host not found in daemon config; specify --grpc-address explicitly")
		}
		grpcAddr, err = deriveGRPCAddress(apiHost)
		if err != nil {
			return errors.WithMessage(err, "failed to derive gRPC address from api_host")
		}
	}

	apiKey, keyOk, _ := cfg.ReadString("$.api_key")
	if !keyOk || apiKey == "" {
		return errors.New("api_key missing in daemon config; daemon is not properly registered")
	}
	if nodeID, idOk, _ := cfg.ReadUint("$.ds_id"); !idOk || nodeID == 0 {
		return errors.New("ds_id missing in daemon config; daemon is not properly registered")
	}

	caFile, _, _ := cfg.ReadString("$.ca_certificate_file")
	certFile, _, _ := cfg.ReadString("$.certificate_chain_file")
	keyFile, _, _ := cfg.ReadString("$.private_key_file")
	if caFile == "" || certFile == "" || keyFile == "" {
		return errors.New("certificate paths missing in daemon config")
	}

	cfgDir := filepath.Dir(deps.cfgPath)
	caFile = resolveCertPath(cfgDir, caFile)
	certFile = resolveCertPath(cfgDir, certFile)
	keyFile = resolveCertPath(cfgDir, keyFile)

	deps.printf("Checking gRPC connectivity to %s ...\n", grpcAddr)
	if err := deps.tcpDial(grpcAddr); err != nil {
		return errors.WithMessage(err, "API gRPC port unreachable; verify GRPC_ENABLED=true on API and port is open")
	}

	deps.printf("Verifying TLS handshake with API gRPC server...\n")
	if err := deps.tlsProbe(caFile, certFile, keyFile, grpcAddr); err != nil {
		if isCertAuthError(err) {
			return errors.WithMessage(
				err,
				"daemon certificates are not compatible with API gRPC server. "+
					"This usually means the daemon was registered with a different panel. "+
					"Please re-install the daemon via `gameapctl daemon install --connect=grpc://...`",
			)
		}

		return errors.WithMessage(err, "TLS probe to API gRPC server failed")
	}

	if _, err := deps.lookPath("gameap-daemon"); err != nil {
		return errors.Wrap(err, "gameap-daemon binary not found")
	}

	if proc, _ := deps.findProcess(ctx); proc == nil {
		deps.printf("WARNING: daemon process is not currently running\n")
	}

	backupPath, err := daemonpkg.Backup(deps.cfgPath)
	if err != nil {
		return errors.WithMessage(err, "failed to backup daemon config")
	}
	deps.printf("Config backed up to %s\n", backupPath)

	rollback := func(originalErr error) error {
		deps.printf("Rolling back...\n")
		_ = deps.stopDaemon(ctx)
		if restoreErr := daemonpkg.Restore(backupPath, deps.cfgPath); restoreErr != nil {
			return fmt.Errorf("CRITICAL: rollback failed: %v (original: %w)", restoreErr, originalErr)
		}
		if startErr := deps.startDaemon(ctx); startErr != nil {
			return fmt.Errorf("CRITICAL: restart after rollback failed: %v (original: %w)", startErr, originalErr)
		}

		return errors.WithMessage(originalErr, "switch to gRPC failed, daemon rolled back to legacy mode")
	}

	if err := cfg.EnsureGRPCEnabled(grpcAddr); err != nil {
		return rollback(errors.WithMessage(err, "failed to modify daemon config"))
	}
	for _, key := range []string{"$.api_host", "$.listen_ip", "$.listen_port"} {
		if err := cfg.DeleteKey(key); err != nil {
			return rollback(errors.WithMessagef(err, "failed to remove %s from daemon config", key))
		}
	}
	if err := cfg.Save(); err != nil {
		return rollback(errors.WithMessage(err, "failed to save daemon config"))
	}

	deps.printf("Stopping daemon...\n")
	if err := deps.stopDaemon(ctx); err != nil {
		return rollback(errors.WithMessage(err, "failed to stop daemon"))
	}

	deps.printf("Starting daemon...\n")
	if err := deps.startDaemon(ctx); err != nil {
		return rollback(errors.WithMessage(err, "failed to start daemon"))
	}

	deps.sleep(postStartGracePeriod)

	if proc, err := deps.findProcess(ctx); err != nil || proc == nil {
		return rollback(errors.New("daemon process not found after restart"))
	}

	deps.printf("Verifying that panel revoked legacy credentials...\n")
	var (
		lastErr   error
		succeeded bool
	)
	for attempt := 0; attempt < verificationPollMaxAttempts; attempt++ {
		lastErr = deps.verifyLegacyRevoked(ctx, apiHost, apiKey)
		if lastErr == nil {
			succeeded = true

			break
		}
		deps.sleep(verificationPollInterval)
	}
	if !succeeded {
		return rollback(errors.Wrapf(
			errVerificationTimeout,
			"after %d attempts (last error: %v)",
			verificationPollMaxAttempts, lastErr,
		))
	}

	deps.printf("Daemon successfully switched to gRPC mode\n")

	state, stateErr := deps.loadState(ctx)
	if stateErr != nil {
		log.Printf("daemon install state not found, skipping state update: %v", stateErr)
	} else {
		state.GRPCEnabled = true
		if saveErr := deps.saveState(ctx, state); saveErr != nil {
			log.Printf("failed to persist state (non-fatal): %v", saveErr)
		}
	}

	return nil
}

// deriveGRPCAddress strips scheme and path from api_host and replaces the port with
// defaultGRPCPort. Mirrors gameap-daemon's config.GRPCAddress() logic.
func deriveGRPCAddress(apiHost string) (string, error) {
	host := strings.TrimSpace(apiHost)
	if host == "" {
		return "", errors.New("empty api_host")
	}

	if !strings.Contains(host, "://") {
		host = "http://" + host
	}

	u, err := url.Parse(host)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse api_host")
	}

	hostname := u.Hostname()
	if hostname == "" {
		return "", errors.New("api_host has no hostname")
	}

	return net.JoinHostPort(hostname, defaultGRPCPort), nil
}

func resolveCertPath(cfgDir, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(cfgDir, path)
}

func realTLSProbe(caFile, certFile, keyFile, addr string) error {
	caBytes, err := os.ReadFile(caFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read CA certificate %s", caFile)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return errors.Errorf("failed to parse CA certificate %s", caFile)
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return errors.Wrap(err, "failed to load client certificate/key")
	}

	host, _, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		host = addr
	}

	cfg := &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
		ServerName:   host,
		MinVersion:   tls.VersionTLS12,
	}

	dialer := &net.Dialer{Timeout: tlsDialTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, cfg)
	if err != nil {
		return err
	}
	_ = conn.Close()

	return nil
}

func isCertAuthError(err error) bool {
	if err == nil {
		return false
	}
	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return true
	}
	var invalid x509.CertificateInvalidError
	if errors.As(err, &invalid) {
		return true
	}
	msg := err.Error()

	return strings.Contains(msg, "bad certificate") ||
		strings.Contains(msg, "certificate signed by unknown authority") ||
		strings.Contains(msg, "unknown certificate authority")
}

// realVerifyLegacyRevoked queries the legacy /gdaemon_api/get_token endpoint.
// After the daemon successfully registers via gRPC, the panel rejects the
// legacy HTTP path with 409 Conflict and a body containing grpcDisabledMarker.
// Any other response (2xx, 4xx != 409, 5xx, transport failure) is treated as
// "not yet revoked" and retried by the polling loop.
func realVerifyLegacyRevoked(ctx context.Context, apiHost, apiKey string) error {
	base := normalizeAPIHost(apiHost)
	endpoint := base + "/gdaemon_api/get_token"

	reqCtx, cancel := context.WithTimeout(ctx, httpRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return errors.Wrap(err, "failed to build request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: httpRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "request failed")
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := strings.TrimSpace(string(body))

	if resp.StatusCode != http.StatusConflict {
		return errors.Errorf("expected HTTP 409, got %d: %s", resp.StatusCode, bodyStr)
	}

	if !strings.Contains(bodyStr, grpcDisabledMarker) {
		return errors.Errorf("HTTP 409 without expected marker %q: %s", grpcDisabledMarker, bodyStr)
	}

	return nil
}

func normalizeAPIHost(apiHost string) string {
	h := strings.TrimRight(strings.TrimSpace(apiHost), "/")
	if !strings.Contains(h, "://") {
		h = "https://" + h
	}

	return h
}
