package gameap

import (
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
			"https://packages.hz1.gameap.io",
		}

		for _, cr := range repos {
			client := http.DefaultClient
			client.Timeout = 5 * time.Second //nolint:mnd

			//nolint:noctx
			r, err := client.Get(cr)
			if err != nil {
				continue
			}

			statusOK := r.StatusCode == http.StatusOK
			r.Body.Close()

			if statusOK {
				repo = cr

				break
			}
		}
	})

	return repo
}
