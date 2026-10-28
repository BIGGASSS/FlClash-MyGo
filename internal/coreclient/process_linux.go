//go:build linux

package coreclient

import (
	"io"
	"os"
	"os/exec"
	"strconv"
)

func startChild(path, address string, stderr io.Writer, elevated bool) (*child, error) {
	if !elevated || os.Geteuid() == 0 {
		return commandChild(exec.Command(path, address), stderr)
	}
	return commandChild(exec.Command("pkexec", "env", "FLCLASH_OWNER_UID="+strconv.Itoa(os.Getuid()), "FLCLASH_OWNER_GID="+strconv.Itoa(os.Getgid()), path, address), stderr)
}
