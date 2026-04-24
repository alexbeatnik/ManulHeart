// ManulHeart driver — CLI entry point.
//
// Usage:
//
//	manul <hunt-file>                 run a single hunt file
//	manul <directory>                 run all .hunt files in the directory
//	manul .                           run all .hunt files in the current directory
//	manul run <hunt-file> [flags]     explicit run subcommand (same as above)
//	manul run-step '<DSL command>'    execute a single DSL command
//
// Examples:
//
//	manul examples/saucedemo.hunt
//	manul examples/saucedemo.hunt --headless
//	manul examples/
//	manul .
//	manul run examples/saucedemo.hunt --cdp http://127.0.0.1:9222
//	manul run-step "Click the 'Login' button" --cdp http://127.0.0.1:9222
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/manulengineer/manulheart/pkg/browser"
	"github.com/manulengineer/manulheart/pkg/config"
	"github.com/manulengineer/manulheart/pkg/dsl"
	"github.com/manulengineer/manulheart/pkg/report"
	"github.com/manulengineer/manulheart/pkg/runtime"
	"github.com/manulengineer/manulheart/pkg/utils"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	firstArg := os.Args[1]

	if firstArg == "--version" || firstArg == "-version" || firstArg == "-v" {
		fmt.Printf("manul-heart v0.0.9.29 (core 0.0.1.0)\n")
		os.Stdout.Sync()
		return
	}

	// If the first arg is a flag (starts with -), or it's a known target-like string,
	// treat the whole execution as an implicit "run".
	if strings.HasPrefix(firstArg, "-") || looksLikeTarget(firstArg) {
		if err := cmdRun(os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	switch firstArg {
	case "run":
		if err := cmdRun(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "run-step":
		if err := cmdRunStep(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %q\n\n", firstArg)
		printUsage()
		os.Exit(1)
	}
}

// looksLikeTarget returns true if the argument appears to be a .hunt file,
// a directory path, or "." — i.e. something to run directly.
func looksLikeTarget(arg string) bool {
	if arg == "." {
		return true
	}
	if strings.HasSuffix(arg, ".hunt") {
		return true
	}
	// Check if it's an existing directory.
	info, err := os.Stat(arg)
	if err == nil && info.IsDir() {
		return true
	}
	return false
}

// ── run subcommand ────────────────────────────────────────────────────────────

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cdpEndpoint := fs.String("cdp", "", "CDP endpoint URL (if set, skip auto-launch and connect to existing browser)")
	verbose := fs.Bool("verbose", false, "enable verbose logging")
	jsonOut := fs.Bool("json", false, "print JSON result to stdout")
	timeout := fs.Duration("timeout", 30*time.Second, "default command timeout")
	userDataDir := fs.String("user-data-dir", "", "Chrome profile directory (empty = unique temp dir per run)")
	headless := fs.Bool("headless", false, "run Chrome in headless mode")
	debug := fs.Bool("debug", false, "enable debug mode (pause on each step)")
	explainMode := fs.Bool("explain", false, "enable explain mode (show targeting candidates)")
	screenshot := fs.String("screenshot", "none", "screenshot mode: none, on-fail, always")
	htmlReport := fs.Bool("html-report", false, "generate HTML report after run")
	tags := fs.String("tags", "", "comma-separated tags to filter hunt files")
	retries := fs.Int("retries", 0, "number of retries for failed steps")
	disableCache := fs.Bool("disable-cache", false, "disable DOM snapshot caching")
	_ = fs.Int("workers", 1, "number of parallel workers (placeholder for compatibility)")
	_ = fs.String("browser", "chromium", "browser type (default: chromium)")
	breakLinesStr := fs.String("break-lines", "", "comma-separated line numbers to pause on (debugging)")
	showVersion := fs.Bool("version", false, "show engine version and exit")

	var target string
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: manul <hunt-file|directory> [flags]\n\nFlags:\n")
		fs.PrintDefaults()
	}

	remaining := args
	for len(remaining) > 0 {
		if err := fs.Parse(remaining); err != nil {
			return err
		}
		if fs.NArg() > 0 {
			if target == "" {
				target = fs.Arg(0)
			}
			remaining = fs.Args()[1:]
		} else {
			remaining = nil
		}
	}

	if *showVersion {
		fmt.Printf("manul-heart v0.0.9.29 (core 0.0.1.0)\n")
		os.Stdout.Sync()
		return nil
	}
 
	if target == "" {
		fs.Usage()
		return fmt.Errorf("hunt file or directory path is required")
	}

	// Collect .hunt files from target.
	huntFiles, err := collectHuntFiles(target)
	if err != nil {
		return err
	}
	if len(huntFiles) == 0 {
		return fmt.Errorf("no .hunt files found in %q", target)
	}

	cfg := config.Default()
	
	// Load environment variable defaults (priority: CLI > ENV > Default)
	if os.Getenv("MANUL_HEADLESS") == "true" {
		cfg.Headless = true // I need to add Headless to Config struct too
	}
	if t := os.Getenv("MANUL_TIMEOUT"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			cfg.DefaultTimeout = d
		} else if i, err := strconv.Atoi(t); err == nil {
			cfg.DefaultTimeout = time.Duration(i) * time.Millisecond
		}
	}
	if os.Getenv("MANUL_EXPLAIN") == "true" {
		cfg.ExplainMode = true
	}
	if s := os.Getenv("MANUL_SCREENSHOT"); s != "" {
		cfg.Screenshot = s
	}

	cfg.Verbose = *verbose
	if *timeout != 30*time.Second { // only override if user provided a flag
		cfg.DefaultTimeout = *timeout
	}
	cfg.DebugMode = *debug
	if *explainMode {
		cfg.ExplainMode = true
	}
	if *screenshot != "none" {
		cfg.Screenshot = *screenshot
	}
	cfg.HTMLReport = *htmlReport
	cfg.Retries = *retries
	cfg.DisableCache = *disableCache
	if *tags != "" {
		cfg.Tags = strings.Split(*tags, ",")
		for i := range cfg.Tags {
			cfg.Tags[i] = strings.TrimSpace(cfg.Tags[i])
		}
	}
	if *breakLinesStr != "" {
		cfg.DebugMode = true
		for _, part := range strings.Split(*breakLinesStr, ",") {
			if ln, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				cfg.BreakLines = append(cfg.BreakLines, ln)
			}
		}
	}

	logLevel := utils.LogLevelInfo
	if *verbose {
		logLevel = utils.LogLevelDebug
	}
	logger := utils.NewLogger(nil).WithLevel(logLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Browser lifecycle ─────────────────────────────────────────────
	var chrome *browser.ChromeProcess

	if *cdpEndpoint != "" {
		cfg.CDPEndpoint = *cdpEndpoint
	} else {
		opts := browser.DefaultChromeOptions()
		opts.UserDataDir = *userDataDir // empty string → unique temp dir
		if *headless {
			cfg.Headless = true
		}
		opts.Headless = cfg.Headless

		logger.Info("Launching Chrome (port %d, profile %s)…", opts.Port, opts.UserDataDir)
		chrome, err = browser.LaunchChrome(ctx, opts)
		if err != nil {
			return fmt.Errorf("launch chrome: %w", err)
		}

		// Use sync.Once so Close is safe to call from both defer and signal goroutine.
		var closeOnce sync.Once
		closeChrome := func() {
			closeOnce.Do(func() {
				logger.Debug("Closing Chrome…")
				chrome.Close()
			})
		}
		defer closeChrome()

		// Cancel context on SIGINT/SIGTERM so running commands abort cleanly,
		// then close Chrome. Avoids os.Exit(1) which bypasses defers.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			logger.Debug("Signal received, cancelling…")
			cancel()
			closeChrome()
		}()

		cfg.CDPEndpoint = chrome.Endpoint()
	}

	fmt.Fprintf(os.Stderr, "😼 Manul: found %d hunt file(s)\n", len(huntFiles))

	var totalFailed int
	for i, huntFile := range huntFiles {
		filename := filepath.Base(huntFile)
		if len(huntFiles) > 1 {
			fmt.Fprintf(os.Stderr, "\n%s\n📜 [%d/%d] %s\n%s\n",
				strings.Repeat("=", 60), i+1, len(huntFiles), filename, strings.Repeat("=", 60))
		} else {
			fmt.Fprintf(os.Stderr, "\n%s\n📜 %s\n%s\n",
				strings.Repeat("=", 60), filename, strings.Repeat("=", 60))
		}

		hunt, err := dsl.ParseFile(huntFile)
		if err != nil {
			logger.Error("parse %q: %v", huntFile, err)
			totalFailed++
			continue
		}

		// Resolve @import directives and expand USE blocks
		if err := dsl.ResolveImports(hunt); err != nil {
			logger.Error("imports %q: %v", huntFile, err)
			totalFailed++
			continue
		}
		if err := hunt.Expand(); err != nil {
			logger.Error("expand %q: %v", huntFile, err)
			totalFailed++
			continue
		}

		logger.Info("ManulHeart — %s", huntFile)
		if hunt.Title != "" {
			logger.Info("Title: %s", hunt.Title)
		}
		logger.Info("Commands: %d", len(hunt.Commands))
		logger.Info("CDP: %s", cfg.CDPEndpoint)

		b := browser.NewCDPBrowser(cfg.CDPEndpoint)
		page, err := b.FirstPage(ctx)
		if err != nil {
			return fmt.Errorf("connect to browser at %q: %w", cfg.CDPEndpoint, err)
		}

		// Scope the page lifetime to this iteration so connections do not accumulate.
		func() {
			defer page.Close()
			rt := runtime.New(cfg, page, logger)
			result, err := rt.RunHunt(ctx, hunt)
			if err != nil {
				logger.Error("hunt %q failed: %v", huntFile, err)
				totalFailed++
				return
			}

			printResult(result, *jsonOut, logger)

			if hErr := report.AppendRunHistory("reports", result); hErr != nil {
				logger.Warn("run_history append failed: %v", hErr)
			}

			// Generate HTML report if requested
			if cfg.HTMLReport {
				reportPath, rErr := report.GenerateHTML(result, "reports")
				if rErr != nil {
					logger.Warn("HTML report generation failed: %v", rErr)
				} else {
					logger.Info("📊 HTML report: %s", reportPath)
				}
			}

			if !result.Success {
				totalFailed++
			}
		}()
	}

	if totalFailed > 0 {
		return fmt.Errorf("%d/%d hunt file(s) failed", totalFailed, len(huntFiles))
	}
	return nil
}

