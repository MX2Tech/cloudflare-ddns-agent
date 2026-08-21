package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}

func TestLoad_Valid(t *testing.T) {
	path := writeTempConfig(t, `
cloudflare:
  api_token: "test-token"
check_interval: 30s
records:
  - zone: example.com
    hostname: hub.example.com
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cloudflare.APIToken != "test-token" {
		t.Errorf("got token %q", cfg.Cloudflare.APIToken)
	}
	if len(cfg.Records) != 1 || cfg.Records[0].Hostname != "hub.example.com" {
		t.Errorf("got records %+v", cfg.Records)
	}
	interval, err := cfg.Interval()
	if err != nil {
		t.Fatalf("unexpected error parsing interval: %v", err)
	}
	if interval.Seconds() != 30 {
		t.Errorf("got interval %v, want 30s", interval)
	}
}

func TestLoad_MissingToken(t *testing.T) {
	path := writeTempConfig(t, `
cloudflare:
  api_token: ""
check_interval: 30s
records:
  - zone: example.com
    hostname: hub.example.com
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
}

func TestLoad_NoRecords(t *testing.T) {
	path := writeTempConfig(t, `
cloudflare:
  api_token: "test-token"
check_interval: 30s
records: []
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for empty records, got nil")
	}
}

func TestLoad_RecordMissingHostname(t *testing.T) {
	path := writeTempConfig(t, `
cloudflare:
  api_token: "test-token"
check_interval: 30s
records:
  - zone: example.com
    hostname: ""
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing hostname, got nil")
	}
}

func TestLoad_InvalidInterval(t *testing.T) {
	path := writeTempConfig(t, `
cloudflare:
  api_token: "test-token"
check_interval: "not-a-duration"
records:
  - zone: example.com
    hostname: hub.example.com
`)

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid interval, got nil")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	if _, err := Load("/nonexistent/path/config.yaml"); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
