package letsencrypt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"example.com", []string{"example.com"}},
		{"a, b ,c", []string{"a", "b", "c"}},
		{"  *.example.com  , example.com ", []string{"*.example.com", "example.com"}},
		{",,empty,,", []string{"empty"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitAndTrim(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
