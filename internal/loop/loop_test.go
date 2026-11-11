package loop

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MX2Tech/cloudflare-ddns-agent/internal/cloudflare"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/config"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/updater"
)

type fakeClient struct {
	content string // current A record value; "" = record missing
	writes  int
}

func (f *fakeClient) GetZoneID(string) (string, error) { return "zone-1", nil }

func (f *fakeClient) GetRecord(string, string) (*cloudflare.Record, error) {
	if f.content == "" {
		return nil, nil
	}
	return &cloudflare.Record{ID: "rec-1", Content: f.content}, nil
}

func (f *fakeClient) CreateRecord(_, _, ip string) error { f.content = ip; f.writes++; return nil }

func (f *fakeClient) UpdateRecord(_, _, _, ip string) error { f.content = ip; f.writes++; return nil }

type clock struct{ t time.Time }

func (c *clock) now() time.Time            { return c.t }
func (c *clock) sleep(d time.Duration)     { c.t = c.t.Add(d) }
func validConfig() (*config.Config, error) { return testConfig("30s"), nil }

func testConfig(interval string) *config.Config {
	return &config.Config{
		Cloudflare:    config.CloudflareConfig{APIToken: "t"},
		CheckInterval: interval,
		Records:       []config.Record{{Zone: "mx2tech.cloud", Hostname: "cliente.mx2tech.cloud"}},
	}
}

// runCycles runs the loop for n cycles and returns the logged lines.
func runCycles(t *testing.T, n int, o Options) []string {
	t.Helper()
	var lines []string
	cycles := 0
	o.Log = func(l string) { lines = append(lines, l) }
	o.Stop = func() bool { cycles++; return cycles >= n }
	Run(o)
	return lines
}

func TestRun_CreatesMissingRecordAndThenStaysQuiet(t *testing.T) {
	c := &clock{t: time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}
	fc := &fakeClient{}
	lines := runCycles(t, 10, Options{
		LoadConfig: validConfig,
		NewClient:  func(string) updater.CloudflareClient { return fc },
		DetectIP:   func() (string, error) { return "200.1.2.3", nil },
		Sleep:      c.sleep,
		Now:        c.now,
	})

	if fc.writes != 1 {
		t.Fatalf("expected exactly 1 write to Cloudflare, got %d", fc.writes)
	}
	// started + created + one "unchanged" confirmation; the other 8 cycles are silent.
	if len(lines) != 3 {
		t.Fatalf("expected 3 log lines in 10 cycles, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[1], "action=created ip=200.1.2.3") {
		t.Errorf("expected created line, got %q", lines[1])
	}
}

func TestRun_LogsEveryIPChange(t *testing.T) {
	c := &clock{t: time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}
	fc := &fakeClient{content: "200.1.2.3"}
	ips := []string{"200.1.2.3", "200.9.9.9", "200.9.9.9", "200.1.2.3"}
	i := 0
	lines := runCycles(t, len(ips), Options{
		LoadConfig: validConfig,
		NewClient:  func(string) updater.CloudflareClient { return fc },
		DetectIP:   func() (string, error) { ip := ips[i]; i++; return ip, nil },
		Sleep:      c.sleep,
		Now:        c.now,
	})

	updated := 0
	for _, l := range lines {
		if strings.Contains(l, "action=updated") {
			updated++
		}
	}
	if updated != 2 {
		t.Fatalf("expected 2 updated lines, got %d:\n%s", updated, strings.Join(lines, "\n"))
	}
}

func TestRun_RepeatsIdenticalLineOnlyAfterAnHour(t *testing.T) {
	c := &clock{t: time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}
	fc := &fakeClient{}
	// 30s interval: 121 cycles span one hour.
	lines := runCycles(t, 122, Options{
		LoadConfig: validConfig,
		NewClient:  func(string) updater.CloudflareClient { return fc },
		DetectIP:   func() (string, error) { return "", errors.New("offline") },
		Sleep:      c.sleep,
		Now:        c.now,
	})

	errorsLogged := 0
	for _, l := range lines {
		if strings.Contains(l, "offline") {
			errorsLogged++
		}
	}
	if errorsLogged != 2 {
		t.Fatalf("expected the error once plus one hourly repeat, got %d", errorsLogged)
	}
}

func TestRun_KeepsRunningWhenConfigIsBrokenAndRecovers(t *testing.T) {
	c := &clock{t: time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)}
	fc := &fakeClient{}
	calls := 0
	var slept []time.Duration
	lines := runCycles(t, 3, Options{
		LoadConfig: func() (*config.Config, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("config missing")
			}
			return testConfig("45s"), nil
		},
		NewClient: func(string) updater.CloudflareClient { return fc },
		DetectIP:  func() (string, error) { return "200.1.2.3", nil },
		Sleep:     func(d time.Duration) { slept = append(slept, d); c.sleep(d) },
		Now:       c.now,
	})

	if !strings.Contains(strings.Join(lines, "\n"), "config missing") {
		t.Errorf("config error was not logged:\n%s", strings.Join(lines, "\n"))
	}
	if fc.writes != 1 {
		t.Errorf("expected the record to be created after the config recovered, got %d writes", fc.writes)
	}
	if slept[0] != FallbackInterval || slept[1] != 45*time.Second {
		t.Errorf("expected fallback then configured interval, got %v", slept)
	}
}

func TestFormatResult(t *testing.T) {
	ok := FormatResult(updater.Result{Hostname: "h", Action: "unchanged", IP: "1.2.3.4"})
	if ok != "level=info hostname=h action=unchanged ip=1.2.3.4" {
		t.Errorf("unexpected info line %q", ok)
	}
	bad := FormatResult(updater.Result{Hostname: "h", Action: "error", Err: errors.New("boom")})
	if bad != `level=error hostname=h msg="boom"` {
		t.Errorf("unexpected error line %q", bad)
	}
}
