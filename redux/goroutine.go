package redux

import (
	"runtime"
	"sync"
)

// goID returns an identifier for the calling goroutine, or 0 if it cannot be
// determined.
//
// Redux's reentrancy guards ("Reducers may not dispatch actions.", "You may not
// call store.subscribe() while the reducer is executing.") rely on JavaScript
// being single-threaded: if the flag is set, *you* are the one inside the
// reducer. Go removes that guarantee -- a second goroutine dispatching at the
// same time would trip a naive flag and get an error describing something it
// did not do.
//
// Scoping the flag to a goroutine restores the original meaning exactly:
// concurrent dispatches serialise on the store's dispatch lock, while a
// dispatch made *from inside your own reducer* still fails loudly.
//
// Go exposes no goroutine identity, so this parses the header line that
// runtime.Stack always emits first:
//
//	goroutine 42 [running]:
//
// COST: this is the most expensive step in Dispatch. runtime.Stack unwinds the
// whole stack regardless of how small the buffer is, so the price grows with
// call depth -- roughly 1.8us at shallow depth and an order of magnitude more
// at depth 64, which matters because thunks dispatch from inside middleware
// chains. It is therefore called at most once per Dispatch (the result is
// threaded through rather than recomputed) and never at all on GetState,
// Subscribe or unsubscribe unless a reducer is actually running. The buffer is
// pooled and the digits are parsed in place so the call does not allocate.
// redux/bench_test.go tracks all of this.
func goID() uint64 {
	bufp, _ := goIDBuffers.Get().(*[40]byte)
	n := runtime.Stack(bufp[:], false)
	id := parseGoroutineID(bufp[:n])
	goIDBuffers.Put(bufp)
	return id
}

var goIDBuffers = sync.Pool{New: func() any { return new([40]byte) }}

func parseGoroutineID(header []byte) uint64 {
	const prefix = "goroutine "
	if len(header) < len(prefix) {
		return 0
	}
	digits := header[len(prefix):]

	end := 0
	for end < len(digits) && digits[end] >= '0' && digits[end] <= '9' {
		end++
	}

	var id uint64
	for _, c := range digits[:end] {
		id = id*10 + uint64(c-'0')
	}
	if end == 0 {
		return 0
	}
	return id
}
