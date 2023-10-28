package packagemanager

import (
	"fmt"
	"os/exec"
	"testing"
)

func Test(_ *testing.T) {
	p, _ := exec.LookPath("php")

	fmt.Println(p)
}
