package main

import (
	"os"
	"path/filepath"

	"gameapctl/internal/app"
)

func main() {
	os.Args[0] = filepath.Base(os.Args[0])

	app.Run(os.Args)
}
