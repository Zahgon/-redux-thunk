package redux_test

import (
	"testing"

	"github.com/reduxjs/redux-thunk-go/redux"
)

// Dispatch pays for goroutine identity in order to keep Redux's reentrancy
// guards meaningful under concurrency. That cost grows with stack depth, so it
// is measured rather than assumed -- BenchmarkDispatchFromDeepStack is the one
// that matters for thunks, which dispatch from inside middleware chains.
func BenchmarkDispatch(b *testing.B) {
	store := redux.CreateStore(counterReducer)
	action := redux.NewAction("INCREMENT", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		store.Dispatch(action)
	}
}

func BenchmarkDispatchFromDeepStack(b *testing.B) {
	store := redux.CreateStore(counterReducer)
	action := redux.NewAction("INCREMENT", nil)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		atDepth(64, func() { store.Dispatch(action) })
	}
}

func BenchmarkGetState(b *testing.B) {
	store := redux.CreateStore(counterReducer)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = store.GetState()
	}
}

//go:noinline
func atDepth(n int, fn func()) {
	if n == 0 {
		fn()
		return
	}
	atDepth(n-1, fn)
}
