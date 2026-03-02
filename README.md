# Go Health Check

A lightweight uptime monitor written in Go. It periodically checks a list of URLs and reports whether each one is up or down, how long it took to respond, and what HTTP status code was returned.

## Features

- Checks multiple URLs **concurrently** using goroutines
- Configurable check interval via environment variables
- Per-request **timeout** so a hanging server never blocks the whole checker
- Results printed to the terminal with timestamps
- Writes results to a **log file** (appended on every run)
- Serves a live **HTTP dashboard** at `/status` showing current state
- Sends **webhook notifications** (Slack, Discord, etc.) when a site goes down

## Requirements

- Go 1.22+

## Setup

1. Clone the repository:
   ```
   git clone git@github.com:goran-popovic/health-check-go.git
   cd health-check-go
   ```

2. Install dependencies:
   ```
   go mod download
   ```

3. Copy the example env file and configure it:
   ```
   cp .env.example .env
   ```

4. Edit `.env` with your targets and preferred settings.

## Configuration

All configuration is done via the `.env` file:

| Variable | Description | Default | Example |
|---|---|---|---|
| `CHECK_INTERVAL_SECONDS` | How often to run checks | — | `30` |
| `TARGETS` | Comma-separated list of `Name=URL` pairs | — | `Google=https://www.google.com,GitHub=https://github.com` |
| `LOG_FILE` | Path to the log file | `health-check.log` | `health-check.log` |
| `HTTP_PORT` | Port for the HTTP status dashboard | `8080` | `8080` |
| `WEBHOOK_URL` | Webhook endpoint for down alerts (leave empty to disable) | — | `https://hooks.slack.com/services/...` |

## Running

```
make run
```

Or without Make:

```
go run .
```

Example output:
```
Go Health Check starting...
2009/11/10 12:00:00 HTTP dashboard starting on http://localhost:8080/status
2009/11/10 12:00:00 Monitoring 2 targets every 30s — logging to health-check.log
2009/11/10 12:00:00 --- Checking at 12:00:00 ---
2009/11/10 12:00:00 ✓ Google (https://www.google.com) — 200 — 312ms
2009/11/10 12:00:00 ✓ GitHub (https://github.com) — 200 — 451ms
```

Stop with `Ctrl+C`.

The HTTP dashboard is available at `http://localhost:8080/status` while the app is running.

## Building

Use Make to compile a binary:

```
# Build for the current OS (Windows → .exe)
make build

# Cross-compile for Linux (amd64)
make linux

# Cross-compile for macOS (Apple Silicon)
make mac-arm

# Clean up compiled binaries
make clean
```

Binaries are stripped of debug symbols (`-ldflags="-s -w"`) to reduce file size by ~30%.

## Running Tests

```
make test
```

Or without Make:

```
go test ./...
```

### Race Detector

To run tests with the race detector (detects concurrent access bugs):

```
make race
```

> **Windows note**: the race detector requires CGO and a C compiler (gcc).
> Install MinGW via [Chocolatey](https://chocolatey.org/): `choco install mingw`
> Then add `C:\ProgramData\mingw64\mingw64\bin` to your system PATH.

## Project Structure

```
go-health-check/
    main.go              — entry point, ticker loop, HTTP server, runChecks()
    Makefile             — build, test, cross-compile, and run shortcuts
    checker/
        checker.go       — Target and Result structs, Check() and CheckAll()
        checker_test.go
    config/
        config.go        — loads and parses environment variables
        config_test.go
    logger/
        logger.go        — creates a logger that writes to stdout and a log file
    notifier/
        notifier.go      — Notifier interface, WebhookNotifier implementation
        notifier_test.go
    store/
        store.go         — thread-safe in-memory store for latest check results
        store_test.go
```
