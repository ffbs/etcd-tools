package signify

import (
	"bytes"
	"os/exec"
	"strings"
)

var SIGNIFY_COMMAND string = "signify-openbsd"

type Cmdline struct {
	PrivateKey string
}

func (sc Cmdline) Sign(content []byte) (string, error) {
	var signature bytes.Buffer
	cmd := exec.Command(SIGNIFY_COMMAND, "-S", "-m-", "-s", sc.PrivateKey, "-x-")
	cmd.Stdout = &signature
	w, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	if _, err := w.Write(content); err != nil {
		return "", err
	}
	w.Close()

	if err := cmd.Wait(); err != nil {
		return "", err
	}

	return strings.SplitN(string(signature.Bytes()), "\n", 3)[1], nil
}