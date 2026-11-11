package schtaskinstall

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestGenerateTaskXML_IsWellFormedAndRunsLoopAsSystem(t *testing.T) {
	doc := GenerateTaskXML(`C:\Program Files\cloudflare-ddns-agent\cloudflare-ddns-agent.exe`, Watchdog)

	// encoding="UTF-16" is only true after EncodeUTF16LE, so parse the body.
	body := doc[strings.Index(doc, "<Task"):]
	var task struct {
		Principal string `xml:"Principals>Principal>UserId"`
		Command   string `xml:"Actions>Exec>Command"`
		Arguments string `xml:"Actions>Exec>Arguments"`
		Interval  string `xml:"Triggers>TimeTrigger>Repetition>Interval"`
		Policy    string `xml:"Settings>MultipleInstancesPolicy"`
		Limit     string `xml:"Settings>ExecutionTimeLimit"`
	}
	if err := xml.Unmarshal([]byte(body), &task); err != nil {
		t.Fatalf("task XML is not well-formed: %v\n%s", err, doc)
	}

	if task.Principal != "S-1-5-18" {
		t.Errorf("task must run as SYSTEM (S-1-5-18), got %q", task.Principal)
	}
	if task.Command != `C:\Program Files\cloudflare-ddns-agent\cloudflare-ddns-agent.exe` {
		t.Errorf("unexpected command %q", task.Command)
	}
	if task.Arguments != "run" {
		t.Errorf("task must start the long-lived loop, got arguments %q", task.Arguments)
	}
	if task.Interval != "PT5M" {
		t.Errorf("expected 5 minute watchdog, got %q", task.Interval)
	}
	if task.Policy != "IgnoreNew" {
		t.Errorf("watchdog must not start a second loop, got policy %q", task.Policy)
	}
	if task.Limit != "PT0S" {
		t.Errorf("loop must not be killed by an execution time limit, got %q", task.Limit)
	}
	if !strings.Contains(doc, "<BootTrigger>") {
		t.Errorf("task must start at boot:\n%s", doc)
	}
}

func TestGenerateTaskXML_EscapesBinaryPath(t *testing.T) {
	doc := GenerateTaskXML(`C:\A&B\agent.exe`, Watchdog)
	if !strings.Contains(doc, `<Command>C:\A&amp;B\agent.exe</Command>`) {
		t.Errorf("binary path not escaped:\n%s", doc)
	}
}

func TestIsoMinutes_HasOneMinuteFloor(t *testing.T) {
	if got := isoMinutes(10 * time.Second); got != "PT1M" {
		t.Errorf("expected PT1M, got %q", got)
	}
	if got := isoMinutes(15 * time.Minute); got != "PT15M" {
		t.Errorf("expected PT15M, got %q", got)
	}
}

func TestEncodeUTF16LE_HasBOMAndRoundTrips(t *testing.T) {
	in := "<Task>ação</Task>"
	out := EncodeUTF16LE(in)
	if out[0] != 0xFF || out[1] != 0xFE {
		t.Fatalf("missing UTF-16LE BOM: % x", out[:2])
	}
	units := make([]uint16, (len(out)-2)/2)
	for i := range units {
		units[i] = uint16(out[2+2*i]) | uint16(out[3+2*i])<<8
	}
	if got := string(utf16.Decode(units)); got != in {
		t.Errorf("round trip mismatch: %q", got)
	}
}

func TestInstallWith_CreatesTaskFromTempFileAndStartsIt(t *testing.T) {
	var ran [][]string
	cleaned := false
	writeTemp := func(data []byte) (string, func(), error) {
		return `C:\tmp\task.xml`, func() { cleaned = true }, nil
	}
	run := func(name string, args ...string) error {
		ran = append(ran, append([]string{name}, args...))
		return nil
	}

	if err := installWith(`C:\agent.exe`, writeTemp, run, run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := [][]string{
		{"schtasks", "/End", "/TN", TaskName},
		{"schtasks", "/Create", "/TN", TaskName, "/XML", `C:\tmp\task.xml`, "/F"},
		{"schtasks", "/Run", "/TN", TaskName},
	}
	if len(ran) != len(want) {
		t.Fatalf("expected %d commands, got %d: %v", len(want), len(ran), ran)
	}
	for i := range want {
		if strings.Join(ran[i], " ") != strings.Join(want[i], " ") {
			t.Errorf("command %d: want %v, got %v", i, want[i], ran[i])
		}
	}
	if !cleaned {
		t.Error("temporary task XML was not removed")
	}
}

func TestInstallWith_IgnoresEndFailureWhenTaskDoesNotExistYet(t *testing.T) {
	writeTemp := func(data []byte) (string, func(), error) { return "x.xml", func() {}, nil }
	run := func(name string, args ...string) error {
		if args[0] == "/End" {
			return errors.New("task not found")
		}
		return nil
	}
	if err := installWith(`C:\agent.exe`, writeTemp, run, run); err != nil {
		t.Fatalf("first install must not fail on /End: %v", err)
	}
}

func TestInstallWith_PropagatesCreateError(t *testing.T) {
	writeTemp := func(data []byte) (string, func(), error) { return "x.xml", func() {}, nil }
	run := func(name string, args ...string) error {
		if args[0] == "/Create" {
			return errors.New("access denied")
		}
		return nil
	}
	if err := installWith(`C:\agent.exe`, writeTemp, run, run); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUninstallWith_EndsAndDeletesTask(t *testing.T) {
	var ran [][]string
	run := func(name string, args ...string) error {
		ran = append(ran, append([]string{name}, args...))
		return nil
	}
	if err := uninstallWith(run, run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ran) != 2 || ran[1][1] != "/Delete" {
		t.Fatalf("expected /End then /Delete, got %v", ran)
	}
}
