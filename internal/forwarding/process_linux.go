package forwarding

import (
	"os/exec"
	"syscall"
)

func configureProcess(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL} }

func ownProcess(c *exec.Cmd) (func(), error) { return func() {}, nil }
