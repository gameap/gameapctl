//go:build linux || darwin

package panel

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakePanelBinary(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "gameap")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700))

	return path
}

func TestInstalledVersion(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "reports_the_tag_it_prints",
			body: `echo "GameAP v4.5.0"; echo "Build date: 2026-08-31"`,
			want: "v4.5.0",
		},
		{
			name: "accepts_a_tag_without_the_leading_v",
			body: `echo "GameAP 4.6.1"`,
			want: "4.6.1",
		},
		{
			name: "panel_older_than_v4_5_rejects_the_flag",
			body: `echo "flag provided but not defined: -version" >&2; exit 2`,
			want: "",
		},
		{
			name: "branch_build_reports_no_release",
			body: `echo "GameAP development"`,
			want: "",
		},
		{
			name: "empty_output",
			body: `true`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, InstalledVersion(context.Background(), fakePanelBinary(t, tt.body)))
		})
	}
}

func TestInstalledVersion_MissingBinary(t *testing.T) {
	assert.Empty(t, InstalledVersion(context.Background(), filepath.Join(t.TempDir(), "absent")))
}
