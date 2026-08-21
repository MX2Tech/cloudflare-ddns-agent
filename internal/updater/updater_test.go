package updater

import (
	"errors"
	"testing"

	"github.com/MX2Tech/cloudflare-ddns-agent/internal/cloudflare"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/config"
)

type createCall struct {
	zoneID   string
	hostname string
	ip       string
}

type updateCall struct {
	zoneID   string
	recordID string
	hostname string
	ip       string
}

type fakeClient struct {
	zoneIDs      map[string]string
	records      map[string]*cloudflare.Record // key: hostname
	createCalls  []createCall
	updateCalls  []updateCall
	getRecordErr error
}

func (f *fakeClient) GetZoneID(zoneName string) (string, error) {
	id, ok := f.zoneIDs[zoneName]
	if !ok {
		return "", errors.New("zone not found")
	}
	return id, nil
}

func (f *fakeClient) GetRecord(zoneID, hostname string) (*cloudflare.Record, error) {
	if f.getRecordErr != nil {
		return nil, f.getRecordErr
	}
	return f.records[hostname], nil
}

func (f *fakeClient) CreateRecord(zoneID, hostname, ip string) error {
	f.createCalls = append(f.createCalls, createCall{zoneID: zoneID, hostname: hostname, ip: ip})
	return nil
}

func (f *fakeClient) UpdateRecord(zoneID, recordID, hostname, ip string) error {
	f.updateCalls = append(f.updateCalls, updateCall{zoneID: zoneID, recordID: recordID, hostname: hostname, ip: ip})
	return nil
}

func testConfig(hostname, zone string) *config.Config {
	return &config.Config{
		Records: []config.Record{{Zone: zone, Hostname: hostname}},
	}
}

func TestRun_CreatesWhenRecordMissing(t *testing.T) {
	client := &fakeClient{
		zoneIDs: map[string]string{"example.com": "zone1"},
		records: map[string]*cloudflare.Record{},
	}
	cfg := testConfig("hub.example.com", "example.com")

	results := Run(cfg, client, func() (string, error) { return "203.0.113.5", nil })

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "created" {
		t.Errorf("got action %q, want %q", results[0].Action, "created")
	}
	if len(client.createCalls) != 1 {
		t.Fatalf("expected 1 create call, got %d", len(client.createCalls))
	}
	call := client.createCalls[0]
	if call.zoneID != "zone1" {
		t.Errorf("got zoneID %q, want %q", call.zoneID, "zone1")
	}
	if call.ip != "203.0.113.5" {
		t.Errorf("got ip %q, want %q", call.ip, "203.0.113.5")
	}
}

func TestRun_UpdatesWhenIPDifferent(t *testing.T) {
	client := &fakeClient{
		zoneIDs: map[string]string{"example.com": "zone1"},
		records: map[string]*cloudflare.Record{
			"hub.example.com": {ID: "rec1", Content: "198.51.100.1"},
		},
	}
	cfg := testConfig("hub.example.com", "example.com")

	results := Run(cfg, client, func() (string, error) { return "203.0.113.5", nil })

	if results[0].Action != "updated" {
		t.Errorf("got action %q, want %q", results[0].Action, "updated")
	}
	if len(client.updateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(client.updateCalls))
	}
	call := client.updateCalls[0]
	if call.zoneID != "zone1" {
		t.Errorf("got zoneID %q, want %q", call.zoneID, "zone1")
	}
	if call.ip != "203.0.113.5" {
		t.Errorf("got ip %q, want %q", call.ip, "203.0.113.5")
	}
}

func TestRun_SkipsWhenIPUnchanged(t *testing.T) {
	client := &fakeClient{
		zoneIDs: map[string]string{"example.com": "zone1"},
		records: map[string]*cloudflare.Record{
			"hub.example.com": {ID: "rec1", Content: "203.0.113.5"},
		},
	}
	cfg := testConfig("hub.example.com", "example.com")

	results := Run(cfg, client, func() (string, error) { return "203.0.113.5", nil })

	if results[0].Action != "unchanged" {
		t.Errorf("got action %q, want %q", results[0].Action, "unchanged")
	}
	if len(client.createCalls) != 0 || len(client.updateCalls) != 0 {
		t.Error("expected no create/update calls")
	}
}

func TestRun_ReportsErrorPerHostnameWithoutStoppingOthers(t *testing.T) {
	client := &fakeClient{
		zoneIDs: map[string]string{"good.com": "zone1"},
		records: map[string]*cloudflare.Record{
			"a.good.com": {ID: "rec1", Content: "203.0.113.5"},
		},
	}
	cfg := &config.Config{
		Records: []config.Record{
			{Zone: "bad.com", Hostname: "a.bad.com"},
			{Zone: "good.com", Hostname: "a.good.com"},
		},
	}

	results := Run(cfg, client, func() (string, error) { return "203.0.113.5", nil })

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Action != "error" {
		t.Errorf("got action %q for first record, want %q", results[0].Action, "error")
	}
	if results[1].Action != "unchanged" {
		t.Errorf("got action %q for second record, want %q", results[1].Action, "unchanged")
	}
}

func TestRun_IPDetectionFailureReportsErrorForAll(t *testing.T) {
	client := &fakeClient{}
	cfg := &config.Config{
		Records: []config.Record{
			{Zone: "example.com", Hostname: "a.example.com"},
			{Zone: "example.com", Hostname: "b.example.com"},
		},
	}

	results := Run(cfg, client, func() (string, error) { return "", errors.New("network down") })

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Action != "error" {
			t.Errorf("got action %q, want %q", r.Action, "error")
		}
	}
}
