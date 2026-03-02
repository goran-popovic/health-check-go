// Package store holds the latest check results in memory and protects them
// with a read/write mutex so the ticker goroutine and HTTP handler goroutine
// can safely access the same data at the same time.
//
// In Laravel terms: think of this as a Repository — a single source of truth
// for the latest results, with built-in concurrency protection.
package store

import (
	"sync" // sync.RWMutex — the read/write lock

	"github.com/goran-popovic/go-health-check/checker"
)

// Store holds the latest check results behind a read/write mutex.
//
// WHY A POINTER RECEIVER (*Store) ON ALL METHODS:
// The Store contains a sync.RWMutex. As we covered with *log.Logger,
// a mutex must NEVER be copied — copying it gives each copy its own
// independent lock, which breaks thread safety entirely.
// Using pointer receivers (*Store) guarantees all method calls operate
// on the original Store in memory, never on a copy.
//
// This is also why New() returns *Store (a pointer) rather than Store (a value).
type Store struct {
	// mu protects the results slice below.
	// RWMutex is smarter than a plain Mutex:
	//   - Multiple goroutines can READ simultaneously (RLock/RUnlock)
	//   - Only ONE goroutine can WRITE at a time (Lock/Unlock), and
	//     while writing, all readers are blocked too.
	// Perfect for our case: many browser requests read, one ticker writes.
	//
	// PHP parallel: MySQL's shared read lock (SELECT) vs exclusive write lock (UPDATE).
	mu sync.RWMutex

	// results holds the last set of check results.
	// It starts as nil — before the first check runs,
	// the dashboard will show a "no results yet" message.
	results []checker.Result
}

// New creates and returns a pointer to an empty Store.
//
// We return *Store (pointer) not Store (value) — because Store contains
// a mutex and must never be copied. Everyone who uses the store shares
// this one pointer, so they all coordinate through the same lock.
//
// PHP parallel: like a singleton service — one instance, shared everywhere.
func New() *Store {
	return &Store{}
}

// Update replaces the stored results with a fresh set.
// Called by the ticker goroutine after every round of checks.
//
// Uses Lock/Unlock (exclusive writer lock) — while we're writing,
// no other goroutine can read or write. This is brief (just a slice
// assignment), so readers are only blocked for a tiny moment.
func (s *Store) Update(results []checker.Result) {
	// Lock claims exclusive write access.
	// Any goroutine currently reading (RLock) will finish first,
	// then we proceed. Any new reader trying to RLock will wait
	// until we call Unlock.
	s.mu.Lock()

	// defer Unlock so it always runs when Update() returns,
	// even if something panics — like finally{} in PHP.
	defer s.mu.Unlock()

	// Replace the stored slice with the new results.
	// This is the only "dangerous" line — without the lock above,
	// a reader could see a half-written slice.
	s.results = results
}

// Latest returns a copy of the current results.
// Called by the HTTP handler on every browser request.
//
// Uses RLock/RUnlock (shared reader lock) — multiple browser requests
// can call Latest() at the same time without blocking each other.
// Only blocked if Update() is currently holding the write lock.
//
// WHY WE RETURN A COPY, NOT THE SLICE DIRECTLY:
// If we returned s.results directly, the caller would hold a reference
// to our internal slice. After we release the lock, the ticker could
// call Update() and replace s.results — but the caller still holds
// the old reference, now pointing at unprotected memory.
// Returning a copy means the caller owns their data independently.
func (s *Store) Latest() []checker.Result {
	// RLock claims shared read access.
	// Multiple goroutines can hold RLock simultaneously — they're all
	// just reading, so they can't interfere with each other.
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Make a copy of the slice so the caller gets their own independent data.
	// copy() is a built-in — like array_slice() in PHP but into a pre-allocated slice.
	cp := make([]checker.Result, len(s.results))
	copy(cp, s.results)
	return cp
}
