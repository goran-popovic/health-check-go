package notifier

// Same package as the code under test (package notifier, not package notifier_test).
// This lets us access the unexported 'payload' struct to verify the JSON body.

import (
	"encoding/json"
	"io"          // io.ReadAll — reads the full request body into a []byte
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goran-popovic/go-health-check/checker"
)

// TestWebhookNotifier_Success verifies that a successful POST:
//   - returns no error
//   - sends the correct Content-Type header
//   - sends a JSON body with the right site name, URL, and error message
func TestWebhookNotifier_Success(t *testing.T) {
	// Capture what the fake server receives so we can assert on it.
	var receivedBody        []byte
	var receivedContentType string

	// httptest.NewServer — same pattern as checker_test.go.
	// Our fake "webhook endpoint" reads the request and returns 200.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")

		// io.ReadAll reads the entire request body into a []byte.
		// Like file_get_contents("php://input") in PHP.
		// We ignore the error here — if it fails the JSON unmarshal below will catch it.
		receivedBody, _ = io.ReadAll(r.Body)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := WebhookNotifier{URL: server.URL}

	result := checker.Result{
		Target: checker.Target{Name: "Fake Site", URL: "https://fake.com"},
		Up:     false,
		Error:  "connection refused",
	}

	err := n.Notify(result)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify the Content-Type header was set correctly.
	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type %q, got %q", "application/json", receivedContentType)
	}

	// Unmarshal the body the fake server received back into a payload struct.
	// Because this test is in 'package notifier', it can access the unexported payload type.
	// This is the main reason we test in the same package — it lets us inspect internals.
	var p payload
	if err := json.Unmarshal(receivedBody, &p); err != nil {
		t.Fatalf("request body was not valid JSON: %v", err)
	}

	// Verify each field was populated correctly.
	if p.Site != "Fake Site" {
		t.Errorf("payload.Site: expected %q, got %q", "Fake Site", p.Site)
	}
	if p.URL != "https://fake.com" {
		t.Errorf("payload.URL: expected %q, got %q", "https://fake.com", p.URL)
	}
	if p.Error != "connection refused" {
		t.Errorf("payload.Error: expected %q, got %q", "connection refused", p.Error)
	}
}

// TestWebhookNotifier_BadStatus verifies that a non-2xx response from the webhook
// causes Notify() to return an error.
// This tests our response-status check: if resp.StatusCode >= 300 → return error.
func TestWebhookNotifier_BadStatus(t *testing.T) {
	// Fake server that always returns 500.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	n := WebhookNotifier{URL: server.URL}

	result := checker.Result{
		Target: checker.Target{Name: "Test", URL: "https://test.com"},
		Up:     false,
	}

	err := n.Notify(result)
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}

// TestWebhookNotifier_BadURL verifies that an invalid URL causes Notify() to return
// an error rather than panic or hang.
// No fake server needed — http.Post will fail immediately on a malformed URL.
func TestWebhookNotifier_BadURL(t *testing.T) {
	n := WebhookNotifier{URL: "not-a-valid-url"}

	result := checker.Result{
		Target: checker.Target{Name: "Test", URL: "https://test.com"},
		Up:     false,
	}

	err := n.Notify(result)
	if err == nil {
		t.Fatal("expected an error for an invalid URL, got nil")
	}
}
