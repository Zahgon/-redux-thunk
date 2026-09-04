package redux_test

import (
	"strings"
	"testing"

	"github.com/reduxjs/redux-thunk-go/redux"
)

type counterState struct {
	Value int
}

func counterReducer(state counterState, action any) counterState {
	if a, ok := action.(redux.Action); ok && a.ActionType() == "INCREMENT" {
		return counterState{Value: state.Value + 1}
	}
	return state
}

// recoverMessage runs fn and reports the message of the *redux.Error it
// panicked with. It reports "" when fn returns normally.
func recoverMessage(t *testing.T, fn func()) (message string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		err, ok := r.(*redux.Error)
		if !ok {
			t.Fatalf("expected a *redux.Error panic, got %T: %v", r, r)
		}
		message = err.Error()
	}()
	fn()
	return ""
}

func TestCreateStoreRunsTheReducerWithAnInitAction(t *testing.T) {
	var seen []string
	store := redux.CreateStore(func(state counterState, action any) counterState {
		if a, ok := action.(redux.Action); ok {
			seen = append(seen, a.ActionType())
		}
		return state
	}, redux.WithPreloadedState(counterState{Value: 5}))

	if len(seen) != 1 {
		t.Fatalf("expected the reducer to run exactly once during construction, ran %d times", len(seen))
	}
	if !strings.HasPrefix(seen[0], "@@redux/INIT") {
		t.Errorf("expected an @@redux/INIT action, got %q", seen[0])
	}
	if store.GetState().Value != 5 {
		t.Errorf("expected the preloaded state to survive init, got %d", store.GetState().Value)
	}
}

func TestCreateStoreRejectsANilReducer(t *testing.T) {
	message := recoverMessage(t, func() {
		redux.CreateStore[counterState](nil)
	})

	if message != "Expected the root reducer to be a function. Instead, received: 'undefined'" {
		t.Errorf("unexpected message: %q", message)
	}
}

func TestDispatchUpdatesStateAndReturnsTheAction(t *testing.T) {
	store := redux.CreateStore(counterReducer)
	action := redux.NewAction("INCREMENT", nil)

	returned := store.Dispatch(action)

	if store.GetState().Value != 1 {
		t.Errorf("expected the reducer to have run, state is %d", store.GetState().Value)
	}
	if returned, ok := returned.(redux.AnyAction); !ok || returned.ActionType() != "INCREMENT" {
		t.Errorf("expected dispatch to return the action it was given, got %v", returned)
	}
}

func TestDispatchRejectsReentrancyFromAReducer(t *testing.T) {
	var store redux.Store[counterState]
	store = redux.CreateStore(func(state counterState, action any) counterState {
		if a, ok := action.(redux.Action); ok && a.ActionType() == "REENTER" {
			store.Dispatch(redux.NewAction("INCREMENT", nil))
		}
		return state
	})

	message := recoverMessage(t, func() {
		store.Dispatch(redux.NewAction("REENTER", nil))
	})

	if message != "Reducers may not dispatch actions." {
		t.Errorf("unexpected message: %q", message)
	}
}

func TestDispatchRecoversTheGuardAfterAPanickingReducer(t *testing.T) {
	store := redux.CreateStore(func(state counterState, action any) counterState {
		if a, ok := action.(redux.Action); ok && a.ActionType() == "BOOM" {
			panic(&redux.Error{Message: "reducer exploded"})
		}
		return counterReducer(state, action)
	})

	if message := recoverMessage(t, func() {
		store.Dispatch(redux.NewAction("BOOM", nil))
	}); message != "reducer exploded" {
		t.Fatalf("expected the reducer's panic to propagate, got %q", message)
	}

	store.Dispatch(redux.NewAction("INCREMENT", nil))

	if store.GetState().Value != 1 {
		t.Error("expected the store to still be usable after a reducer panic")
	}
}

func TestSubscribeNotifiesListenersAndUnsubscribes(t *testing.T) {
	store := redux.CreateStore(counterReducer)
	calls := 0

	unsubscribe := store.Subscribe(func() { calls++ })
	store.Dispatch(redux.NewAction("INCREMENT", nil))
	unsubscribe()
	store.Dispatch(redux.NewAction("INCREMENT", nil))

	if calls != 1 {
		t.Errorf("expected exactly one notification, got %d", calls)
	}
}

func TestUnsubscribeIsIdempotent(t *testing.T) {
	store := redux.CreateStore(counterReducer)
	unsubscribe := store.Subscribe(func() {})

	unsubscribe()
	unsubscribe()

	store.Dispatch(redux.NewAction("INCREMENT", nil))
}

func TestSubscribeRejectsANilListener(t *testing.T) {
	store := redux.CreateStore(counterReducer)

	message := recoverMessage(t, func() { store.Subscribe(nil) })

	if message != "Expected the listener to be a function. Instead, received: 'undefined'" {
		t.Errorf("unexpected message: %q", message)
	}
}

func TestSubscribeRejectsCallsFromInsideTheReducer(t *testing.T) {
	var store redux.Store[counterState]
	var caught string
	store = redux.CreateStore(func(state counterState, action any) counterState {
		if a, ok := action.(redux.Action); ok && a.ActionType() == "SUBSCRIBE" {
			caught = recoverMessage(t, func() { store.Subscribe(func() {}) })
		}
		return state
	})

	store.Dispatch(redux.NewAction("SUBSCRIBE", nil))

	if caught != "You may not call store.subscribe() while the reducer is executing. "+
		"If you would like to be notified after the store has been updated, subscribe from a "+
		"component and invoke store.getState() in the callback to access the latest state. "+
		"See https://redux.js.org/api/store#subscribelistener for more details." {
		t.Errorf("unexpected message: %q", caught)
	}
}

func TestReplaceReducerSwapsTheReducer(t *testing.T) {
	store := redux.CreateStore(counterReducer)
	store.Dispatch(redux.NewAction("INCREMENT", nil))

	store.ReplaceReducer(func(state counterState, action any) counterState {
		if a, ok := action.(redux.Action); ok && a.ActionType() == "INCREMENT" {
			return counterState{Value: state.Value + 10}
		}
		return state
	})
	store.Dispatch(redux.NewAction("INCREMENT", nil))

	if store.GetState().Value != 11 {
		t.Errorf("expected 11, got %d", store.GetState().Value)
	}
}

func TestReplaceReducerRejectsANilReducer(t *testing.T) {
	store := redux.CreateStore(counterReducer)

	message := recoverMessage(t, func() { store.ReplaceReducer(nil) })

	if message != "Expected the nextReducer to be a function. Instead, received: 'undefined" {
		t.Errorf("unexpected message: %q", message)
	}
}

func TestAnyActionReportsItsType(t *testing.T) {
	action := redux.NewAction("FOO", map[string]any{"result": 42})

	if action.ActionType() != "FOO" {
		t.Errorf("expected FOO, got %q", action.ActionType())
	}
	if action["result"] != 42 {
		t.Errorf("expected the extra properties to be carried, got %v", action["result"])
	}
	if (redux.AnyAction{}).ActionType() != "" {
		t.Error("expected a typeless action to report the empty string")
	}
}
