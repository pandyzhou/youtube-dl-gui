//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
)

func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// tree is the program's process group, which also holds what it starts.
type tree struct {
	pgid int
}

func attach(cmd *exec.Cmd) (*tree, error) {
	return &tree{pgid: cmd.Process.Pid}, nil
}

func (t *tree) kill() { _ = syscall.Kill(-t.pgid, syscall.SIGKILL) }

func (t *tree) close() {}
