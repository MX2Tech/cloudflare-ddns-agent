// Package loop runs the reconcile cycle forever, for platforms where the
// agent is a long-lived process (Windows Task Scheduler) instead of a
// systemd timer firing "update".
package loop

import (
	"fmt"
	"time"

	"github.com/MX2Tech/cloudflare-ddns-agent/internal/config"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/updater"
)

// FallbackInterval is used while the config cannot be loaded.
const FallbackInterval = 30 * time.Second

// RepeatAfter is how long an identical log line stays suppressed. It keeps
// a 30s loop from writing thousands of "unchanged" lines a day while still
// leaving an hourly heartbeat in the log.
const RepeatAfter = time.Hour

type Options struct {
	LoadConfig func() (*config.Config, error)
	NewClient  func(apiToken string) updater.CloudflareClient
	DetectIP   updater.IPDetector
	Log        func(line string)
	Sleep      func(time.Duration)
	Now        func() time.Time
	// Stop, when set, is checked after every cycle; returning true ends Run.
	Stop func() bool
}

// Run reloads the config and reconciles every record each cycle, so edits
// to config.yaml take effect without restarting the process.
func Run(o Options) {
	if o.Now == nil {
		o.Now = time.Now
	}
	seen := newDedup(RepeatAfter, o.Now)
	interval := FallbackInterval

	o.Log(`level=info msg="run loop started"`)
	for {
		cfg, err := o.LoadConfig()
		if err != nil {
			if line := fmt.Sprintf("level=error msg=%q", err.Error()); seen.allow("config", line) {
				o.Log(line)
			}
		} else {
			if d, err := cfg.Interval(); err == nil {
				interval = d
			}
			seen.forget("config")
			for _, r := range updater.Run(cfg, o.NewClient(cfg.Cloudflare.APIToken), o.DetectIP) {
				line := FormatResult(r)
				if r.Action == "created" || r.Action == "updated" {
					o.Log(line)
					seen.record(r.Hostname, line)
					continue
				}
				if seen.allow(r.Hostname, line) {
					o.Log(line)
				}
			}
		}

		if o.Stop != nil && o.Stop() {
			return
		}
		o.Sleep(interval)
	}
}

// FormatResult renders a reconcile result in the agent's key=value format.
func FormatResult(r updater.Result) string {
	if r.Action == "error" {
		return fmt.Sprintf("level=error hostname=%s msg=%q", r.Hostname, r.Err.Error())
	}
	return fmt.Sprintf("level=info hostname=%s action=%s ip=%s", r.Hostname, r.Action, r.IP)
}

type dedup struct {
	window time.Duration
	now    func() time.Time
	line   map[string]string
	at     map[string]time.Time
}

func newDedup(window time.Duration, now func() time.Time) *dedup {
	return &dedup{window: window, now: now, line: map[string]string{}, at: map[string]time.Time{}}
}

// allow reports whether line should be logged for key, and records it if so.
func (d *dedup) allow(key, line string) bool {
	if d.line[key] == line && d.now().Sub(d.at[key]) < d.window {
		return false
	}
	d.record(key, line)
	return true
}

func (d *dedup) record(key, line string) {
	d.line[key] = line
	d.at[key] = d.now()
}

func (d *dedup) forget(key string) {
	delete(d.line, key)
	delete(d.at, key)
}
