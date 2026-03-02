// main.go is the entry point of the application.
// Execution starts here — like index.php in Laravel.
package main

import (
	"fmt"
	"log"
	"net/http" // Go's built-in HTTP server — no framework needed
	"time"

	"github.com/joho/godotenv"
	"github.com/goran-popovic/go-health-check/checker"
	"github.com/goran-popovic/go-health-check/config"
	"github.com/goran-popovic/go-health-check/logger"
	"github.com/goran-popovic/go-health-check/notifier"
	"github.com/goran-popovic/go-health-check/store"
)

func main() {
	fmt.Println("Go Health Check starting...")

	// Load .env file into the environment — like Laravel's .env loading.
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file: ", err)
	}

	// Load all config from environment variables via our config package.
	// config.Load() returns (Config, error) — two values.
	cfg, err := config.Load()
	if err != nil {
		// log.Fatal prints the error and exits — like die() in PHP.
		log.Fatal("Configuration error: ", err)
	}

	// Set up the logger — it writes to both stdout and the log file.
	// logger.New returns three values:
	//   l       — the *log.Logger we use to write lines
	//   cleanup — a func() that closes the file when we're done
	//   err     — non-nil if the file couldn't be opened
	l, cleanup, err := logger.New(cfg.LogFile)
	if err != nil {
		log.Fatal("Logger error: ", err)
	}
	// defer the cleanup so the log file is properly closed when main() exits.
	// Even if the app crashes, Go will run deferred functions — like finally{} in PHP.
	defer cleanup()

	// Create the shared store — a single source of truth for the latest results.
	// Both the ticker goroutine (writer) and the HTTP handler (reader) will use
	// this same *store.Store pointer, coordinating access through its RWMutex.
	//
	// PHP parallel: like binding a singleton in Laravel's service container —
	// one instance shared across the whole application.
	st := store.New()

	// --- Set up the notifier ---
	//
	// We declare n as the Notifier INTERFACE type, not the concrete WebhookNotifier type.
	// This is the key to Go interfaces — runChecks() will accept a Notifier and call
	// n.Notify(), without knowing or caring which concrete type is behind it.
	//
	// If WEBHOOK_URL is empty, n stays nil — runChecks() checks for nil before calling.
	// This is Go's way of making a feature optional without extra config flags.
	var n notifier.Notifier
	if cfg.WebhookURL != "" {
		// WebhookNotifier satisfies the Notifier interface because it has a Notify() method
		// with the right signature — Go figures this out automatically, no declaration needed.
		n = notifier.WebhookNotifier{URL: cfg.WebhookURL}
		l.Printf("Notifications enabled — webhook: %s", cfg.WebhookURL)
	} else {
		l.Println("Notifications disabled — set WEBHOOK_URL to enable")
	}

	// --- Start the HTTP server in a goroutine ---
	//
	// http.HandleFunc registers a handler for a URL path — like Route::get() in Laravel.
	// The second argument is the handler function. We use handleStatus(st) which
	// returns an http.HandlerFunc — explained below.
	//
	// http.ListenAndServe starts the server and blocks forever (like nginx listening).
	// We run it in a goroutine so it doesn't block the ticker loop below.
	// The 'go' keyword means: start this in the background and move on immediately.
	//
	// If the server fails to start (e.g. port already in use), log.Fatal exits.
	// We wrap it in a goroutine but still want to catch startup errors — so we
	// call log.Fatal inside the goroutine, which will kill the whole program.
	go func() {
		addr := ":" + cfg.HTTPPort
		l.Printf("HTTP dashboard starting on http://localhost%s/status", addr)

		// http.ListenAndServe blocks until the server dies.
		// nil means use the default ServeMux (the global route registry
		// where http.HandleFunc registered our /status route above).
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatal("HTTP server error: ", err)
		}
	}()

	// Register the /status route BEFORE starting the server above.
	// handleStatus(st) returns a func(ResponseWriter, *Request) — the handler.
	// We pass st so the handler can call st.Latest() to read the results.
	http.HandleFunc("/status", handleStatus(st))

	l.Printf("Monitoring %d targets every %ds — logging to %s\n",
		len(cfg.Targets), cfg.IntervalSeconds, cfg.LogFile)

	// Convert the interval integer to a time.Duration.
	interval := time.Duration(cfg.IntervalSeconds) * time.Second

	// Create a ticker with the configured interval.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run the first check immediately — before the first tick.
	runChecks(cfg.Targets, l, st, n)

	// Then repeat on every tick — runs forever until Ctrl+C.
	for range ticker.C {
		runChecks(cfg.Targets, l, st, n)
	}
}

