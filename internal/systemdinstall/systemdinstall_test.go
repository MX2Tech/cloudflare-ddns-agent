package systemdinstall

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGenerateServiceUnit(t *testing.T) {
	unit := GenerateServiceUnit()
	if !strings.Contains(unit, "ExecStart=/usr/local/bin/cloudflare-ddns-agent update") {
		t.Errorf("service unit missing ExecStart line:\n%s", unit)
	}
	if !strings.Contains(unit, "Type=oneshot") {
		t.Errorf("service unit missing Type=oneshot:\n%s", unit)
	}
}

func TestGenerateTimerUnit(t *testing.T) {
	unit := GenerateTimerUnit(30 * time.Second)
	if !strings.Contains(unit, "OnUnitActiveSec=30s") {
		t.Errorf("timer unit missing correct interval:\n%s", unit)
	}
	if !strings.Contains(unit, "WantedBy=timers.target") {
		t.Errorf("timer unit missing WantedBy:\n%s", unit)
	}
}

func TestGenerateTimerUnit_DifferentInterval(t *testing.T) {
	unit := GenerateTimerUnit(90 * time.Second)
	if !strings.Contains(unit, "OnUnitActiveSec=90s") {
		t.Errorf("timer unit missing correct interval:\n%s", unit)
	}
}

func TestInstallWith_WritesUnitsAndEnablesTimer(t *testing.T) {
	var written []string
	var ran [][]string

	writeFile := func(path string, data []byte, perm os.FileMode) error {
		written = append(written, path)
		return nil
	}
	run := func(name string, args ...string) error {
		ran = append(ran, append([]string{name}, args...))
		return nil
	}

	if err := installWith(30*time.Second, writeFile, run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(written) != 2 {
		t.Fatalf("expected 2 files written, got %d: %v", len(written), written)
	}
	if len(ran) != 2 {
		t.Fatalf("expected 2 commands run, got %d: %v", len(ran), ran)
	}
	if ran[1][1] != "enable" {
		t.Errorf("expected second command to enable the timer, got %v", ran[1])
	}
}

func TestInstallWith_PropagatesWriteError(t *testing.T) {
	writeFile := func(path string, data []byte, perm os.FileMode) error {
		return errors.New("permission denied")
	}
	run := func(name string, args ...string) error { return nil }

	if err := installWith(30*time.Second, writeFile, run); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUninstallWith_RemovesUnitsAndDisablesTimer(t *testing.T) {
	var removed []string
	var ran [][]string

	remove := func(path string) error {
		removed = append(removed, path)
		return nil
	}
	run := func(name string, args ...string) error {
		ran = append(ran, append([]string{name}, args...))
		return nil
	}

	if err := uninstallWith(run, remove); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(removed) != 2 {
		t.Fatalf("expected 2 files removed, got %d: %v", len(removed), removed)
	}
}
