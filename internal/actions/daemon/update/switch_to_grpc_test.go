package update

import (
	"context"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gameap/gameapctl/internal/pkg/gameapctl"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validLegacyConfig = `api_host: "https://panel.example.com"
api_key: "test-api-key"
ds_id: 7
listen_ip: "0.0.0.0"
listen_port: 31717
ca_certificate_file: "certs/ca.crt"
certificate_chain_file: "certs/server.crt"
private_key_file: "certs/server.key"
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "gameap-daemon.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))

	return p
}

func newRecordingDeps(t *testing.T) *recordingDeps {
	t.Helper()

	return &recordingDeps{
		t:         t,
		procRuns:  true,
		verifySeq: []error{nil},
	}
}

var (
	errShouldNotBeCalled = errors.New("should not be called")
	errConnRefused       = errors.New("connection refused")
	errStartBoom         = errors.New("start boom")
	errNoStateFile       = errors.New("no state file")
	errLegacyStillUp     = errors.New("expected HTTP 409, got 200")
)

type recordingDeps struct {
	t            *testing.T
	stopped      int
	started      int
	procRuns     bool
	stopErr      error
	startErr     error
	tcpErr       error
	tlsErr       error
	lookPathErr  error
	verifySeq    []error
	verifyCalls  int
	stateLoadErr error
	stateSaved   bool
	state        gameapctl.DaemonInstallState
	printf       []string
}

func (r *recordingDeps) build(cfgPath, explicit string) switchDeps {
	return switchDeps{
		cfgPath:          cfgPath,
		explicitGRPCAddr: explicit,
		stopDaemon: func(context.Context) error {
			r.stopped++

			return r.stopErr
		},
		startDaemon: func(context.Context) error {
			r.started++

			return r.startErr
		},
		findProcess: func(context.Context) (*process.Process, error) {
			if r.procRuns {
				return &process.Process{}, nil
			}

			return nil, nil
		},
		lookPath: func(string) (string, error) {
			if r.lookPathErr != nil {
				return "", r.lookPathErr
			}

			return "/usr/bin/gameap-daemon", nil
		},
		tcpDial:  func(string) error { return r.tcpErr },
		tlsProbe: func(_, _, _, _ string) error { return r.tlsErr },
		verifyLegacyRevoked: func(_ context.Context, _, _ string) error {
			defer func() { r.verifyCalls++ }()
			if r.verifyCalls < len(r.verifySeq) {
				return r.verifySeq[r.verifyCalls]
			}

			return r.verifySeq[len(r.verifySeq)-1]
		},
		sleep: func(time.Duration) {},
		loadState: func(context.Context) (gameapctl.DaemonInstallState, error) {
			if r.stateLoadErr != nil {
				return gameapctl.DaemonInstallState{}, r.stateLoadErr
			}

			return r.state, nil
		},
		saveState: func(_ context.Context, s gameapctl.DaemonInstallState) error {
			r.stateSaved = true
			r.state = s

			return nil
		},
		printf: func(format string, a ...interface{}) {
			r.printf = append(r.printf, format)
		},
	}
}

func TestSwitchToGRPC_AlreadyEnabled_ShortCircuits(t *testing.T) {
	cfg := validLegacyConfig + "grpc:\n  enabled: true\n"
	p := writeConfig(t, cfg)
	r := newRecordingDeps(t)
	r.tcpErr = errShouldNotBeCalled
	r.tlsErr = errShouldNotBeCalled

	err := switchToGRPC(context.Background(), r.build(p, ""))
	require.NoError(t, err)

	original, _ := os.ReadFile(p)
	assert.Equal(t, cfg, string(original))
	assert.Equal(t, 0, r.stopped)
	assert.Equal(t, 0, r.started)
	assert.Equal(t, 0, r.verifyCalls)
}