// runChecks runs all checks concurrently, logs the results, saves them to the
// store, and fires notifications for any sites that are down.
//
// n is a notifier.Notifier interface — it can be nil if notifications are disabled,
// or any concrete type (WebhookNotifier, future EmailNotifier, etc.).
// runChecks doesn't know or care which implementation it gets.
func runChecks(targets []checker.Target, l *log.Logger, st *store.Store, n notifier.Notifier) {
	l.Printf("--- Checking at %s ---", time.Now().Format("15:04:05"))

	results := checker.CheckAll(targets)

	for _, result := range results {
		l.Println(result)

		// Send a notification if the site is down and a notifier is configured.
		// n == nil means WEBHOOK_URL was empty — skip silently.
		//
		// In Go, calling a method on a nil interface panics — so we always check first.
		// This is different from a nil pointer: a nil interface has no type AND no value.
		if !result.Up && n != nil {
			if err := n.Notify(result); err != nil {
				// Log the error but don't stop — a failed notification shouldn't
				// bring down the whole monitoring loop.
				l.Printf("Notification error for %s: %v", result.Target.Name, err)
			}
		}
	}

	// Save the latest results to the store.
	// st.Update() takes the write lock internally — safe to call from any goroutine.
	st.Update(results)
}

// handleStatus returns an HTTP handler function that renders the status dashboard.
//
// WHY DOES THIS RETURN A FUNCTION INSTEAD OF BEING A HANDLER DIRECTLY?
// http.HandleFunc expects a func(ResponseWriter, *Request) — a plain function
// with no extra parameters. But our handler needs access to *store.Store.
//
// The solution is a "closure" — handleStatus takes *store.Store as a parameter
// and returns a new function that "closes over" (remembers) the store.
// The returned function has the right signature for http.HandleFunc, but still
// has access to st via the closure.
//
// PHP parallel: like a controller action that has access to injected dependencies
// via the constructor — the handler function is the action, handleStatus is
// the constructor that injects the store.
func handleStatus(st *store.Store) http.HandlerFunc {
	// http.HandlerFunc is just a type alias for func(ResponseWriter, *Request).
	// Returning it makes the intent explicit.
	return func(w http.ResponseWriter, r *http.Request) {
		results := st.Latest()

		// Set the Content-Type header so the browser renders it as HTML.
		// Like header('Content-Type: text/html') in PHP.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// fmt.Fprintf writes formatted output to w (the response body).
		// w implements io.Writer — just like our log file did.
		// Like echo in PHP.
		fmt.Fprintf(w, "<html><body>\n")
		fmt.Fprintf(w, "<h1>Health Check Dashboard</h1>\n")
		fmt.Fprintf(w, "<p>Last updated: %s</p>\n", time.Now().Format("2006-01-02 15:04:05"))

		if len(results) == 0 {
			// Before the first ticker fires, the store is empty.
			fmt.Fprintf(w, "<p>No results yet — first check in progress...</p>\n")
			fmt.Fprintf(w, "</body></html>\n")
			return
		}

		fmt.Fprintf(w, "<table border='1' cellpadding='8'>\n")
		fmt.Fprintf(w, "<tr><th>Name</th><th>URL</th><th>Status</th><th>Code</th><th>Duration</th></tr>\n")

		for _, r := range results {
			// Choose a background colour based on whether the site is up.
			// Green for up, red for down — like a simple status badge.
			color := "#c8f7c5" // light green
			status := "UP"
			if !r.Up {
				color = "#f7c5c5" // light red
				status = "DOWN"
			}

			fmt.Fprintf(w,
				"<tr style='background:%s'><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%v</td></tr>\n",
				color, r.Target.Name, r.Target.URL, status, r.StatusCode, r.Duration,
			)
		}

		fmt.Fprintf(w, "</table>\n")
		fmt.Fprintf(w, "</body></html>\n")
	}
}
