package publicip

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectFrom_FirstSourceSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("203.0.113.5\n"))
	}))
	defer srv.Close()

	ip, err := DetectFrom([]string{srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "203.0.113.5" {
		t.Errorf("got %q, want %q", ip, "203.0.113.5")
	}
}

func TestDetectFrom_FallsBackOnFailure(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("198.51.100.7"))
	}))
	defer good.Close()

	ip, err := DetectFrom([]string{bad.URL, good.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "198.51.100.7" {
		t.Errorf("got %q, want %q", ip, "198.51.100.7")
	}
}

func TestDetectFrom_AllSourcesFail(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()

	if _, err := DetectFrom([]string{bad.URL}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDetectFrom_InvalidIPResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>not an ip</html>"))
	}))
	defer srv.Close()

	if _, err := DetectFrom([]string{srv.URL}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDetectFrom_IPv6ResponseFallsBackToNextSource(t *testing.T) {
	ipv6 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("2001:db8::1"))
	}))
	defer ipv6.Close()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("198.51.100.7"))
	}))
	defer good.Close()

	ip, err := DetectFrom([]string{ipv6.URL, good.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "198.51.100.7" {
		t.Errorf("got %q, want %q", ip, "198.51.100.7")
	}
}
