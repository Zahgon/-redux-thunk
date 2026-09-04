package redux_test

import (
	"strings"
	"testing"

	"github.com/reduxjs/redux-thunk-go/redux"
)

// These mirror the two guards real redux@5 applies at the top of `dispatch`.
// The first is the one a thunk user hits when the middleware is missing, and its
// message is the only thing that tells them so.
func TestDispatchRejectsNonPlainActions(t *testing.T) {
	cases := []struct {
		name     string
		action   any
		wantKind string
	}{
		{"function", func() {}, "function"},
		{"nil", nil, "undefined"},
		{"string", "INCREMENT", "string"},
		{"number", 42, "number"},
		{"slice", []int{1}, "array"},
		{"boolean", true, "boolean"},
		{"nil pointer", (*counterState)(nil), "null"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := redux.CreateStore(counterReducer)
			message := recoverMessage(t, func() { store.Dispatch(tc.action) })

			want := "Actions must be plain objects. Instead, the actual type was: '" + tc.wantKind + "'."
			if !strings.HasPrefix(message, want) {
				t.Fatalf("message = %q, want it to start with %q", message, want)
			}
			if !strings.Contains(message, "'redux-thunk' to handle dispatching functions") {
				t.Errorf("message = %q, want it to name the middleware remedy", message)
			}
		})
	}
}

func TestDispatchRejectsActionsWithoutAType(t *testing.T) {
	const want = `Actions may not have an undefined "type" property. ` +
		"You may have misspelled an action type string constant."

	cases := map[string]any{
		"empty map":                 redux.AnyAction{},
		"map with no type":          map[string]any{"result": 1},
		"struct without ActionType": struct{ Result int }{Result: 1},
	}

	for name, action := range cases {
		t.Run(name, func(t *testing.T) {
			store := redux.CreateStore(counterReducer)
			if message := recoverMessage(t, func() { store.Dispatch(action) }); message != want {
				t.Fatalf("message = %q, want %q", message, want)
			}
		})
	}
}

// Redux's third guard. A `type` that is present but not a string gets its own
// message; reporting "undefined" for it would send the reader looking for a
// missing property that is in fact right there.
func TestDispatchRejectsANonStringActionType(t *testing.T) {
	store := redux.CreateStore(counterReducer)

	message := recoverMessage(t, func() {
		store.Dispatch(redux.AnyAction{redux.TypeKey: 7})
	})

	const want = `Action "type" property must be a string. ` +
		`Instead, the actual type was: 'number'. Value was: '7' (stringified)`
	if message != want {
		t.Fatalf("message = %q, want %q", message, want)
	}
}

// Redux only checks that `type` is defined and is a string, so the empty string
// is a legal action type. Rejecting it would be stricter than the original.
func TestDispatchAcceptsAnEmptyActionType(t *testing.T) {
	store := redux.CreateStore(counterReducer)

	if got := store.Dispatch(redux.AnyAction{redux.TypeKey: ""}); got == nil {
		t.Fatal("expected the action to be returned")
	}
}

func TestDispatchAcceptsPlainActions(t *testing.T) {
	store := redux.CreateStore(counterReducer)

	for _, action := range []any{
		redux.NewAction("INCREMENT", nil),
		map[string]any{redux.TypeKey: "INCREMENT"},
		incrementAction{},
		&incrementAction{},
	} {
		if got := store.Dispatch(action); got == nil {
			t.Fatalf("Dispatch(%#v) returned nil", action)
		}
	}

	// The unnamed map is a plain object as far as the guard is concerned, but it
	// carries no methods, so the reducer's type switch on redux.Action skips it.
	// Three of the four dispatches therefore reach the counter.
	if got := store.GetState().Value; got != 3 {
		t.Fatalf("state = %d, want 3", got)
	}
}

type incrementAction struct{}

func (incrementAction) ActionType() string { return "INCREMENT" }

// Actions that are not one of the three fast-path shapes fall through to the
// reflective branch, which must apply exactly the same three guards.
func TestDispatchValidatesUncommonActionShapes(t *testing.T) {
	store := redux.CreateStore(counterReducer)

	t.Run("typed map with a type", func(t *testing.T) {
		if got := store.Dispatch(map[string]string{redux.TypeKey: "INCREMENT"}); got == nil {
			t.Fatal("expected the action to be returned")
		}
	})

	t.Run("typed map without a type", func(t *testing.T) {
		message := recoverMessage(t, func() { store.Dispatch(map[string]int{"result": 1}) })
		if !strings.HasPrefix(message, `Actions may not have an undefined "type" property.`) {
			t.Fatalf("message = %q", message)
		}
	})

	t.Run("typed map with a non-string type", func(t *testing.T) {
		message := recoverMessage(t, func() { store.Dispatch(map[string]int{redux.TypeKey: 7}) })
		if !strings.HasPrefix(message, `Action "type" property must be a string.`) {
			t.Fatalf("message = %q", message)
		}
	})

	t.Run("map with a non-string key", func(t *testing.T) {
		message := recoverMessage(t, func() { store.Dispatch(map[int]any{1: "x"}) })
		if !strings.HasPrefix(message, "Actions must be plain objects.") {
			t.Fatalf("message = %q", message)
		}
	})

	t.Run("pointer to a struct implementing Action", func(t *testing.T) {
		if got := store.Dispatch(&incrementAction{}); got == nil {
			t.Fatal("expected the action to be returned")
		}
	})

	t.Run("channel", func(t *testing.T) {
		message := recoverMessage(t, func() { store.Dispatch(make(chan int)) })
		if !strings.Contains(message, "the actual type was: 'chan int'") {
			t.Fatalf("message = %q", message)
		}
	})
}
