package panel

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"

	packagemanager "github.com/gameap/gameapctl/pkg/package_manager"
	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func GenerateEncryptionKey(_ context.Context, dir string) error {
	fmt.Println("Generating encryption key ...")

	cmdName, args, err := packagemanager.DefinePHPCommandAndArgs(
		filepath.Join(dir, "artisan"), "key:generate", "--force",
	)
	if err != nil {
		return errors.WithMessage(err, "failed to define php command and args")
	}

	cmd := exec.Command(cmdName, args...)
	cmd.Dir = dir
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	err = cmd.Run()
	log.Println('\n', cmd.String())
	if err != nil {
		return errors.WithMessage(err, "failed to execute key generate command")
	}

	return nil
}

func RunMigration(_ context.Context, path string, withSeed bool) error {
	if withSeed {
		fmt.Println("Running migration with seed ...")
	} else {
		fmt.Println("Running migration ...")
	}

	var cmd *exec.Cmd
	if withSeed {
		cmdName, args, err := packagemanager.DefinePHPCommandAndArgs(
			filepath.Join(path, "artisan"), "migrate", "--seed",
		)
		if err != nil {
			return errors.WithMessage(err, "failed to define php command and args")
		}

		cmd = exec.Command(cmdName, args...)
	} else {
		cmdName, args, err := packagemanager.DefinePHPCommandAndArgs(
			filepath.Join(path, "artisan"), "migrate",
		)
		if err != nil {
			return errors.WithMessage(err, "failed to define php command and args")
		}

		cmd = exec.Command(cmdName, args...)
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
	cmdName, args, err := packagemanager.DefinePHPCommandAndArgs(
		filepath.Join(path, "artisan"), "key:generate", "--force",
	)
	if err != nil {
		return errors.WithMessage(err, "failed to define php command and args")
	}

	cmd := exec.Command(cmdName, args...)

	log.Println('\n', cmd.String())
	cmd.Dir = path
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	err = cmd.Run()
	if err != nil {
		return errors.WithMessage(err, "failed to clear config cache")
	}

	cmdName, args, err = packagemanager.DefinePHPCommandAndArgs(
		filepath.Join(path, "artisan"), "view:cache",
	)
	if err != nil {
		return errors.WithMessage(err, "failed to define php command and args")
	}

	cmd = exec.Command(cmdName, args...)
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

func ChangePassword(_ context.Context, path, username, password string) error {
	cmdName, args, err := packagemanager.DefinePHPCommandAndArgs(
		filepath.Join(path, "artisan"), "user:change-password", username, password,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to define php command and args")
	}

	cmd := exec.Command(cmdName, args...)

	log.Println('\n', fmt.Sprintf("php artisan user:change-password %s ********", username))
	cmd.Dir = path
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	err = cmd.Run()
	if err != nil {
		return errors.WithMessage(err, "failed to execute artisan command")
	}

	return nil
}

func SetDaemonCreateToken(_ context.Context, path, token string) error {
	cmdName, args, err := packagemanager.DefinePHPCommandAndArgs(
		filepath.Join(path, "artisan"),
		"tinker",
		"--execute",
		fmt.Sprintf(
			"Illuminate\\Support\\Facades\\Cache::put('gdaemonAutoCreateToken', '%s', 9999);",
			token,
		),
	)
	if err != nil {
		return errors.WithMessage(err, "failed to define php command and args")
	}

	err = utils.ExecCommand(cmdName, args...)
	if err != nil {
		return errors.WithMessage(err, "failed to execute tinker command")
	}

	return nil
}

func UpgradeGames(_ context.Context, path string) error {
	cmdName, args, err := packagemanager.DefinePHPCommandAndArgs(
		filepath.Join(path, "artisan"), "games:upgrade",
	)
	if err != nil {
		return errors.WithMessage(err, "failed to define php command and args")
	}

	err = utils.ExecCommand(cmdName, args...)
	if err != nil {
		return errors.WithMessage(err, "failed to execute artisan command")
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
