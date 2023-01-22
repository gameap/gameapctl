package packagemanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parsePHPVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{
			name:  "php-7.2",
			input: "PHP 7.2.24-0ubuntu0.18.04.7 (cli) (built: Oct  8 2020 12:31:42) ( NTS )",
			want:  "7.2",
		},
		{
			name:  "php-7.4",
			input: "PHP 7.4.3 (cli) (built: Feb 27 2020 15:23:01) ( NTS )",
			want:  "7.4",
		},
		{
			name:  "php-8.0",
			input: "PHP 8.0.0 (cli) (built: Dec  2 2020 15:59:48) ( NTS )",
			want:  "8.0",
		},
		{
			name:  "php-7.2-multistring",
			input: "\nSome information\nPHP 7.2.24-0ubuntu0.18.04.7 (cli) (built: Oct  8 2020 12:31:42) ( NTS )",
			want:  "7.2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v, err := parsePHPVersion(test.input)

			if test.wantErr != nil {
				assert.EqualError(t, err, test.wantErr.Error())
			}

			if test.want != "" {
				assert.Equal(t, test.want, v)
			}
		})
	}
}
