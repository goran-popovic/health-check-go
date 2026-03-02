package store

// We test in the same package (package store, not package store_test) so we
// can inspect internal state directly if needed — same pattern as our other tests.

import (
	"sync"     // sync.WaitGroup — explained in the concurrent test below
	"testing"

	"github.com/goran-popovic/go-health-check/checker"
)

// TestStore_UpdateAndLatest verifies the basic round-trip:
// whatever we put in with Update(), we should get back with Latest().
func TestStore_UpdateAndLatest(t *testing.T) {
	st := New()

	// Build a small slice of results to store.
	input := []checker.Result{
		{Target: checker.Target{Name: "Google", URL: "https://google.com"}, StatusCode: 200, Up: true},
		{Target: checker.Target{Name: "Fake", URL: "https://fake.com"}, StatusCode: 0, Up: false},
	}

	st.Update(input)

	got := st.Latest()

	// Check we got the right number of results back.
	if len(got) != len(input) {
		t.Fatalf("expected %d results, got %d", len(input), len(got))
	}

	// Check each result matches what we put in.
	for i, r := range got {
		if r.Target.Name != input[i].Target.Name {
			t.Errorf("result[%d].Name: expected %q, got %q", i, input[i].Target.Name, r.Target.Name)
		}
		if r.Up != input[i].Up {
			t.Errorf("result[%d].Up: expected %v, got %v", i, input[i].Up, r.Up)
		}
	}
}

// TestStore_Latest_ReturnsCopy verifies that Latest() returns an independent copy
// of the internal slice — not a reference to it.
//
// This matters for thread safety: if Latest() returned the real internal slice,
// a caller could hold onto it and read from it while Update() is writing to it
// in another goroutine — a race condition even with our mutex.
// By returning a copy, the caller owns their own data completely.
func TestStore_Latest_ReturnsCopy(t *testing.T) {
	st := New()

	st.Update([]checker.Result{
		{Target: checker.Target{Name: "Original"}, Up: true},
	})

	// Get the slice back from the store.
	got := st.Latest()

	// Mutate the returned slice — change the first element.
	got[0].Target.Name = "Mutated"

	// Now call Latest() again — if we got a copy, the store's internal slice
	// should still have the original value "Original", not "Mutated".
	fresh := st.Latest()

	if fresh[0].Target.Name != "Original" {
		t.Errorf("store was mutated through returned slice — Latest() must return a copy, got %q", fresh[0].Target.Name)
	}
}

// TestStore_Concurrent verifies that the store is safe to use from multiple
// goroutines simultaneously — the main guarantee of our RWMutex.
//
// This test is most useful when run with the race detector:
//   go test -race ./store/...
//
// Without the mutex, the race detector would catch concurrent reads/writes
// and report a data race. With the mutex, everything is properly serialised.
//
// --- sync.WaitGroup ---
// A WaitGroup is a counter for goroutines. You:
//   wg.Add(n)  — say "I'm about to launch n goroutines"
//   wg.Done()  — called by each goroutine when it finishes (decrements the counter)
//   wg.Wait()  — block until the counter reaches zero (all goroutines finished)
//
// PHP parallel: imagine Promise::all() — wait for all async tasks to complete
// before moving on.
func TestStore_Concurrent(t *testing.T) {
	st := New()

	// Seed the store with an initial result so Latest() has something to read.
	st.Update([]checker.Result{
		{Target: checker.Target{Name: "Test"}, Up: true},
	})

	var wg sync.WaitGroup

	// Launch 10 writer goroutines — each calls Update() with a fresh result.
	// In production this would be just the ticker, but more writers stress-tests the lock.
	for i := 0; i < 10; i++ {
		wg.Add(1) // tell the WaitGroup we're launching one more goroutine
		go func() {
			defer wg.Done() // signal when this goroutine finishes
			st.Update([]checker.Result{
				{Target: checker.Target{Name: "Test"}, Up: true},
			})
		}()
	}

	// Launch 10 reader goroutines — each calls Latest() concurrently with the writers.
	// RWMutex allows multiple readers simultaneously, but each writer gets exclusive access.
	// If the locking were wrong, the race detector would flag this immediately.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = st.Latest() // we don't care about the value, just that it doesn't race
		}()
	}

	// Block until all 20 goroutines have finished.
	// Without this, the test function would return and the goroutines would be killed
	// before the race detector had a chance to observe any problems.
	wg.Wait()
}
