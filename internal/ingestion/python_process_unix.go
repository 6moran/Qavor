//go:build !windows

package ingestion

import (
	"os/exec"
	"syscall"
)

type parserProcessController interface {
	terminate() error
	close() error
}

type unixParserProcessController struct {
	pgid int
}

func configureParserProcess(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

func attachParserProcess(cmd *exec.Cmd) (parserProcessController, error) {
	return unixParserProcessController{pgid: cmd.Process.Pid}, nil
}

func (p unixParserProcessController) terminate() error {
	return syscall.Kill(-p.pgid, syscall.SIGKILL)
}

func (unixParserProcessController) close() error { return nil }
