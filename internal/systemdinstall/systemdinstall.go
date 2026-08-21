package systemdinstall

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

const (
	ServiceUnitPath = "/etc/systemd/system/cloudflare-ddns-agent.service"
	TimerUnitPath   = "/etc/systemd/system/cloudflare-ddns-agent.timer"
	BinaryPath      = "/usr/local/bin/cloudflare-ddns-agent"
)

func GenerateServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=cloudflare-ddns-agent update

[Service]
Type=oneshot
ExecStart=%s update
`, BinaryPath)
}

func GenerateTimerUnit(interval time.Duration) string {
	return fmt.Sprintf(`[Unit]
Description=Executa cloudflare-ddns-agent periodicamente

[Timer]
OnBootSec=10s
OnUnitActiveSec=%s
AccuracySec=1s

[Install]
WantedBy=timers.target
`, formatSeconds(interval))
}

func formatSeconds(d time.Duration) string {
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// Runner executes an external command by name with args.
type Runner func(name string, args ...string) error

func execRunner(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Install writes the service and timer unit files and enables the timer.
func Install(interval time.Duration) error {
	return installWith(interval, os.WriteFile, execRunner)
}

func installWith(interval time.Duration, writeFile func(string, []byte, os.FileMode) error, run Runner) error {
	if err := writeFile(ServiceUnitPath, []byte(GenerateServiceUnit()), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", ServiceUnitPath, err)
	}
	if err := writeFile(TimerUnitPath, []byte(GenerateTimerUnit(interval)), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", TimerUnitPath, err)
	}
	if err := run("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	if err := run("systemctl", "enable", "--now", "cloudflare-ddns-agent.timer"); err != nil {
		return fmt.Errorf("systemctl enable --now cloudflare-ddns-agent.timer: %w", err)
	}
	return nil
}

// Uninstall disables the timer and removes the unit files.
func Uninstall() error {
	return uninstallWith(execRunner, os.Remove)
}

func uninstallWith(run Runner, remove func(string) error) error {
	_ = run("systemctl", "disable", "--now", "cloudflare-ddns-agent.timer")
	if err := remove(ServiceUnitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", ServiceUnitPath, err)
	}
	if err := remove(TimerUnitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", TimerUnitPath, err)
	}
	if err := run("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	return nil
}
