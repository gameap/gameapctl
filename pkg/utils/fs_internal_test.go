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

var nginxConfig = `server {
    listen       80;
    server_name  localhost;

    #charset koi8-r;
    #access_log  /var/log/nginx/log/host.access.log  main;
    root /var/www/gameap/public;
    index index.php index.html;

    location / {
		try_files $uri $uri/ /index.php$is_args$args;
        
        location = /index.php
		{
            #fastcgi_pass    localhost:9000;
            fastcgi_pass    unix:/var/run/php/php7.2-fpm.sock;
			fastcgi_param   SCRIPT_FILENAME $document_root$fastcgi_script_name;
			include         fastcgi_params;
        }
    }

    #error_page  404              /404.html;

    # redirect server error pages to the static page /50x.html
    #
    error_page   500 502 503 504  /50x.html;
    location = /50x.html {
        root   /usr/share/nginx/html;
    }
}`

func Test_findLineAndReplace_nginxConfig(t *testing.T) {
	r := strings.NewReader(nginxConfig)
	w := bytes.NewBuffer([]byte{})

	err := findLineAndReplace(context.Background(), r, w, map[string]string{
		"server_name":          "server_name	gameap.ru;",
		"listen":               "listen	81;",
		"fastcgi_pass    unix": "fastcgi_pass	unix:/var/run/php/php8.1-fpm.sock;",
	})
	result := w.String()

	require.NoError(t, err)
	assert.Contains(
		t,
		result,
		"server_name	gameap.ru;",
	)
	assert.Contains(
		t,
		result,
		"listen	81;",
	)
	assert.Contains(
		t,
		result,
		"fastcgi_pass	unix:/var/run/php/php8.1-fpm.sock;",
	)
	assert.Equal(t, "}\n", result[len(result)-2:])
}
