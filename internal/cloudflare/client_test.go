package cloudflare

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Client{
		APIToken:   "test-token",
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	}
}

func TestGetZoneID_Found(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zones" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"errors":  []interface{}{},
			"result":  []map[string]string{{"id": "zone123", "name": "example.com"}},
		})
	})

	id, err := client.GetZoneID("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "zone123" {
		t.Errorf("got %q, want %q", id, "zone123")
	}
}

func TestGetZoneID_NotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"errors":  []interface{}{},
			"result":  []map[string]string{},
		})
	})

	if _, err := client.GetZoneID("nope.com"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetRecord_Exists(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"errors":  []interface{}{},
			"result": []map[string]string{
				{"id": "rec123", "content": "203.0.113.5"},
			},
		})
	})

	rec, err := client.GetRecord("zone123", "hub.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec == nil {
		t.Fatal("expected record, got nil")
	}
	if rec.ID != "rec123" || rec.Content != "203.0.113.5" {
		t.Errorf("got %+v", rec)
	}
}

func TestGetRecord_DoesNotExist(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"errors":  []interface{}{},
			"result":  []map[string]string{},
		})
	})

	rec, err := client.GetRecord("zone123", "missing.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec != nil {
		t.Errorf("expected nil record, got %+v", rec)
	}
}

func TestCreateRecord_Success(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"errors":  []interface{}{},
			"result":  map[string]string{"id": "newrec", "content": "203.0.113.9"},
		})
	})

	if err := client.CreateRecord("zone123", "new.example.com", "203.0.113.9"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateRecord_Success(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"errors":  []interface{}{},
			"result":  map[string]string{"id": "rec123", "content": "203.0.113.10"},
		})
	})

	if err := client.UpdateRecord("zone123", "rec123", "hub.example.com", "203.0.113.10"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"errors": []map[string]interface{}{
				{"code": 1000, "message": "Invalid API Token"},
			},
			"result": nil,
		})
	})

	if _, err := client.GetZoneID("example.com"); err == nil {
		t.Fatal("expected error, got nil")
	}
}
