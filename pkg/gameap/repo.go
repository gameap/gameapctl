package gameap

import (
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

var repo string
var repoOnce sync.Once

func Repository() string {
	repoOnce.Do(func() {
		repos := []string{
			"https://packages.gameap.com",
			"https://packages.gameap.ru",
			"http://packages.hz1.gameap.io",
		}

		for _, cr := range repos {
			client := http.DefaultClient
			client.Timeout = 5 * time.Second //nolint:gomnd

			//nolint:bodyclose,noctx
			r, err := client.Get(cr)
			if err != nil {
				continue
			}
			defer func(body io.ReadCloser) {
				err := body.Close()
				if err != nil {
					log.Println(err)
				}
			}(r.Body)

			if r.StatusCode == http.StatusOK {
				repo = cr

				break
			}
		}
	})

	return repo
}
