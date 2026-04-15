//go:build !windows

package browser

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcGroup starts the command in its own process group so we can kill
// all child processes (Chrome spawns many) with a single signal.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// Close terminates the Chrome process and all its children via process group kill.
func (cp *ChromeProcess) Close() error {
	if cp.cmd == nil || cp.cmd.Process == nil {
		return nil
	}
	pid := cp.cmd.Process.Pid
	// Kill the entire process group (negative PID).
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_ = cp.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-done
	}
	return nil
}
