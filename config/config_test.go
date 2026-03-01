// config_test.go tests the config package.
// In Go, test files live next to the code they test, in the same package.
// The file must end in _test.go — Go ignores it in normal builds.
package config

import (
	"testing" // built-in testing package — no external library needed
)

// TestParseTargets_Valid tests that a well-formed TARGETS string is parsed correctly.
// All test functions must start with "Test" and take *testing.T as the argument.
// *testing.T is the "test reporter" — you call methods on it to fail tests.
func TestParseTargets_Valid(t *testing.T) {
	raw := "Google=https://www.google.com,GitHub=https://github.com"

	// parseTargets is unexported (lowercase) but since our test is in the
	// same package (package config), we can still call it directly.
	// This is like testing a private method in PHP by putting the test
	// in the same class — except Go makes it cleaner with package scope.
	targets, err := parseTargets(raw)

	// If parsing returned an error, the test should fail immediately.
	// t.Fatalf is like t.Errorf but also stops the test right away —
	// no point continuing if parsing failed entirely.
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Check we got the right number of targets.
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	// Check the first target's fields.
	if targets[0].Name != "Google" {
		t.Errorf("expected targets[0].Name to be 'Google', got %q", targets[0].Name)
	}
	if targets[0].URL != "https://www.google.com" {
		t.Errorf("expected targets[0].URL to be 'https://www.google.com', got %q", targets[0].URL)
	}

	// Check the second target's fields.
	if targets[1].Name != "GitHub" {
		t.Errorf("expected targets[1].Name to be 'GitHub', got %q", targets[1].Name)
	}
	if targets[1].URL != "https://github.com" {
		t.Errorf("expected targets[1].URL to be 'https://github.com', got %q", targets[1].URL)
	}
}

// TestParseTargets_WithQueryString tests that URLs with "=" in query strings
// are handled correctly — the reason we use SplitN(..., 2).
func TestParseTargets_WithQueryString(t *testing.T) {
	raw := "Search=https://example.com?q=hello"

	targets, err := parseTargets(raw)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if targets[0].URL != "https://example.com?q=hello" {
		t.Errorf("expected full URL with query string, got %q", targets[0].URL)
	}
}

// TestParseTargets_Empty tests that an empty string returns an error.
func TestParseTargets_Empty(t *testing.T) {
	_, err := parseTargets("")

	// We expect an error here — if there's no error, the test should fail.
	if err == nil {
		t.Error("expected an error for empty input, got nil")
	}
}

// TestParseTargets_MissingURL tests that a malformed entry (no "=") returns an error.
func TestParseTargets_MissingURL(t *testing.T) {
	raw := "GoogleWithNoURL"

	_, err := parseTargets(raw)

	if err == nil {
		t.Error("expected an error for missing URL, got nil")
	}
}

// TestParseTargets_TrimsWhitespace tests that extra spaces are handled gracefully.
func TestParseTargets_TrimsWhitespace(t *testing.T) {
	// Spaces around the comma and around the "=" sign
	raw := "  Google  =  https://www.google.com  ,  GitHub  =  https://github.com  "

	targets, err := parseTargets(raw)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if targets[0].Name != "Google" {
		t.Errorf("expected trimmed name 'Google', got %q", targets[0].Name)
	}
	if targets[0].URL != "https://www.google.com" {
		t.Errorf("expected trimmed URL, got %q", targets[0].URL)
	}
}
