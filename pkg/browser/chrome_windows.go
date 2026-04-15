//go:build windows

package browser

import (
	"os/exec"
	"time"
)

// setProcGroup is a no-op on Windows (process groups work differently).
func setProcGroup(cmd *exec.Cmd) {
	// Windows does not use Unix process groups.
	// Chrome child processes will be terminated via Process.Kill().
}

// Close terminates the Chrome process on Windows.
func (cp *ChromeProcess) Close() error {
	if cp.cmd == nil || cp.cmd.Process == nil {
		return nil
	}
	_ = cp.cmd.Process.Kill()
	done := make(chan struct{})
	go func() {
		_ = cp.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
	return nil
}
