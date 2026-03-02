# Go Health Check

A lightweight uptime monitor written in Go. It periodically checks a list of URLs and reports whether each one is up or down, how long it took to respond, and what HTTP status code was returned.

## Features

- Checks multiple URLs **concurrently** using goroutines
- Configurable check interval via environment variables
- Per-request **timeout** so a hanging server never blocks the whole checker
- Results printed to the terminal with timestamps

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

4. Edit `.env` with your targets and preferred interval.

## Configuration

All configuration is done via the `.env` file:

| Variable | Description | Example |
|---|---|---|
| `CHECK_INTERVAL_SECONDS` | How often to run checks | `30` |
| `TARGETS` | Comma-separated list of `Name=URL` pairs | `Google=https://www.google.com,GitHub=https://github.com` |

## Running

```
go run main.go
```

Example output:
```
Go Health Check starting...
Monitoring 2 targets every 30s

--- Checking at 12:00:00 ---
✓ Google (https://www.google.com) — 200 — 312ms
✓ GitHub (https://github.com) — 200 — 451ms
```

Stop with `Ctrl+C`.

## Running Tests

```
go test ./...
```

## Project Structure

```
go-health-check/
    main.go          — entry point, ticker loop
    checker/
        checker.go   — Target and Result structs, Check() and CheckAll()
        checker_test.go
    config/
        config.go    — loads and parses environment variables
        config_test.go
```
