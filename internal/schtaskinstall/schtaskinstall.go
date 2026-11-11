// Package schtaskinstall registers the agent as a Windows Task Scheduler task.
//
// The task runs "cloudflare-ddns-agent run" (a long-lived loop) as SYSTEM:
// it starts at boot, needs no logged-on user, and a repeating trigger acts
// as a watchdog that relaunches the loop if the process ever dies
// (MultipleInstancesPolicy=IgnoreNew keeps it to a single instance).
package schtaskinstall

import (
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf16"
)

const (
	TaskName = "cloudflare-ddns-agent"
	// Watchdog is how often Task Scheduler re-checks that the loop is running.
	Watchdog = 5 * time.Minute
)

// GenerateTaskXML returns the Task Scheduler definition for binaryPath.
func GenerateTaskXML(binaryPath string, watchdog time.Duration) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Keeps Cloudflare DNS A records pointed at this machine's public IP (cloudflare-ddns-agent).</Description>
    <URI>\%s</URI>
  </RegistrationInfo>
  <Triggers>
    <BootTrigger>
      <Enabled>true</Enabled>
      <Delay>PT30S</Delay>
    </BootTrigger>
    <TimeTrigger>
      <StartBoundary>2026-01-01T00:00:00</StartBoundary>
      <Enabled>true</Enabled>
      <Repetition>
        <Interval>%s</Interval>
        <StopAtDurationEnd>false</StopAtDurationEnd>
      </Repetition>
    </TimeTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>S-1-5-18</UserId>
      <RunLevel>HighestAvailable</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>999</Count>
    </RestartOnFailure>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>%s</Command>
      <Arguments>run</Arguments>
    </Exec>
  </Actions>
</Task>
`, TaskName, isoMinutes(watchdog), escape(binaryPath))
}

// isoMinutes formats d as an ISO-8601 duration in whole minutes. Task
// Scheduler rejects repetition intervals under one minute.
func isoMinutes(d time.Duration) string {
	m := int(d / time.Minute)
	if m < 1 {
		m = 1
	}
	return fmt.Sprintf("PT%dM", m)
}

func escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// EncodeUTF16LE encodes s as UTF-16LE with a byte-order mark, the encoding
// schtasks expects for /XML files.
func EncodeUTF16LE(s string) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 2+2*len(units))
	out[0], out[1] = 0xFF, 0xFE
	for i, u := range units {
		binary.LittleEndian.PutUint16(out[2+2*i:], u)
	}
	return out
}

// Runner executes an external command by name with args.
type Runner func(name string, args ...string) error

// TempWriter stores data in a temporary file and returns its path plus a
// cleanup function.
type TempWriter func(data []byte) (path string, cleanup func(), err error)

func execRunner(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// execQuiet discards output: "schtasks /End" on a task that does not exist
// yet prints a scary "ERROR: file not found" on every first install.
func execQuiet(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func osTempWriter(data []byte) (string, func(), error) {
	f, err := os.CreateTemp("", "cloudflare-ddns-agent-*.xml")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.Remove(f.Name()) }
	if _, err := f.Write(data); err != nil {
		f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return f.Name(), cleanup, nil
}

// Install registers (or replaces) the scheduled task and starts it now.
func Install(binaryPath string) error {
	return installWith(binaryPath, osTempWriter, execRunner, execQuiet)
}

func installWith(binaryPath string, writeTemp TempWriter, run, quiet Runner) error {
	path, cleanup, err := writeTemp(EncodeUTF16LE(GenerateTaskXML(binaryPath, Watchdog)))
	if err != nil {
		return fmt.Errorf("writing task definition: %w", err)
	}
	defer cleanup()

	// Stop a running loop first so a replaced binary takes effect right away.
	_ = quiet("schtasks", "/End", "/TN", TaskName)
	if err := run("schtasks", "/Create", "/TN", TaskName, "/XML", path, "/F"); err != nil {
		return fmt.Errorf("schtasks /Create: %w", err)
	}
	if err := run("schtasks", "/Run", "/TN", TaskName); err != nil {
		return fmt.Errorf("schtasks /Run: %w", err)
	}
	return nil
}

// Uninstall stops and deletes the scheduled task.
func Uninstall() error {
	return uninstallWith(execRunner, execQuiet)
}

func uninstallWith(run, quiet Runner) error {
	_ = quiet("schtasks", "/End", "/TN", TaskName)
	if err := run("schtasks", "/Delete", "/TN", TaskName, "/F"); err != nil {
		return fmt.Errorf("schtasks /Delete: %w", err)
	}
	return nil
}
