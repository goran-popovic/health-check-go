// main.go is the entry point of the application.
// Execution starts here — like index.php in Laravel.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/goran-popovic/go-health-check/checker"
	"github.com/goran-popovic/go-health-check/config"
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

	// Convert the interval integer to a time.Duration.
	interval := time.Duration(cfg.IntervalSeconds) * time.Second

	fmt.Printf("Monitoring %d targets every %s\n", len(cfg.Targets), interval)

	// Create a ticker with the configured interval.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run the first check immediately — before the first tick.
	runChecks(cfg.Targets)

	// Then repeat on every tick — runs forever until Ctrl+C.
	for range ticker.C {
		runChecks(cfg.Targets)
	}
}

// runChecks runs all checks concurrently and prints the results.
func runChecks(targets []checker.Target) {
	fmt.Printf("\n--- Checking at %s ---\n", time.Now().Format("15:04:05"))

	results := checker.CheckAll(targets)

	for _, result := range results {
		fmt.Println(result)
	}
}
