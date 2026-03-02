// Package notifier defines how the app sends alerts when a site goes down.
// It uses a Go interface so the notification mechanism is swappable —
// you could add email, SMS, or PagerDuty without touching runChecks().
//
// PHP parallel: think of this like a Laravel Notification channel —
// you define a contract (interface) and swap implementations freely.
package notifier

import (
	"bytes"           // bytes.NewReader — wraps []byte as an io.Reader
	"encoding/json"   // json.Marshal — like json_encode() in PHP
	"fmt"
	"net/http"

	"github.com/goran-popovic/go-health-check/checker"
)

// Notifier is the interface every notification type must satisfy.
//
// --- HOW GO INTERFACES WORK ---
//
// In PHP you explicitly declare that a class implements an interface:
//   class WebhookNotifier implements Notifier { ... }
//
// In Go, interfaces are IMPLICIT — any type that has the required methods
// automatically satisfies the interface. No declaration needed.
// The interface and the struct don't even need to know about each other.
//
// This means:
//   - You can define a Notifier interface and any type with a Notify() method qualifies
//   - Third-party types you don't control can satisfy your interface
//   - It's easier to write small, focused interfaces
//
// The rule of thumb in Go: accept interfaces, return concrete types.
// runChecks() will accept a Notifier (interface) — it doesn't care HOW
// the notification is sent, only THAT it gets sent.
type Notifier interface {
	// Notify sends an alert about a failed check.
	// Returns an error if the notification couldn't be delivered.
	Notify(result checker.Result) error
}

// WebhookNotifier sends a JSON POST request to a webhook URL.
// Works with Slack incoming webhooks, Discord webhooks, custom endpoints, etc.
//
// This is a VALUE type (not a pointer) because it has no internal state
// that changes after creation — just a URL string. Copying it is safe.
type WebhookNotifier struct {
	// URL is the webhook endpoint to POST to.
	// e.g. "https://hooks.slack.com/services/..." or a webhook.site URL for testing.
	URL string
}

// payload is the JSON structure we send to the webhook.
// Struct tags (the `json:"..."` parts) control the field names in the JSON output.
// Like specifying column names in a Laravel $casts or API resource.
//
// This is unexported (lowercase) — it's an internal detail of WebhookNotifier,
// not part of the public API of this package.
type payload struct {
	// Text is the human-readable alert message.
	// `json:"text"` means this field appears as "text" in the JSON — not "Text".
	Text string `json:"text"`

	// Site is the name of the target that went down.
	Site string `json:"site"`

	// URL is the address that went down.
	URL string `json:"url"`

	// Error holds the error message, if any (e.g. "connection refused").
	Error string `json:"error,omitempty"` // omitempty — omit this field if it's empty
}

// Notify implements the Notifier interface for WebhookNotifier.
// It builds a JSON payload and POSTs it to the configured webhook URL.
//
// Note: value receiver (WebhookNotifier, not *WebhookNotifier) — no mutex inside,
// so copying is safe and a pointer is unnecessary.
func (n WebhookNotifier) Notify(result checker.Result) error {
	// Build the message text — what humans will see in Slack/Discord/etc.
	text := fmt.Sprintf("🚨 %s is DOWN — %s", result.Target.Name, result.Target.URL)
	if result.Error != "" {
		text += fmt.Sprintf(" — Error: %s", result.Error)
	} else {
		text += fmt.Sprintf(" — Status: %d", result.StatusCode)
	}

	// Build the payload struct we'll send as JSON.
	p := payload{
		Text:  text,
		Site:  result.Target.Name,
		URL:   result.Target.URL,
		Error: result.Error,
	}

	// --- json.Marshal ---
	//
	// Converts our struct to a JSON []byte — like json_encode() in PHP.
	// e.g. {"text":"🚨 Fake Site is DOWN...","site":"Fake Site","url":"..."}
	//
	// Unlike json.NewEncoder(w).Encode() which writes directly to a writer,
	// json.Marshal gives us the bytes in memory so we can wrap them as a Reader
	// for the HTTP request body.
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to marshal notification payload: %w", err)
	}

	// --- bytes.NewReader ---
	//
	// http.Post() needs an io.Reader as the body — something it can read bytes FROM.
	// bytes.NewReader wraps our []byte as an io.Reader.
	// Like passing a string body to Guzzle: $client->post($url, ['body' => $json])
	resp, err := http.Post(n.URL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to send notification to %s: %w", n.URL, err)
	}
	defer resp.Body.Close()

	// Check for non-2xx response — the webhook accepted the request but rejected it.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned unexpected status %d", resp.StatusCode)
	}

	return nil
}
