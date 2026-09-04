package redux_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/reduxjs/redux-thunk-go/redux"
)

// Redux keeps listeners in a JavaScript Map, which iterates in insertion order.
// Go randomises map iteration, so this is repeated: a single dispatch could pass
// by luck, but forty in a row cannot.
func TestListenersAreNotifiedInSubscriptionOrder(t *testing.T) {
	const (
		listeners = 8
		rounds    = 40
	)

	store := redux.CreateStore(counterReducer)
	var order []int

	for i := 0; i < listeners; i++ {
		store.Subscribe(func() { order = append(order, i) })
	}

	for round := 0; round < rounds; round++ {
		order = order[:0]
		store.Dispatch(redux.NewAction("INCREMENT", nil))

		for i, got := range order {
			if got != i {
				t.Fatalf("round %d notified in order %v, want ascending", round, order)
			}
		}
	}
}

func TestUnsubscribeKeepsTheRemainingOrder(t *testing.T) {
	store := redux.CreateStore(counterReducer)
	var order []string

	store.Subscribe(func() { order = append(order, "first") })
	dropSecond := store.Subscribe(func() { order = append(order, "second") })
	store.Subscribe(func() { order = append(order, "third") })
	dropSecond()

	store.Dispatch(redux.NewAction("INCREMENT", nil))

	if strings.Join(order, ",") != "first,third" {
		t.Fatalf("order = %v, want [first third]", order)
	}
}

// Redux's isDispatching is true only for the duration of the reducer call, so
// these two guards fire there and nowhere else. In particular they must not fire
// for a thunk, which runs in the middleware chain before the reducer is reached.
func TestGetStateIsRejectedInsideTheReducer(t *testing.T) {
	var store redux.Store[counterState]
	reducer := redux.Reducer[counterState](func(state counterState, action any) counterState {
		if action.(redux.Action).ActionType() == "PEEK" {
			store.GetState()
		}
		return state
	})
	store = redux.CreateStore(reducer)

	message := recoverMessage(t, func() { store.Dispatch(redux.NewAction("PEEK", nil)) })

	if !strings.HasPrefix(message, "You may not call store.getState() while the reducer is executing.") {
		t.Fatalf("message = %q", message)
	}
}

func TestUnsubscribeIsRejectedInsideTheReducer(t *testing.T) {
	var unsubscribe func()
	var store redux.Store[counterState]
	reducer := redux.Reducer[counterState](func(state counterState, action any) counterState {
		if action.(redux.Action).ActionType() == "DROP" {
			unsubscribe()
		}
		return state
	})
	store = redux.CreateStore(reducer)
	unsubscribe = store.Subscribe(func() {})

	message := recoverMessage(t, func() { store.Dispatch(redux.NewAction("DROP", nil)) })

	if !strings.HasPrefix(message, "You may not unsubscribe from a store listener while the reducer is executing.") {
		t.Fatalf("message = %q", message)
	}
}

func TestGetStateIsAllowedFromTheMiddlewareChain(t *testing.T) {
	peek := redux.Middleware[counterState](func(api *redux.MiddlewareAPI[counterState]) func(redux.Next) redux.Dispatch {
		return func(next redux.Next) redux.Dispatch {
			return func(action any) any {
				api.GetState()
				return next(action)
			}
		}
	})
	store := redux.CreateStore(counterReducer, redux.WithEnhancer(redux.ApplyMiddleware(peek)))

	store.Dispatch(redux.NewAction("INCREMENT", nil))

	if got := store.GetState().Value; got != 1 {
		t.Fatalf("state = %d, want 1", got)
	}
}

// An enhancer that is supplied but nil is a mistake Redux reports immediately.
// Dropping it silently produced a store without the thunk middleware, which then
// failed much later with a confusing "Actions must be plain objects".
func TestCreateStoreRejectsANilEnhancer(t *testing.T) {
	message := recoverMessage(t, func() {
		redux.CreateStore(counterReducer, redux.WithEnhancer[counterState](nil))
	})

	if message != "Expected the enhancer to be a function. Instead, received: 'null'" {
		t.Fatalf("message = %q", message)
	}
}

// The composed dispatch is published while middlewares may already be handing
// the forwarding closure to other goroutines. Under -race this fails if the
// publication is an unsynchronised variable write.
func TestForwardingDispatchIsSafeToPublish(t *testing.T) {
	var wg sync.WaitGroup
	spawn := redux.Middleware[counterState](func(api *redux.MiddlewareAPI[counterState]) func(redux.Next) redux.Dispatch {
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { _ = recover() }()
				api.Dispatch(redux.NewAction("INCREMENT", nil))
			}()
		}
		return func(next redux.Next) redux.Dispatch {
			return func(action any) any { return next(action) }
		}
	})

	store := redux.CreateStore(counterReducer, redux.WithEnhancer(redux.ApplyMiddleware(spawn)))
	wg.Wait()

	if fmt.Sprint(store.GetState().Value) == "" {
		t.Fatal("unreachable")
	}
}
