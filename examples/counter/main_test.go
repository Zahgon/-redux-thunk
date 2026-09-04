package main

import (
	"testing"

	"github.com/reduxjs/redux-thunk-go/redux"
)

// The reducer here matches on the concrete action struct, so nothing else in
// this example ever calls ActionType. The string it returns is still part of
// the contract -- it is the `INCREMENT_COUNTER` constant the upstream README
// publishes -- so it is pinned here rather than left to drift.
func TestIncrementActionCarriesTheUpstreamActionType(t *testing.T) {
	var action redux.Action = increment()

	if got := action.ActionType(); got != "INCREMENT_COUNTER" {
		t.Fatalf("ActionType() = %q, want %q", got, "INCREMENT_COUNTER")
	}
}

func Example() {
	run()
	// Output:
	// increment: 1
	// incrementIfOdd on odd counter: 2
	// incrementIfOdd on even counter: 2
	// incrementAsync: 3
}
