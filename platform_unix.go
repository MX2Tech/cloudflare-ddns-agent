//go:build !windows

package main

import (
	"io"
	"os"
	"time"

	"github.com/MX2Tech/cloudflare-ddns-agent/internal/systemdinstall"
)

const (
	installDescription   = "Install and enable the systemd timer"
	uninstallDescription = "Stop and remove the systemd timer"
	installedMessage     = "cloudflare-ddns-agent installed and timer enabled"
	uninstalledMessage   = "cloudflare-ddns-agent timer removed"
)

func defaultConfigPath() string { return "/etc/cloudflare-ddns-agent/config.yaml" }

// On Linux "run" is only for interactive use (the systemd timer calls
// "update"), so it logs to stdout and journald/the terminal keeps it.
func openRunLog() (io.Writer, func(), error) { return os.Stdout, func() {}, nil }

func installService(interval time.Duration) error { return systemdinstall.Install(interval) }

func uninstallService() error { return systemdinstall.Uninstall() }
