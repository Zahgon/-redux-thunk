// Tests in this file have no counterpart in the TypeScript source. JavaScript
// runs redux-thunk on a single-threaded event loop, so upstream never had to
// answer the questions Go forces: is a thunk-enabled store safe to dispatch to
// from several goroutines, and does an async thunk that resolves on another
// goroutine still see a consistent store?
//
// They exist to keep the port honest under `go test -race`.
package thunk_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/async"
	"github.com/reduxjs/redux-thunk-go/redux"
)

type tally struct {
	Count int
}

func tallyReducer(state tally, action any) tally {
	if a, ok := action.(redux.Action); ok && a.ActionType() == "BUMP" {
		return tally{Count: state.Count + 1}
	}
	return state
}

func newTallyStore() redux.Store[tally] {
	return redux.CreateStore(tallyReducer, redux.WithEnhancer(
		redux.ApplyMiddleware(thunk.TypedThunk[tally]().AsMiddleware())))
}

func TestConcurrentPlainDispatches(t *testing.T) {
	store := newTallyStore()

	const goroutines = 64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			store.Dispatch(redux.NewAction("BUMP", nil))
		}()
	}
	wg.Wait()

	if store.GetState().Count != goroutines {
		t.Errorf("expected %d, got %d", goroutines, store.GetState().Count)
	}
}

func TestConcurrentThunkDispatches(t *testing.T) {
	store := newTallyStore()

	const goroutines = 64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			store.Dispatch(func(dispatch redux.Dispatch, _ func() tally, _ any) any {
				return dispatch(redux.NewAction("BUMP", nil))
			})
		}()
	}
	wg.Wait()

	if store.GetState().Count != goroutines {
		t.Errorf("expected %d, got %d", goroutines, store.GetState().Count)
	}
}

func TestThunkGetStateIsSafeFromOtherGoroutines(t *testing.T) {
	store := newTallyStore()
	var observed int64

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			store.Dispatch(func(dispatch redux.Dispatch, getState func() tally, _ any) any {
				dispatch(redux.NewAction("BUMP", nil))
				atomic.AddInt64(&observed, int64(getState().Count))
				return nil
			})
		}()
	}
	wg.Wait()

	if store.GetState().Count != goroutines {
		t.Errorf("expected %d, got %d", goroutines, store.GetState().Count)
	}
	if atomic.LoadInt64(&observed) == 0 {
		t.Error("expected every thunk to have observed a non-zero count")
	}
}

// TestAsyncThunkResolvesOffTheDispatchGoroutine is the Go shape of the README's
// `incrementAsync` sample: the thunk returns immediately and the follow-up
// dispatch happens later, on another goroutine.
func TestAsyncThunkResolvesOffTheDispatchGoroutine(t *testing.T) {
	store := newTallyStore()

	bumpLater := func(dispatch redux.Dispatch, _ func() tally, _ any) any {
		return async.New(func() (any, error) {
			dispatch(redux.NewAction("BUMP", nil))
			return nil, nil
		})
	}

	future, ok := store.Dispatch(bumpLater).(*async.Future[any])
	if !ok {
		t.Fatal("expected the thunk's Future to be returned by dispatch")
	}
	if _, err := future.Await(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.GetState().Count != 1 {
		t.Errorf("expected 1, got %d", store.GetState().Count)
	}
}

func TestConcurrentSubscribeAndDispatch(t *testing.T) {
	store := newTallyStore()
	var notifications int64

	var wg sync.WaitGroup
	wg.Add(64)
	for i := range 64 {
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				unsubscribe := store.Subscribe(func() { atomic.AddInt64(&notifications, 1) })
				defer unsubscribe()
				return
			}
			store.Dispatch(redux.NewAction("BUMP", nil))
		}()
	}
	wg.Wait()

	if store.GetState().Count != 32 {
		t.Errorf("expected 32, got %d", store.GetState().Count)
	}
}
