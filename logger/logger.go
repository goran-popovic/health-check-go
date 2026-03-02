// Package logger sets up the application logger.
// It writes to both the terminal (stdout) AND a log file at the same time —
// so you can watch the output live AND have a persistent record on disk.
//
// In Laravel terms: this is like configuring Log::channel('stack') which
// fans log output to multiple handlers (daily file + stderr, etc.).
package logger

import (
	"fmt"
	"io"  // io.Writer — the interface for "anything you can write bytes to"
	"log" // Go's built-in logging package
	"os"  // os.OpenFile — open/create files
)

// New creates a logger that writes to both stdout and a log file simultaneously.
//
// Return type breakdown:
//   - *log.Logger  — a POINTER to a Logger (explained below)
//   - func()       — a cleanup function that closes the file; call it with defer
//   - error        — non-nil if the file couldn't be opened
//
// --- Why *log.Logger (pointer) and not log.Logger (value)? ---
//
// In Go, when you pass or return a struct BY VALUE, Go makes a full copy of it.
// In PHP, objects are always reference-like — you never get a surprise copy.
// In Go, you have to be explicit.
//
// log.Logger contains a sync.Mutex inside it (a lock for thread safety).
// A mutex MUST never be copied — if you copy it, each copy has its own
// independent lock, which breaks thread safety completely.
// Imagine two goroutines each holding a different copy of the lock —
// they'd both think they own it, and writes to the file would collide.
//
// By returning *log.Logger (a pointer), we guarantee:
//   1. No copy is ever made — everyone shares the same underlying instance.
//   2. The mutex inside it stays intact and works correctly.
//   3. The caller's l.Println() calls all go to the same writer.
//
// PHP parallel: it's like returning $this vs returning clone $this.
// A pointer = $this (same object). A value = clone $this (independent copy).
//
// Contrast this with our own structs (Target, Result, Config) which are
// returned BY VALUE because they are plain data — no mutexes, no shared
// state, and copying them is perfectly safe and intentional.
func New(filePath string) (*log.Logger, func(), error) {
	// --- os.OpenFile ---
	//
	// Opens a file with specific flags controlling how it's opened.
	// Like fopen() in PHP, but more explicit about the mode.
	//
	// Flags (combined with | — bitwise OR, like combining fopen modes):
	//   os.O_CREATE  — create the file if it doesn't exist (like 'a' in fopen)
	//   os.O_APPEND  — always write at the end, never overwrite (like 'a' in fopen)
	//   os.O_WRONLY  — open for writing only (we don't need to read the log file here)
	//
	// The third argument (0644) is the Unix file permission for newly created files:
	//   6 = owner can read+write, 4 = group can read, 4 = others can read
	//   Like chmod 644 in terminal. On Windows this is largely ignored.
	//
	// os.OpenFile returns *os.File (a pointer) — same reason as *log.Logger above:
	// *os.File wraps a real OS file handle. Copying it would give two variables
	// pointing at the same underlying OS resource, which causes double-close bugs.
	// The pointer makes ownership clear: one variable, one file handle.
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open log file %q: %w", filePath, err)
	}

	// --- io.MultiWriter ---
	//
	// io.MultiWriter takes multiple io.Writer values and returns a single writer
	// that fans every write out to ALL of them simultaneously.
	//
	// io.Writer is an interface — any type that has a Write([]byte) method qualifies.
	// Both *os.File (our log file) and os.Stdout implement io.Writer, so both work here.
	//
	// This is Go's version of Monolog's "stack" channel — one write goes everywhere.
	multi := io.MultiWriter(os.Stdout, file)

	// --- log.New ---
	//
	// Creates a new Logger that writes to the given io.Writer.
	// Arguments:
	//   multi             — our multi-writer (stdout + file)
	//   ""                — prefix string added before every log line (we use none)
	//   log.Ldate|log.Ltime — flags that auto-prepend date and time to each line
	//
	// log.Ldate = "2009/11/10"  and  log.Ltime = "23:00:00"
	// Together they produce: "2009/11/10 23:00:00 <your message>"
	//
	// log.New returns *log.Logger — a pointer — for the mutex reason explained above.
	// We just pass that pointer straight through to our caller.
	logger := log.New(multi, "", log.Ldate|log.Ltime)

	// The cleanup function closes the file when the caller is done.
	// We return it so the caller can defer it — like returning a Closeable in Java,
	// or a destructor in PHP that flushes/closes the file handle.
	//
	// Note: 'file' here is *os.File (a pointer). The closure captures the pointer,
	// not a copy of the file — so file.Close() closes the real underlying file handle.
	cleanup := func() {
		file.Close()
	}

	return logger, cleanup, nil
}
