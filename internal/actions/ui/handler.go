package ui

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/gameap/gameapctl/ui"
	"github.com/urfave/cli/v2"
)

const (
	defaultReadTimeout  = 5 * time.Second
	defaultWriteTimeout = 10 * time.Second
)

var (
	ErrUnsupportedPlatform = errors.New("unsupported platform")
)

var done = make(chan struct{})

func Handle(cliCtx *cli.Context) error {
	fs := http.FileServer(http.FS(ui.Assets()))
	http.Handle("/", http.StripPrefix("/", fs))
	http.HandleFunc("/ws", serveWs)

	addr := "localhost:17080"
	url := "http://" + addr

	noBrowser := cliCtx.Bool("no-browser")
	if !noBrowser {
		go func() {
			time.Sleep(1 * time.Second)
			fmt.Println("Opening", url, "in your default browser...")
			var err error
			switch runtime.GOOS {
			case "linux":
				err = exec.Command("xdg-open", url).Start()
			case "windows":
				err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
			case "darwin":
				err = exec.Command("open", url).Start()
			default:
				err = ErrUnsupportedPlatform
			}
			if err != nil {
				log.Fatal(err)
			}
		}()
	}

	fmt.Println("Server is running at", url)

	srv := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	select {
	case <-done:
	case <-cliCtx.Context.Done():
	}

	log.Println("Shutting down the server...")
	err := srv.Shutdown(cliCtx.Context)
	if err != nil {
		return err
	}

	return nil
}
