//go:build darwin

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
	script := `on run argv
do shell script ("exec env FLCLASH_OWNER_UID=" & item 3 of argv & " FLCLASH_OWNER_GID=" & item 4 of argv & " " & quoted form of item 1 of argv & " " & quoted form of item 2 of argv) with administrator privileges
end run`
	return commandChild(exec.Command("/usr/bin/osascript", "-e", script, path, address, strconv.Itoa(os.Getuid()), strconv.Itoa(os.Getgid())), stderr)
}
