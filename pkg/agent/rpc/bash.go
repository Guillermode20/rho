package rpc

import (
	"bytes"
	"os/exec"
)

// executeBash runs a shell command and returns its combined output.
// Returns (stdout, error) where error wraps stderr if the command failed.
func executeBash(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return out.String(), err
	}
	return out.String(), nil
}
