package packagemanager

import (
	"fmt"
	"os/exec"
	"testing"
)

func Test(t *testing.T) {
	p, _ := exec.LookPath("php")

	fmt.Println(p)
}
