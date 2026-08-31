package panel

import (
	"testing"

	"github.com/gameap/gameapctl/pkg/releasefinder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendUnit(t *testing.T) {
	appendSeconds := appendUnit("s")

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "bare_number_gains_the_unit", value: "45", want: "45s"},
		{name: "value_that_already_has_a_unit_passes_through", value: "45s", want: "45s"},
		{name: "surrounding_space_is_trimmed", value: " 45 ", want: "45s"},
		{name: "empty_stays_empty", value: "", want: ""},
		{name: "unrelated_value_passes_through", value: "forever", want: "forever"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, appendSeconds(tt.value))
		})
	}
}

func TestDurationToSeconds(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "unit_is_stripped", value: "45s", want: "45"},
		{name: "compound_duration_becomes_whole_seconds", value: "1m30s", want: "90"},
		{name: "bare_number_passes_through", value: "45", want: "45"},
		{name: "sub_second_duration_truncates", value: "1500ms", want: "1"},
		{name: "empty_stays_empty", value: "", want: ""},
		{name: "unparseable_value_passes_through", value: "forever", want: "forever"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, durationToSeconds(tt.value))
		})
	}
}

func TestByteSizeToBytes(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "kilobytes_are_powers_of_1024", value: "64K", want: "65536"},
		{name: "megabytes", value: "1M", want: "1048576"},
		{name: "long_suffix", value: "1MiB", want: "1048576"},
		{name: "lowercase_suffix", value: "64k", want: "65536"},
		{name: "bare_byte_count_passes_through", value: "65536", want: "65536"},
		{name: "empty_stays_empty", value: "", want: ""},
		{name: "unparseable_value_passes_through", value: "plenty", want: "plenty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, byteSizeToBytes(tt.value))
		})
	}
}

// TestConfigEnvMigrations_HaveNoDuplicateSources guards the table against a
// second entry claiming a key an earlier one already renames or drops: the
// second would silently never fire.
func TestConfigEnvMigrations_HaveNoDuplicateSources(t *testing.T) {
	seen := make(map[string]string)

	for _, migration := range configEnvMigrations {
		for _, rename := range migration.Renames {
			previous, taken := seen[rename.Old]
			require.False(t, taken, "%s is already migrated by %s", rename.Old, previous)

			seen[rename.Old] = migration.MinVersion
		}

		for _, drop := range migration.Drops {
			previous, taken := seen[drop.Key]
			require.False(t, taken, "%s is already migrated by %s", drop.Key, previous)

			seen[drop.Key] = migration.MinVersion
		}
	}
}

// TestConfigEnvMigrations_AreOrderIndependentWithinAnEntry keeps a chained
// rename from hiding inside a single entry, where the order of the slice would
// decide the outcome. Chains belong between entries, which are applied in
// order. One check covers both directions: a chain exists in either only if
// some name is both an Old and a New of the same entry.
func TestConfigEnvMigrations_AreOrderIndependentWithinAnEntry(t *testing.T) {
	for _, migration := range configEnvMigrations {
		sources := make(map[string]bool, len(migration.Renames))

		for _, rename := range migration.Renames {
			sources[rename.Old] = true
		}

		for _, rename := range migration.Renames {
			assert.False(t, sources[rename.New],
				"%s: %s renames into %s, which the same entry also renames",
				migration.MinVersion, rename.Old, rename.New)
		}
	}
}

// TestConfigEnvMigrations_ConvertAndRevertComeInPairs keeps a downgrade from
// carrying a value the older panel cannot parse.
func TestConfigEnvMigrations_ConvertAndRevertComeInPairs(t *testing.T) {
	for _, migration := range configEnvMigrations {
		for _, rename := range migration.Renames {
			if rename.Convert == nil && rename.Revert == nil {
				continue
			}

			assert.NotNil(t, rename.Revert, "%s converts its value but cannot revert it", rename.Old)
		}
	}
}

func TestConfigEnvMigrations_AreOrderedByVersion(t *testing.T) {
	require.NotEmpty(t, configEnvMigrations)

	for i := 1; i < len(configEnvMigrations); i++ {
		assert.True(t,
			releasefinder.IsAtLeast(configEnvMigrations[i].MinVersion, configEnvMigrations[i-1].MinVersion),
			"migration %d (%s) is older than the one before it (%s)",
			i, configEnvMigrations[i].MinVersion, configEnvMigrations[i-1].MinVersion)
	}
}
