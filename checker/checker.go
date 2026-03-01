// Package checker contains all the logic for monitoring URLs.
// In Laravel terms, think of this as a Service class — it does the "work"
// of the application, separate from the entry point (main.go).
package checker

import (
	"context"  // for context.WithTimeout — request cancellation and deadlines
	"fmt"
	"net/http"
	"time"
)

// defaultTimeout is how long we wait for a single HTTP request before giving up.
// A package-level variable — using var instead of const so tests can override it
// temporarily without needing to change the real code.
// If a server doesn't respond within this time, we treat it as down.
var defaultTimeout = 10 * time.Second

// Target represents a single URL we want to monitor.
type Target struct {
	// Name is a human-readable label, e.g. "My Blog"
	Name string

	// URL is the full address to check, e.g. "https://example.com"
	URL string
}

// Result represents the outcome of checking a single Target.
type Result struct {
	// Target is the URL we checked.
	Target Target

	// StatusCode is the HTTP status code returned, e.g. 200, 404, 500.
	// It will be 0 if the request failed entirely (no response at all).
	StatusCode int

	// Duration is how long the request took to complete.
	Duration time.Duration

	// Up is true if the site responded with a 2xx status code.
	Up bool

	// Error holds the error message if the request failed.
	Error string
}

// Check performs an HTTP GET request to t.URL and returns a Result.
// It will give up and return an error if the server doesn't respond
// within defaultTimeout.
func (t Target) Check() Result {
	start := time.Now()

	// --- context.WithTimeout ---
	//
	// A "context" in Go carries a deadline, cancellation signal, and
	// request-scoped values. Think of it as a timer attached to an operation.
	//
	// context.Background() is the root context — a plain empty context
	// with no deadline. Like a blank slate.
	// In PHP there's no equivalent — PHP requests always have a max_execution_time
	// set in php.ini, but you can't set per-request timeouts programmatically.
	//
	// context.WithTimeout(parent, duration) creates a NEW context derived from
	// the parent, with a deadline of "now + duration". It returns two values:
	//   ctx    — the new context with the deadline attached
	//   cancel — a function you MUST call to free resources when done
	//
	// If the deadline expires before the operation finishes, Go automatically
	// cancels any operation using this context and returns an error.
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)

	// cancel() MUST always be called — even if the request finishes before
	// the timeout. Without it, the context leaks resources until the deadline.
	// defer guarantees it runs when Check() returns — like finally{} in PHP.
	defer cancel()

	// --- http.NewRequestWithContext ---
	//
	// Instead of http.Get() (which has no timeout), we build a request manually
	// and attach our context to it. The context carries the deadline.
	//
	// http.NewRequestWithContext(ctx, method, url, body) creates a new request.
	// "GET" is the HTTP method, t.URL is the address, nil means no request body.
	// It returns (request, error) — we check the error as always.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		return Result{
			Target:   t,
			Duration: time.Since(start),
			Up:       false,
			Error:    err.Error(),
		}
	}

	// http.DefaultClient is Go's built-in HTTP client — we use it to send
	// the request. The client respects the deadline in the context.
	// If the server doesn't respond before the deadline, Do() returns an error.
	//
	// PHP equivalent: $client->get($url, ['timeout' => 10]) in Guzzle.
	resp, err := http.DefaultClient.Do(req)

	duration := time.Since(start)

	if err != nil {
		return Result{
			Target:   t,
			Duration: duration,
			Up:       false,
			Error:    err.Error(),
		}
	}

	defer resp.Body.Close()

	up := resp.StatusCode >= 200 && resp.StatusCode < 300

	return Result{
		Target:     t,
		StatusCode: resp.StatusCode,
		Duration:   duration,
		Up:         up,
	}
}

// CheckAll checks all targets CONCURRENTLY and returns all results.
// Each check runs in its own goroutine and has its own independent timeout.
func CheckAll(targets []Target) []Result {
	// A buffered channel to collect results from all goroutines.
	results := make(chan Result, len(targets))

	for _, target := range targets {
		t := target
		go func() {
			// Each goroutine calls Check() which has its own context+timeout.
			// So if one site hangs for 10s, the others are not affected —
			// they finish and send their results while we wait for the slow one.
			results <- t.Check()
		}()
	}

	all := make([]Result, 0, len(targets))
	for range targets {
		all = append(all, <-results)
	}

	return all
}

// String returns a human-readable summary of a Result.
// Go calls this automatically when you pass a Result to fmt.Println().
// In PHP, this is like __toString() on a class.
func (r Result) String() string {
	if r.Up {
		return fmt.Sprintf("✓ %s (%s) — %d — %v", r.Target.Name, r.Target.URL, r.StatusCode, r.Duration)
	}

	if r.Error != "" {
		return fmt.Sprintf("✗ %s (%s) — ERROR: %s — %v", r.Target.Name, r.Target.URL, r.Error, r.Duration)
	}

	return fmt.Sprintf("✗ %s (%s) — %d — %v", r.Target.Name, r.Target.URL, r.StatusCode, r.Duration)
}
