package publicip

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var Sources = []string{
	"https://ipv4.icanhazip.com",
	"https://api.ipify.org",
}

var httpClient = &http.Client{Timeout: 5 * time.Second}

// Detect returns this machine's current public IPv4 address, trying each
// of Sources in order until one succeeds.
func Detect() (string, error) {
	return DetectFrom(Sources)
}

// DetectFrom is like Detect but takes an explicit list of "what's my IP"
// endpoints, so tests can point it at an httptest.Server instead of the
// real internet.
func DetectFrom(sources []string) (string, error) {
	var lastErr error
	for _, url := range sources {
		ip, err := fetchIP(url)
		if err != nil {
			lastErr = err
			continue
		}
		return ip, nil
	}
	return "", fmt.Errorf("could not determine public IP from any source: %w", lastErr)
}

func fetchIP(url string) (string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("requesting %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response from %s: %w", url, err)
	}

	ip := strings.TrimSpace(string(body))
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return "", fmt.Errorf("%s returned invalid IPv4 address %q", url, ip)
	}

	return ip, nil
}
