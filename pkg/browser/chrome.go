// Package browser — Chrome process lifecycle management.
package browser

import (
	"context"
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ChromeProcess manages a Chrome browser process spawned for automation.
type ChromeProcess struct {
	cmd         *exec.Cmd
	port        int
	userDataDir string
}

// ChromeOptions configures the Chrome process to spawn.
type ChromeOptions struct {
	// Port for Chrome's remote debugging protocol. Default: 9222.
	Port int
	// UserDataDir is the Chrome profile directory. Default: /tmp/manulheart-chrome.
	UserDataDir string
	// DisableGPU disables GPU acceleration. Default: true.
	DisableGPU bool
	// Headless runs Chrome without a visible window.
	Headless bool
}

// DefaultChromeOptions returns sensible defaults for automation.
func DefaultChromeOptions() ChromeOptions {
	return ChromeOptions{
		Port:        9222,
		UserDataDir: "/tmp/manulheart-chrome",
		DisableGPU:  true,
		Headless:    false,
	}
}

// LaunchChrome starts a Chrome process with remote debugging enabled.
// It blocks until Chrome's CDP endpoint is reachable (or context expires).
func LaunchChrome(ctx context.Context, opts ChromeOptions) (*ChromeProcess, error) {
	chromePath, err := findChrome()
	if err != nil {
		return nil, err
	}

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", opts.Port),
		"--no-first-run",
		"--no-default-browser-check",
		fmt.Sprintf("--user-data-dir=%s", opts.UserDataDir),
	}
	if opts.DisableGPU {
		args = append(args, "--disable-gpu")
	}
	if opts.Headless {
		args = append(args, "--headless=new")
	}

	cmd := exec.CommandContext(ctx, chromePath, args...)
	// Detach stdout/stderr — Chrome is noisy by default.
	cmd.Stdout = nil
	cmd.Stderr = nil
	// Inherit environment (required for DISPLAY on Linux).
	cmd.Env = os.Environ()
	// Platform-specific process group setup (implemented in chrome_unix.go / chrome_windows.go).
	setProcGroup(cmd)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start chrome: %w", err)
	}

	cp := &ChromeProcess{
		cmd:         cmd,
		port:        opts.Port,
		userDataDir: opts.UserDataDir,
	}

	// Wait for CDP endpoint to become reachable.
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", opts.Port)
	if err := waitForCDP(ctx, endpoint, 15*time.Second); err != nil {
		// Chrome started but CDP never became reachable — kill it.
		_ = cp.Close()
		return nil, fmt.Errorf("chrome started but CDP not reachable at %s: %w", endpoint, err)
	}

	return cp, nil
}

// Endpoint returns the HTTP CDP endpoint URL.
func (cp *ChromeProcess) Endpoint() string {
	return fmt.Sprintf("http://127.0.0.1:%d", cp.port)
}

// findChrome searches for a Chrome binary in common locations.
func findChrome() (string, error) {
	var candidates []string
	switch runtime.GOOS {
	case "linux":
		candidates = []string{
			"google-chrome-stable",
			"google-chrome",
			"chromium-browser",
			"chromium",
		}
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"google-chrome",
			"chromium",
		}
	case "windows":
		candidates = []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		}
	default:
		candidates = []string{"google-chrome", "chromium"}
	}

	for _, c := range candidates {
		// Absolute path — check existence directly.
		if strings.Contains(c, "/") || strings.Contains(c, `\`) {
			if _, err := os.Stat(c); err == nil {
				return c, nil
			}
			continue
		}
		// Short name — look up in PATH.
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("chrome not found; install Google Chrome or set it in PATH")
}

// waitForCDP polls the CDP /json endpoint until it responds or timeout expires.
func waitForCDP(ctx context.Context, endpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := endpoint + "/json"
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Quick TCP check first (cheaper than full HTTP).
		u, _ := neturl.Parse(endpoint)
		conn, err := net.DialTimeout("tcp", u.Host, 500*time.Millisecond)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		conn.Close()

		// TCP is open — verify CDP responds.
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for CDP at %s", endpoint)
}
