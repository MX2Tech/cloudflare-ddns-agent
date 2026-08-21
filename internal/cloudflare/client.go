package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultBaseURL = "https://api.cloudflare.com/client/v4"

type Client struct {
	APIToken   string
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(apiToken string) *Client {
	return &Client{
		APIToken:   apiToken,
		BaseURL:    DefaultBaseURL,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Record is a DNS A record as returned by the Cloudflare API.
type Record struct {
	ID      string
	Content string
}

type apiResponse struct {
	Success bool            `json:"success"`
	Errors  []apiError      `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type dnsRecord struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

func (c *Client) do(method, path string, body interface{}, out *apiResponse) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from %s %s: %w", method, path, err)
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("parsing response from %s %s: %w", method, path, err)
	}

	if !out.Success {
		return fmt.Errorf("cloudflare API error on %s %s: %s", method, path, formatErrors(out.Errors))
	}

	return nil
}

func formatErrors(errs []apiError) string {
	if len(errs) == 0 {
		return "unknown error"
	}
	msg := ""
	for i, e := range errs {
		if i > 0 {
			msg += "; "
		}
		msg += fmt.Sprintf("[%d] %s", e.Code, e.Message)
	}
	return msg
}

// GetZoneID resolves a zone name (e.g. "example.com") to its Cloudflare zone ID.
func (c *Client) GetZoneID(zoneName string) (string, error) {
	var out apiResponse
	if err := c.do(http.MethodGet, "/zones?name="+zoneName, nil, &out); err != nil {
		return "", err
	}

	var zones []zone
	if err := json.Unmarshal(out.Result, &zones); err != nil {
		return "", fmt.Errorf("parsing zones result: %w", err)
	}
	if len(zones) == 0 {
		return "", fmt.Errorf("zone %q not found in this Cloudflare account", zoneName)
	}
	return zones[0].ID, nil
}

// GetRecord looks up an A record by hostname within a zone.
// Returns nil, nil if no such record exists (not an error).
func (c *Client) GetRecord(zoneID, hostname string) (*Record, error) {
	var out apiResponse
	path := fmt.Sprintf("/zones/%s/dns_records?type=A&name=%s", zoneID, hostname)
	if err := c.do(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}

	var records []dnsRecord
	if err := json.Unmarshal(out.Result, &records); err != nil {
		return nil, fmt.Errorf("parsing dns_records result: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &Record{ID: records[0].ID, Content: records[0].Content}, nil
}

// CreateRecord creates a new A record for hostname pointing to ip.
func (c *Client) CreateRecord(zoneID, hostname, ip string) error {
	var out apiResponse
	path := fmt.Sprintf("/zones/%s/dns_records", zoneID)
	body := map[string]interface{}{
		"type":    "A",
		"name":    hostname,
		"content": ip,
		"ttl":     1,
		"proxied": false,
	}
	return c.do(http.MethodPost, path, body, &out)
}

// UpdateRecord updates an existing A record to point to ip.
func (c *Client) UpdateRecord(zoneID, recordID, hostname, ip string) error {
	var out apiResponse
	path := fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, recordID)
	body := map[string]interface{}{
		"type":    "A",
		"name":    hostname,
		"content": ip,
		"ttl":     1,
		"proxied": false,
	}
	return c.do(http.MethodPatch, path, body, &out)
}
