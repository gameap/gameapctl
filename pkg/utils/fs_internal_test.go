package utils

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var config = `SOME_VAR1=some_value
SOME_VAR2=some_value
SOME_VAR3=some_value3
`

func Test_findLineAndReplace(t *testing.T) {
	r := strings.NewReader(config)
	w := bytes.NewBuffer([]byte{})

	err := findLineAndReplace(context.Background(), r, w, map[string]string{
		"SOME_VAR2=": "SOME_VAR2=changed_value",
		"SOME_VAR3=": "SOME_VAR3=changed_value3",
	})

	require.NoError(t, err)
	assert.Equal(
		t,
		"SOME_VAR1=some_value\nSOME_VAR2=changed_value\nSOME_VAR3=changed_value3\n",
		w.String(),
	)
}

var configWithSpaces = `SOME_VAR1=some_value
    SOME_VAR2=some_value
	SOME_VAR3=some_value3
`

func Test_findLineAndReplace_withSpaces(t *testing.T) {
	r := strings.NewReader(configWithSpaces)
	w := bytes.NewBuffer([]byte{})

	err := findLineAndReplace(context.Background(), r, w, map[string]string{
		"SOME_VAR2=": "SOME_VAR2=changed_value",
		"SOME_VAR3=": "SOME_VAR3=changed_value3",
	})

	require.NoError(t, err)
	assert.Equal(
		t,
		"SOME_VAR1=some_value\n    SOME_VAR2=changed_value\n	SOME_VAR3=changed_value3\n",
		w.String(),
	)
}
