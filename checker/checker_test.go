// checker_test.go tests the checker package.
// package checker — same package as the code we're testing.
package checker

import (
	"net/http"
	"net/http/httptest" // Go's built-in fake HTTP server for testing
	"testing"
	"time"
)

// TestCheck_Up tests that Check() correctly identifies a responding server as "up".
//
// httptest.NewServer creates a real local HTTP server that listens on a random port.
// This means we never hit real URLs in tests — tests are fast and work offline.
//
// PHP equivalent: using Mockery to mock a Guzzle HTTP client,
// but Go's httptest is built-in and starts a real server instead of a mock.
func TestCheck_Up(t *testing.T) {
	// httptest.NewServer takes an http.HandlerFunc — a function that handles requests.
	// This fake server always responds with 200 OK.
	// http.HandlerFunc() is a type conversion (not a function call) —
	// it turns our function into something that satisfies the http.Handler interface.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// w.WriteHeader sends the HTTP status code.
		// If you don't call it, Go defaults to 200 — but being explicit is clearer in tests.
		w.WriteHeader(http.StatusOK) // 200
	}))

	// server.Close() shuts down the fake server when the test finishes.
	// defer = runs when this function returns — like finally{} in PHP.
	defer server.Close()

	// server.URL is the address of the fake server, e.g. "http://127.0.0.1:54321"
	target := Target{Name: "Test", URL: server.URL}
	result := target.Check()

	if !result.Up {
		t.Errorf("expected Up=true, got Up=false")
	}
	if result.StatusCode != 200 {
		t.Errorf("expected StatusCode=200, got %d", result.StatusCode)
	}
	if result.Error != "" {
		t.Errorf("expected no error, got %q", result.Error)
	}
}

// TestCheck_Down_BadStatus tests that a non-2xx response is treated as "down".
func TestCheck_Down_BadStatus(t *testing.T) {
	// This fake server always responds with 500 Internal Server Error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // 500
	}))
	defer server.Close()

	target := Target{Name: "Test", URL: server.URL}
	result := target.Check()

	if result.Up {
		t.Errorf("expected Up=false for 500 response, got Up=true")
	}
	if result.StatusCode != 500 {
		t.Errorf("expected StatusCode=500, got %d", result.StatusCode)
	}
}

// TestCheck_Down_NoServer tests that Check() handles a completely unreachable URL.
// No fake server — we just use an address nothing is listening on.
func TestCheck_Down_NoServer(t *testing.T) {
	// Port 1 is reserved and nothing listens on it — connection will be refused.
	target := Target{Name: "Dead", URL: "http://127.0.0.1:1"}
	result := target.Check()

	if result.Up {
		t.Errorf("expected Up=false for unreachable server, got Up=true")
	}
	if result.Error == "" {
		t.Errorf("expected an error message, got empty string")
	}
}

// TestCheck_Timeout tests that Check() gives up and returns an error when
// a server hangs and never responds within the timeout.
func TestCheck_Timeout(t *testing.T) {
	// This fake server receives the request but never writes a response —
	// it just blocks forever using a channel that never gets a value.
	// This simulates a hung server.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until the request's context is cancelled — i.e. until the client
		// gives up and cancels the request (which happens on timeout).
		// r.Context().Done() is a channel that closes when the context is cancelled.
		// This way server.Close() can finish cleanly after the client disconnects.
		<-r.Context().Done()
	}))
	defer server.Close()

	// Override the default timeout to something short so the test doesn't
	// actually wait 10 seconds. We temporarily set it to 100ms.
	// This works because defaultTimeout is a package-level variable in the
	// same package (package checker) — test files can access it directly.
	original := defaultTimeout
	defaultTimeout = 100 * time.Millisecond
	defer func() { defaultTimeout = original }() // restore after test

	target := Target{Name: "Hanging", URL: server.URL}
	result := target.Check()

	if result.Up {
		t.Errorf("expected Up=false for timed-out server, got Up=true")
	}
	if result.Error == "" {
		t.Errorf("expected a timeout error message, got empty string")
	}
}

// TestCheckAll_Concurrent tests that CheckAll returns one result per target.
// It also implicitly tests that concurrent execution works correctly —
// if there were a race condition, the Go race detector would catch it.
func TestCheckAll_Concurrent(t *testing.T) {
	// Start two fake servers — one returns 200, one returns 404.
	serverUp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // 200
	}))
	defer serverUp.Close()

	serverDown := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // 404
	}))
	defer serverDown.Close()

	targets := []Target{
		{Name: "Up", URL: serverUp.URL},
		{Name: "Down", URL: serverDown.URL},
	}

	results := CheckAll(targets)

	// We must get exactly one result per target.
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Build a map of results by target name for easy lookup.
	// This is needed because CheckAll returns results in non-deterministic order
	// (whichever goroutine finishes first) — we can't rely on results[0] being "Up".
	//
	// map[string]Result is a map with string keys and Result values.
	// Like: $resultsByName = []; in PHP.
	byName := make(map[string]Result)
	for _, r := range results {
		byName[r.Target.Name] = r
	}

	if !byName["Up"].Up {
		t.Errorf("expected 'Up' target to be up")
	}
	if byName["Down"].Up {
		t.Errorf("expected 'Down' target (404) to be down")
	}
}
