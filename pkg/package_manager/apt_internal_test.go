package packagemanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAptEnv(t *testing.T) {
	t.Setenv("GAMEAPCTL_APT_ENV_PROBE", "probe-value")

	env := aptEnv()

	assert.Contains(t, env, "DEBIAN_FRONTEND=noninteractive")
	assert.Contains(t, env, "NEEDRESTART_SUSPEND=1")
	assert.Contains(t, env, "GAMEAPCTL_APT_ENV_PROBE=probe-value")
}
