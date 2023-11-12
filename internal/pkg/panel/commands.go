package panel

import (
	"context"
	"fmt"
	"log"
	"os/exec"

	"github.com/pkg/errors"
)

func GenerateEncryptionKey(_ context.Context, dir string) error {
	fmt.Println("Generating encryption key ...")
	cmd := exec.Command("php", "artisan", "key:generate", "--force")
	cmd.Dir = dir
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	err := cmd.Run()
	log.Println('\n', cmd.String())
	if err != nil {
		return errors.WithMessage(err, "failed to execute key generate command")
	}

	return nil
}

func RunMigration(_ context.Context, path string, withSeed bool) error {
	fmt.Println("Running migration ...")
	var cmd *exec.Cmd
	if withSeed {
		cmd = exec.Command("php", "artisan", "migrate", "--seed")
	} else {
		cmd = exec.Command("php", "artisan", "migrate")
	}

	cmd.Dir = path
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	log.Println('\n', cmd.String())
	err := cmd.Run()
	if err != nil {
		return errors.WithMessage(err, "failed to execute key generate command")
	}

	return nil
}

func ClearCache(_ context.Context, path string) error {
	cmd := exec.Command("php", "artisan", "config:cache")
	log.Println('\n', cmd.String())
	cmd.Dir = path
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	err := cmd.Run()
	if err != nil {
		return errors.WithMessage(err, "failed to clear config cache")
	}

	cmd = exec.Command("php", "artisan", "view:cache")
	log.Println('\n', cmd.String())
	cmd.Dir = path
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	err = cmd.Run()
	if err != nil {
		return errors.WithMessage(err, "failed to clear view cache")
	}

	return nil
}

func BuildStyles(_ context.Context, path string) error {
	log.Println("Installing npm dependencies ...")

	cmd := exec.Command("npm", "install")
	log.Println('\n', cmd.String())
	cmd.Dir = path
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	err := cmd.Run()
	if err != nil {
		return errors.WithMessage(err, "failed to install dependencies")
	}

	log.Println("Running building ...")
	cmd = exec.Command("npm", "run", "prod")
	log.Println('\n', cmd.String())
	cmd.Dir = path
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	err = cmd.Run()
	if err != nil {
		return errors.WithMessage(err, "failed to build nodejs application")
	}

	return nil
}
