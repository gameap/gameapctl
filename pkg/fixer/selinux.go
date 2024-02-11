package fixer

import (
	"context"
	"os/exec"
	"strings"

	"github.com/gameap/gameapctl/pkg/utils"
	"github.com/pkg/errors"
)

func IsSELinuxEnabled(_ context.Context) (bool, error) {
	if !utils.IsCommandAvailable("getenforce") {
		return false, nil
	}

	output, err := utils.ExecCommandWithOutput("getenforce")
	if err != nil {
		return false, errors.WithMessage(err, "failed to getenforce")
	}

	status := strings.TrimSpace(output)

	if status == "Enforcing" {
		return true, nil
	}

	return false, nil
}

func DisableSELinux(ctx context.Context) error {
	err := exec.Command("setenforce", "0").Run()
	if err != nil {
		return errors.WithMessage(err, "failed to setenforce")
	}

	err = utils.FindLineAndReplace(ctx, "/etc/selinux/config", map[string]string{
		"SELINUX=enforcing": "SELINUX=permissive",
	})
	if err != nil {
		return errors.WithMessage(err, "failed to update /etc/selinux/config config")
	}

	return nil
}
