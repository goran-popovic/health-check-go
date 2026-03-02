// Package config handles loading and parsing of application configuration.
// In Laravel terms, this is like your config/ folder — it reads from the
// environment and returns structured config values your app can use.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings" // for strings.Split and strings.TrimSpace

	"github.com/goran-popovic/go-health-check/checker"
)

// Config holds all configuration for the application.
// This is a struct — like a plain PHP object with only properties.
// Having a single Config struct means main.go gets one clean value
// back instead of reading many separate variables.
type Config struct {
	// IntervalSeconds is how often to run checks.
	IntervalSeconds int

	// Targets is the list of URLs to monitor.
	Targets []checker.Target

	// LogFile is the path to the file where results are written.
	// e.g. "health-check.log"
	LogFile string

	// HTTPPort is the port the status dashboard listens on.
	// e.g. "8080" → http://localhost:8080/status
	HTTPPort string

	// WebhookURL is the endpoint to POST alerts to when a site goes down.
	// Empty string means notifications are disabled.
	WebhookURL string
}

// Load reads all config from environment variables and returns a Config.
//
// It returns (Config, error) — two values, as is standard in Go.
// If anything goes wrong (e.g. no targets defined), we return an error
// and let main.go decide what to do (log.Fatal, use a default, etc.)
//
// In Laravel: this would be config('services.targets') reading from
// config/services.php which itself reads from .env.
func Load() (Config, error) {
	// --- Interval ---

	intervalSeconds := 10 // default value if env var is missing or invalid

	intervalStr := os.Getenv("CHECK_INTERVAL_SECONDS")
	if intervalStr != "" {
		// strconv.Atoi converts string → int, returns error if invalid.
		val, err := strconv.Atoi(intervalStr)
		if err != nil || val <= 0 {
			// We don't fatal here — just warn and use the default.
			// fmt.Println to stderr would be better in production, but fine for now.
			fmt.Println("Warning: invalid CHECK_INTERVAL_SECONDS, using default of 10s")
		} else {
			intervalSeconds = val
		}
	}

	// --- Targets ---

	// os.Getenv returns "" if the variable isn't set.
	targetsStr := os.Getenv("TARGETS")
	if targetsStr == "" {
		// If no targets are defined, return an error — the app can't do anything useful.
		// fmt.Errorf creates a new error with a formatted message.
		// In PHP: throw new \RuntimeException("No targets configured")
		return Config{}, fmt.Errorf("no targets configured — set the TARGETS env variable")
	}

	// Parse the comma-separated list of targets.
	// e.g. "Google=https://google.com,GitHub=https://github.com"
	targets, err := parseTargets(targetsStr)
	if err != nil {
		return Config{}, err
	}

	// --- Log file ---

	// os.Getenv returns "" if the variable isn't set — we use a default in that case.
	logFile := os.Getenv("LOG_FILE")
	if logFile == "" {
		logFile = "health-check.log" // sensible default if not configured
	}

	// --- HTTP port ---

	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8080" // sensible default if not configured
	}

	// --- Webhook URL ---

	// No default here — empty string means "notifications disabled".
	// We don't validate the URL format; if it's wrong the notifier will return an error.
	webhookURL := os.Getenv("WEBHOOK_URL")

	return Config{
		IntervalSeconds: intervalSeconds,
		Targets:         targets,
		LogFile:         logFile,
		HTTPPort:        httpPort,
		WebhookURL:      webhookURL,
	}, nil
}

// parseTargets parses a comma-separated string of "Name=URL" pairs
// into a slice of checker.Target.
//
// This is an unexported function (lowercase) — it's a private helper,
// only usable within this package. Like a private method in PHP.
func parseTargets(raw string) ([]checker.Target, error) {
	// strings.Split splits a string by a delimiter — like explode() in PHP.
	// "Google=https://google.com,GitHub=https://github.com"
	// becomes: ["Google=https://google.com", "GitHub=https://github.com"]
	parts := strings.Split(raw, ",")

	// Pre-allocate a slice with the right capacity.
	targets := make([]checker.Target, 0, len(parts))

	for _, part := range parts {
		// strings.TrimSpace removes leading/trailing whitespace — like trim() in PHP.
		part = strings.TrimSpace(part)
		if part == "" {
			continue // skip empty entries (e.g. trailing comma)
		}

		// strings.SplitN splits by "=" but stops after N parts.
		// We use N=2 so that URLs containing "=" (e.g. query strings like
		// "https://example.com?q=hello") don't get split at the wrong "=".
		// It always splits on the FIRST "=" only, keeping the URL intact.
		// e.g. "My Site=https://example.com?q=hello" → ["My Site", "https://example.com?q=hello"]
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			// If there's no "=" in the part, it's malformed — return an error.
			return nil, fmt.Errorf("invalid target format %q — expected Name=URL", part)
		}

		name := strings.TrimSpace(kv[0])
		url := strings.TrimSpace(kv[1])

		if name == "" || url == "" {
			return nil, fmt.Errorf("invalid target %q — name and URL must not be empty", part)
		}

		// append adds to the slice — like $targets[] = $target in PHP.
		targets = append(targets, checker.Target{Name: name, URL: url})
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no valid targets found in TARGETS env variable")
	}

	return targets, nil
}
