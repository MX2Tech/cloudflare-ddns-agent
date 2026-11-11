//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/MX2Tech/cloudflare-ddns-agent/internal/logfile"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/schtaskinstall"
)

const (
	installDescription   = "Register the scheduled task (SYSTEM, at boot, self-restarting) and start it"
	uninstallDescription = "Stop and delete the scheduled task"
	installedMessage     = "cloudflare-ddns-agent installed and scheduled task started"
	uninstalledMessage   = "cloudflare-ddns-agent scheduled task removed"
	maxLogBytes          = 5 << 20
)

func dataDir() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	return filepath.Join(base, "cloudflare-ddns-agent")
}

func defaultConfigPath() string { return filepath.Join(dataDir(), "config.yaml") }

// Task Scheduler discards stdout, so the loop writes to a capped log file
// next to the config.
func openRunLog() (io.Writer, func(), error) {
	f, err := logfile.Open(filepath.Join(dataDir(), "agent.log"), maxLogBytes)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// The loop re-reads check_interval from the config every cycle, so the
// task definition does not depend on it.
func installService(_ time.Duration) error {
	if err := restrictDataDir(); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating agent binary: %w", err)
	}
	return schtaskinstall.Install(exe)
}

func uninstallService() error { return schtaskinstall.Uninstall() }

// restrictDataDir leaves the config directory (it holds the API token)
// readable only by SYSTEM and Administrators. ProgramData is readable by
// every local user by default. SIDs keep it independent of the Windows
// display language ("Administradores", "SISTEMA").
func restrictDataDir() error {
	cmd := exec.Command("icacls", dataDir(), "/inheritance:r",
		"/grant:r", "*S-1-5-18:(OI)(CI)F", "*S-1-5-32-544:(OI)(CI)F")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restricting permissions on %s: %w: %s", dataDir(), err, out)
	}
	return nil
}
