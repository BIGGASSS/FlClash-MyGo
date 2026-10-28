package coreclient

import (
	"io"
	"os/exec"
)

type child struct {
	done chan struct{}
	kill func() error
}

func commandChild(cmd *exec.Cmd, stderr io.Writer) (*child, error) {
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &child{done: make(chan struct{}), kill: cmd.Process.Kill}
	go func() { _ = cmd.Wait(); close(c.done) }()
	return c, nil
}
