package update

import (
	"context"
	"crypto/x509"
	"errors"
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
		t:        t,
		procRuns: true,
		fetchSeq: []fetchResult{{conn: "grpc"}},
	}
}

var (
	errShouldNotBeCalled = errors.New("should not be called")
	errConnRefused       = errors.New("connection refused")
	errStartBoom         = errors.New("start boom")
	errNoStateFile       = errors.New("no state file")
)

type fetchResult struct {
	conn string
	err  error
}

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
	fetchSeq     []fetchResult
	fetchCalls   int
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
		fetchConnType: func(_ context.Context, _ string, _ uint, _ string) (string, error) {
			defer func() { r.fetchCalls++ }()
			if r.fetchCalls < len(r.fetchSeq) {
				res := r.fetchSeq[r.fetchCalls]

				return res.conn, res.err
			}
			last := r.fetchSeq[len(r.fetchSeq)-1]

			return last.conn, last.err
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
	assert.Equal(t, 0, r.fetchCalls)
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
	assert.Equal(t, 1, r.stopped)
	assert.Equal(t, 1, r.started)
	assert.Equal(t, 1, r.fetchCalls)
	assert.True(t, r.stateSaved)
	assert.True(t, r.state.GRPCEnabled)
}

func TestSwitchToGRPC_VerificationPolls_ThenSucceeds(t *testing.T) {
	p := writeConfig(t, validLegacyConfig)
	r := newRecordingDeps(t)
	r.fetchSeq = []fetchResult{
		{conn: "legacy"},
		{conn: "legacy"},
		{conn: "grpc"},
	}

	err := switchToGRPC(context.Background(), r.build(p, ""))
	require.NoError(t, err)
	assert.Equal(t, 3, r.fetchCalls)
}

func TestSwitchToGRPC_VerificationNeverGRPC_TriggersRollback(t *testing.T) {
	p := writeConfig(t, validLegacyConfig)
	r := newRecordingDeps(t)
	r.fetchSeq = []fetchResult{{conn: "legacy"}}

	err := switchToGRPC(context.Background(), r.build(p, ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolled back to legacy")

	after, _ := os.ReadFile(p)
	assert.Equal(t, validLegacyConfig, string(after))
	assert.Equal(t, verificationPollMaxAttempts, r.fetchCalls)
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

	err := switchToGRPC(context.Background(), r.build(p, "panel.example.com:31718"))
	require.NoError(t, err)

	after, _ := os.ReadFile(p)
	assert.Contains(t, string(after), "address:")
	assert.Contains(t, string(after), "panel.example.com:31718")
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