// collectHuntFiles resolves a target path to a list of .hunt files.
func collectHuntFiles(target string) ([]string, error) {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return nil, fmt.Errorf("resolve path %q: %w", target, err)
	}

	info, err := os.Stat(absTarget)
	if err != nil {
		return nil, fmt.Errorf("path not found: %s", target)
	}

	if !info.IsDir() {
		if !strings.HasSuffix(absTarget, ".hunt") {
			return nil, fmt.Errorf("not a .hunt file: %s", target)
		}
		return []string{absTarget}, nil
	}

	// Collect all .hunt files in the directory (non-recursive).
	entries, err := os.ReadDir(absTarget)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", target, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".hunt") {
			files = append(files, filepath.Join(absTarget, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

// ── run-step subcommand ───────────────────────────────────────────────────────

func cmdRunStep(args []string) error {
	fs := flag.NewFlagSet("run-step", flag.ExitOnError)
	cdpEndpoint := fs.String("cdp", "http://127.0.0.1:9222", "CDP endpoint URL")
	verbose := fs.Bool("verbose", false, "enable verbose logging")
	jsonOut := fs.Bool("json", false, "print JSON result to stdout")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: driver run-step '<command>' [flags]\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	step := fs.Arg(0)
	if step == "" {
		fs.Usage()
		return fmt.Errorf("DSL command is required")
	}

	cfg := config.Default()
	cfg.CDPEndpoint = *cdpEndpoint
	cfg.Verbose = *verbose

	logLevel := utils.LogLevelInfo
	if *verbose {
		logLevel = utils.LogLevelDebug
	}
	logger := utils.NewLogger(nil).WithLevel(logLevel)

	ctx := context.Background()

	b := browser.NewCDPBrowser(cfg.CDPEndpoint)
	page, err := b.FirstPage(ctx)
	if err != nil {
		return fmt.Errorf("connect to browser at %q: %w", cfg.CDPEndpoint, err)
	}
	defer page.Close()

	rt := runtime.New(cfg, page, logger)
	result, err := rt.RunStep(ctx, step)
	if err != nil {
		return err
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
		os.Stdout.Sync()
	} else {
		if result.Success {
			logger.Info("✓ %s", step)
		} else {
			logger.Error("✗ %s → %s", step, result.Error)
			return fmt.Errorf("step failed: %s", result.Error)
		}
	}

	return nil
}

// ── Output helpers ────────────────────────────────────────────────────────────

func printResult(result any, asJSON bool, logger *utils.Logger) {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
		os.Stdout.Sync()
		return
	}

	data, _ := json.Marshal(result)
	var s struct {
		TotalSteps      int   `json:"total_steps"`
		Passed          int   `json:"passed"`
		Failed          int   `json:"failed"`
		TotalDurationMS int64 `json:"total_duration_ms"`
		Success         bool  `json:"success"`
	}
	json.Unmarshal(data, &s)

	if s.Success {
		logger.Info("✓ All %d steps passed (%dms)", s.TotalSteps, s.TotalDurationMS)
		fmt.Fprintln(os.Stderr, "RESULT: PASS")
	} else {
		logger.Error("✗ %d/%d steps failed (%dms)", s.Failed, s.TotalSteps, s.TotalDurationMS)
		fmt.Fprintln(os.Stderr, "RESULT: FAIL")
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `ManulHeart — deterministic DSL-first browser automation (Go)

Usage:
  manul <target> [flags]               Run .hunt files in target (file or directory)
  manul run <target> [flags]           Explicit run subcommand
  manul run-step '<command>' [flags]   Execute a single DSL command

Core Flags:
  --cdp URL           Connect to existing Chrome (skip auto-launch)
  --user-data-dir DIR Chrome profile directory (default: unique temp dir per run)
  --headless          Run Chrome in headless mode
  --verbose           Enable verbose debug logging
  --json              Output structured JSON result to stdout
  --timeout DURATION  Per-command timeout (default: 30s)
  --tags TAGS         Filter hunt files by tags (comma-separated)
  --retries N         Number of retries for failed steps
  --screenshot MODE   Screenshot mode: none, on-fail, always (default: none)
  --html-report       Generate HTML report after run
  --explain           Show targeting candidates (explain mode)

Compatibility Flags:
  --workers N         Parallel workers (default: 1)
  --browser TYPE      Browser type (default: chromium)
  --break-lines L     Pause at specified line numbers (debugging)

Examples:
  manul examples/saucedemo.hunt
  manul examples/saucedemo.hunt --headless
  manul examples/
  manul .
  manul run-step "Click the 'Login' button" --cdp http://127.0.0.1:9222
`)
}
