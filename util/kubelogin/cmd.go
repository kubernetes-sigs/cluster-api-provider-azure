package kubelogin

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"os"
	"os/exec"
)

// ExecCommand executes a command and returns the error if the command fails to run.
func ExecCommand(cmdName string, cmdArgs ...string) (err error) {
	path, err := exec.LookPath(cmdName)
	if err != nil {
		return
	}
	cmd := exec.Command(path, cmdArgs...)
	var errb bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &errb
	cmd.Env = []string{fmt.Sprintf("HOME=%s", os.Getenv("HOME")), fmt.Sprintf("PATH=%s", os.Getenv("PATH"))}
	err = cmd.Run()
	if err != nil {
		return errors.New(errb.String())
	}
	return nil
}
