package redux_test

import (
	"testing"

	"github.com/reduxjs/redux-thunk-go/redux"
)

func TestComposeAppliesRightToLeft(t *testing.T) {
	var order []string
	tag := func(name string) func(redux.Dispatch) redux.Dispatch {
		return func(next redux.Dispatch) redux.Dispatch {
			order = append(order, name)
			return next
		}
	}

	redux.Compose(tag("f"), tag("g"), tag("h"))(func(any) any { return nil })

	for i, want := range []string{"h", "g", "f"} {
		if order[i] != want {
			t.Errorf("index %d: expected %q, got %q", i, want, order[i])
		}
	}
}

func TestComposeWithNoFunctionsIsIdentity(t *testing.T) {
	sentinel := func(action any) any { return action }

	composed := redux.Compose()(sentinel)

	if composed("x") != "x" {
		t.Error("expected the empty composition to pass the dispatch through unchanged")
	}
}

// recordingMiddleware appends its name to log before and after delegating, so
// that the test can assert the left-to-right chain order applyMiddleware
// promises.
func recordingMiddleware(name string, log *[]string) redux.Middleware[counterState] {
	return func(*redux.MiddlewareAPI[counterState]) func(redux.Next) redux.Dispatch {
		return func(next redux.Next) redux.Dispatch {
			return func(action any) any {
				*log = append(*log, name+":before")
				result := next(action)
				*log = append(*log, name+":after")
				return result
			}
		}
	}
}

func TestApplyMiddlewareRunsTheChainLeftToRight(t *testing.T) {
	var log []string
	store := redux.CreateStore(counterReducer, redux.WithEnhancer(
		redux.ApplyMiddleware(
			recordingMiddleware("a", &log),
			recordingMiddleware("b", &log),
		)))

	store.Dispatch(redux.NewAction("INCREMENT", nil))

	want := []string{"a:before", "b:before", "b:after", "a:after"}
	if len(log) != len(want) {
		t.Fatalf("expected %v, got %v", want, log)
	}
	for i := range want {
		if log[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, log)
		}
	}
}

func TestApplyMiddlewareGivesEachMiddlewareTheFullChain(t *testing.T) {
	var log []string
	// redispatch forwards a second action through api.Dispatch. If the API
	// carried the raw store dispatch rather than the forwarding closure, the
	// redispatched action would skip the "tail" middleware entirely.
	redispatch := func(api *redux.MiddlewareAPI[counterState]) func(redux.Next) redux.Dispatch {
		return func(next redux.Next) redux.Dispatch {
			return func(action any) any {
				if a, ok := action.(redux.Action); ok && a.ActionType() == "REDISPATCH" {
					return api.Dispatch(redux.NewAction("INCREMENT", nil))
				}
				return next(action)
			}
		}
	}

	store := redux.CreateStore(counterReducer, redux.WithEnhancer(
		redux.ApplyMiddleware(
			redux.Middleware[counterState](redispatch),
			recordingMiddleware("tail", &log),
		)))

	store.Dispatch(redux.NewAction("REDISPATCH", nil))

	if len(log) != 2 {
		t.Fatalf("expected the redispatched action to reach the tail middleware, log is %v", log)
	}
	if store.GetState().Value != 1 {
		t.Errorf("expected the redispatched action to reach the reducer, state is %d", store.GetState().Value)
	}
}

func TestApplyMiddlewareRejectsDispatchDuringConstruction(t *testing.T) {
	eager := func(api *redux.MiddlewareAPI[counterState]) func(redux.Next) redux.Dispatch {
		api.Dispatch(redux.NewAction("INCREMENT", nil))
		return func(next redux.Next) redux.Dispatch {
			return redux.Dispatch(next)
		}
	}

	message := recoverMessage(t, func() {
		redux.CreateStore(counterReducer, redux.WithEnhancer(
			redux.ApplyMiddleware(redux.Middleware[counterState](eager))))
	})

	want := "Dispatching while constructing your middleware is not allowed. " +
		"Other middleware would not be applied to this dispatch."
	if message != want {
		t.Errorf("unexpected message: %q", message)
	}
}

// JavaScript calls every entry of the middlewares array, so a nil entry throws
// there too. Skipping it would hand back a store that silently lacks the
// middleware the caller listed -- for a thunk user, a store where thunks are no
// longer functions but plain rejected actions.
func TestApplyMiddlewareRejectsNilMiddlewares(t *testing.T) {
	var log []string

	message := recoverMessage(t, func() {
		redux.CreateStore(counterReducer, redux.WithEnhancer(
			redux.ApplyMiddleware(recordingMiddleware("first", &log), nil)))
	})

	if message != "middleware is not a function" {
		t.Errorf("unexpected message: %q", message)
	}
}

func TestApplyMiddlewarePreservesTheOtherStoreMethods(t *testing.T) {
	var log []string
	store := redux.CreateStore(counterReducer,
		redux.WithPreloadedState(counterState{Value: 3}),
		redux.WithEnhancer(redux.ApplyMiddleware(recordingMiddleware("a", &log))))

	notified := 0
	store.Subscribe(func() { notified++ })
	store.Dispatch(redux.NewAction("INCREMENT", nil))

	if store.GetState().Value != 4 {
		t.Errorf("expected 4, got %d", store.GetState().Value)
	}
	if notified != 1 {
		t.Errorf("expected the enhanced store to keep delegating Subscribe, got %d notifications", notified)
	}
}