func TestSwitchToGRPC_APIHostMissing_NoFlag_ReturnsError(t *testing.T) {
	cfg := `api_key: "k"
ds_id: 1
ca_certificate_file: "ca"
certificate_chain_file: "c"
private_key_file: "k"
`
	p := writeConfig(t, cfg)
	r := newRecordingDeps(t)

	err := switchToGRPC(context.Background(), r.build(p, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_host not found")

	after, _ := os.ReadFile(p)
	assert.Equal(t, cfg, string(after))
	assertNoBackup(t, p)
}

func TestSwitchToGRPC_TCPUnreachable_AbortsWithoutChanges(t *testing.T) {
	p := writeConfig(t, validLegacyConfig)
	r := newRecordingDeps(t)
	r.tcpErr = errConnRefused

	err := switchToGRPC(context.Background(), r.build(p, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API gRPC port unreachable")

	after, _ := os.ReadFile(p)
	assert.Equal(t, validLegacyConfig, string(after))
	assertNoBackup(t, p)
	assert.Equal(t, 0, r.stopped)
}

func TestSwitchToGRPC_TLSUnknownCA_ReturnsReinstallMessage(t *testing.T) {
	p := writeConfig(t, validLegacyConfig)
	r := newRecordingDeps(t)
	r.tlsErr = x509.UnknownAuthorityError{}

	err := switchToGRPC(context.Background(), r.build(p, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "re-install the daemon")

	after, _ := os.ReadFile(p)
	assert.Equal(t, validLegacyConfig, string(after))
	assertNoBackup(t, p)
}

func TestSwitchToGRPC_HappyPath(t *testing.T) {
	p := writeConfig(t, validLegacyConfig)
	r := newRecordingDeps(t)

	err := switchToGRPC(context.Background(), r.build(p, ""))
	require.NoError(t, err)

	after, _ := os.ReadFile(p)
	assert.Contains(t, string(after), "grpc:")
	assert.Contains(t, string(after), "enabled: true")
	assert.Contains(t, string(after), "address:")
	assert.Contains(t, string(after), "panel.example.com:31718")
	assert.NotContains(t, string(after), "api_host")
	assert.NotContains(t, string(after), "listen_ip")
	assert.NotContains(t, string(after), "listen_port")
	assert.Contains(t, string(after), "api_key")
	assert.Contains(t, string(after), "ca_certificate_file")
	assert.Equal(t, 1, r.stopped)
	assert.Equal(t, 1, r.started)
	assert.Equal(t, 1, r.verifyCalls)
	assert.True(t, r.stateSaved)
	assert.True(t, r.state.GRPCEnabled)
}

func TestSwitchToGRPC_VerificationPolls_ThenSucceeds(t *testing.T) {
	p := writeConfig(t, validLegacyConfig)
	r := newRecordingDeps(t)
	r.verifySeq = []error{
		errLegacyStillUp,
		errLegacyStillUp,
		nil,
	}

	err := switchToGRPC(context.Background(), r.build(p, ""))
	require.NoError(t, err)
	assert.Equal(t, 3, r.verifyCalls)
}

func TestSwitchToGRPC_LegacyNeverRevoked_TriggersRollback(t *testing.T) {
	p := writeConfig(t, validLegacyConfig)
	r := newRecordingDeps(t)
	r.verifySeq = []error{errLegacyStillUp}

	err := switchToGRPC(context.Background(), r.build(p, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolled back to legacy")

	after, _ := os.ReadFile(p)
	assert.Equal(t, validLegacyConfig, string(after))
	assert.Equal(t, verificationPollMaxAttempts, r.verifyCalls)
	assert.GreaterOrEqual(t, r.started, 2) // initial start + rollback restart
}

func TestSwitchToGRPC_StartDaemonFails_RollbackRestoresAndStarts(t *testing.T) {
	p := writeConfig(t, validLegacyConfig)
	r := newRecordingDeps(t)
	startCalls := 0
	startFn := func(context.Context) error {
		startCalls++
		if startCalls == 1 {
			return errStartBoom
		}

		return nil
	}

	deps := r.build(p, "")
	deps.startDaemon = startFn

	err := switchToGRPC(context.Background(), deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolled back to legacy")

	after, _ := os.ReadFile(p)
	assert.Equal(t, validLegacyConfig, string(after))
	assert.Equal(t, 2, startCalls)
}

func TestSwitchToGRPC_ExplicitGRPCAddress_WrittenToConfig(t *testing.T) {
	p := writeConfig(t, validLegacyConfig)
	r := newRecordingDeps(t)

	err := switchToGRPC(context.Background(), r.build(p, "grpc.example.com:31718"))
	require.NoError(t, err)

	after, _ := os.ReadFile(p)
	assert.Contains(t, string(after), "address:")
	assert.Contains(t, string(after), "grpc.example.com:31718")
	assert.NotContains(t, string(after), "api_host")
	assert.NotContains(t, string(after), "listen_ip")
	assert.NotContains(t, string(after), "listen_port")
	assert.Contains(t, string(after), "api_key")
}

func TestSwitchToGRPC_StateLoadError_NonFatal(t *testing.T) {
	p := writeConfig(t, validLegacyConfig)
	r := newRecordingDeps(t)
	r.stateLoadErr = errNoStateFile

	err := switchToGRPC(context.Background(), r.build(p, ""))
	require.NoError(t, err)
	assert.False(t, r.stateSaved)
}

func TestDeriveGRPCAddress(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "https://panel.example.com", want: "panel.example.com:31718"},
		{input: "http://panel.example.com:8080", want: "panel.example.com:31718"},
		{input: "panel.example.com", want: "panel.example.com:31718"},
		{input: "https://panel.example.com/subpath", want: "panel.example.com:31718"},
		{input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := deriveGRPCAddress(tt.input)
			if tt.wantErr {
				require.Error(t, err)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func assertNoBackup(t *testing.T, cfgPath string) {
	t.Helper()
	dir := filepath.Dir(cfgPath)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".bak.")
	}
}

func TestRealVerifyLegacyRevoked_409WithMarker_Succeeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("daemon is connected via gRPC bidi stream, HTTP API is disabled for this node"))
	}))
	defer srv.Close()

	err := realVerifyLegacyRevoked(context.Background(), srv.URL, "test-key")
	require.NoError(t, err)
}

func TestRealVerifyLegacyRevoked_409WithoutMarker_Fails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("conflict: some other reason"))
	}))
	defer srv.Close()

	err := realVerifyLegacyRevoked(context.Background(), srv.URL, "test-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "without expected marker")
}

func TestRealVerifyLegacyRevoked_200_Fails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"legacy-token"}`))
	}))
	defer srv.Close()

	err := realVerifyLegacyRevoked(context.Background(), srv.URL, "test-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected HTTP 409, got 200")
}

func TestRealVerifyLegacyRevoked_500_Fails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	err := realVerifyLegacyRevoked(context.Background(), srv.URL, "test-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected HTTP 409, got 500")
}

func TestRealVerifyLegacyRevoked_RequestShape(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotAuth   string
		gotCT     string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("HTTP API is disabled for this node"))
	}))
	defer srv.Close()

	err := realVerifyLegacyRevoked(context.Background(), srv.URL, "secret-key")
	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, gotMethod)
	assert.Equal(t, "/gdaemon_api/get_token", gotPath)
	assert.Equal(t, "Bearer secret-key", gotAuth)
	assert.Equal(t, "application/json", gotCT)
}
