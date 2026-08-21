package updater

import (
	"fmt"

	"github.com/MX2Tech/cloudflare-ddns-agent/internal/cloudflare"
	"github.com/MX2Tech/cloudflare-ddns-agent/internal/config"
)

// CloudflareClient is the subset of *cloudflare.Client that Run depends on.
// *cloudflare.Client satisfies this automatically; tests use a fake.
type CloudflareClient interface {
	GetZoneID(zoneName string) (string, error)
	GetRecord(zoneID, hostname string) (*cloudflare.Record, error)
	CreateRecord(zoneID, hostname, ip string) error
	UpdateRecord(zoneID, recordID, hostname, ip string) error
}

// IPDetector matches publicip.Detect's signature.
type IPDetector func() (string, error)

type Result struct {
	Hostname string
	Action   string // "created", "updated", "unchanged", "error"
	IP       string
	Err      error
}

// Run reconciles every record in cfg against the given Cloudflare client,
// using the IP returned by detectIP as the desired target.
func Run(cfg *config.Config, client CloudflareClient, detectIP IPDetector) []Result {
	results := make([]Result, 0, len(cfg.Records))

	ip, err := detectIP()
	if err != nil {
		for _, r := range cfg.Records {
			results = append(results, Result{
				Hostname: r.Hostname,
				Action:   "error",
				Err:      fmt.Errorf("detecting public IP: %w", err),
			})
		}
		return results
	}

	zoneIDCache := map[string]string{}

	for _, rec := range cfg.Records {
		zoneID, ok := zoneIDCache[rec.Zone]
		if !ok {
			id, err := client.GetZoneID(rec.Zone)
			if err != nil {
				results = append(results, Result{Hostname: rec.Hostname, Action: "error", Err: err})
				continue
			}
			zoneID = id
			zoneIDCache[rec.Zone] = zoneID
		}

		results = append(results, reconcile(client, zoneID, rec.Hostname, ip))
	}

	return results
}

func reconcile(client CloudflareClient, zoneID, hostname, ip string) Result {
	existing, err := client.GetRecord(zoneID, hostname)
	if err != nil {
		return Result{Hostname: hostname, Action: "error", Err: err}
	}

	if existing == nil {
		if err := client.CreateRecord(zoneID, hostname, ip); err != nil {
			return Result{Hostname: hostname, Action: "error", Err: err}
		}
		return Result{Hostname: hostname, Action: "created", IP: ip}
	}

	if existing.Content == ip {
		return Result{Hostname: hostname, Action: "unchanged", IP: ip}
	}

	if err := client.UpdateRecord(zoneID, existing.ID, hostname, ip); err != nil {
		return Result{Hostname: hostname, Action: "error", Err: err}
	}
	return Result{Hostname: hostname, Action: "updated", IP: ip}
}
