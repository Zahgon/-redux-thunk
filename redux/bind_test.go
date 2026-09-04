package redux_test

import (
	"testing"

	"github.com/reduxjs/redux-thunk-go/redux"
)

func TestBindActionCreatorDispatchesTheCreatedAction(t *testing.T) {
	var dispatched []any
	dispatch := redux.Dispatch(func(action any) any {
		dispatched = append(dispatched, action)
		return action
	})

	increment := func(by int) redux.AnyAction {
		return redux.NewAction("INCREMENT", map[string]any{"by": by})
	}

	bound, ok := redux.BindActionCreator(increment, dispatch).(func(int) any)
	if !ok {
		t.Fatal("expected the bound creator to keep the original parameter list")
	}

	returned := bound(3)

	if len(dispatched) != 1 {
		t.Fatalf("expected one dispatch, got %d", len(dispatched))
	}
	action, ok := dispatched[0].(redux.AnyAction)
	if !ok || action.ActionType() != "INCREMENT" || action["by"] != 3 {
		t.Errorf("unexpected dispatched action: %v", dispatched[0])
	}
	if returned == nil {
		t.Error("expected the bound creator to return the dispatch result")
	}
}

func TestBindActionCreatorForwardsVariadicArguments(t *testing.T) {
	var received []string
	dispatch := redux.Dispatch(func(action any) any {
		received = action.(redux.AnyAction)["names"].([]string)
		return action
	})

	addAll := func(names ...string) redux.AnyAction {
		return redux.NewAction("ADD_ALL", map[string]any{"names": names})
	}

	bound := redux.BindActionCreator(addAll, dispatch).(func(...string) any)
	bound("a", "b")

	if len(received) != 2 || received[0] != "a" || received[1] != "b" {
		t.Errorf("expected [a b], got %v", received)
	}
}

func TestBindActionCreatorRejectsNonFunctions(t *testing.T) {
	message := recoverMessage(t, func() {
		redux.BindActionCreator(42, func(any) any { return nil })
	})

	if message != "bindActionCreators expected a function actionCreator, instead received int" {
		t.Errorf("unexpected message: %q", message)
	}
}

func TestBindActionCreatorRejectsMultipleResults(t *testing.T) {
	message := recoverMessage(t, func() {
		redux.BindActionCreator(func() (int, error) { return 0, nil }, func(any) any { return nil })
	})

	if message != "bindActionCreators expected an action creator returning exactly one value, got 2" {
		t.Errorf("unexpected message: %q", message)
	}
}

func TestBindActionCreatorsBindsEveryKey(t *testing.T) {
	var types []string
	dispatch := redux.Dispatch(func(action any) any {
		types = append(types, action.(redux.Action).ActionType())
		return action
	})

	bound := redux.BindActionCreators(map[string]any{
		"increment": func() redux.AnyAction { return redux.NewAction("INCREMENT", nil) },
		"decrement": func() redux.AnyAction { return redux.NewAction("DECREMENT", nil) },
	}, dispatch)

	bound["increment"].(func() any)()
	bound["decrement"].(func() any)()

	if len(types) != 2 {
		t.Fatalf("expected two dispatches, got %v", types)
	}
	if types[0] != "INCREMENT" || types[1] != "DECREMENT" {
		t.Errorf("expected [INCREMENT DECREMENT], got %v", types)
	}
}

// Redux binds a key only `if (typeof actionCreator === 'function')`, so a
// non-function value is skipped, not rejected. The singular BindActionCreator
// still panics, matching Redux's internal helper which assumes a function.
func TestBindActionCreatorsSkipsNonFunctionValues(t *testing.T) {
	bound := redux.BindActionCreators(map[string]any{
		"increment":    func() redux.AnyAction { return redux.NewAction("INCREMENT", nil) },
		"notAFunction": 42,
		"alsoNot":      nil,
	}, func(action any) any { return action })

	if len(bound) != 1 {
		t.Fatalf("expected only the function entry to be bound, got %v", bound)
	}
	if _, ok := bound["increment"]; !ok {
		t.Error("expected the increment key to survive")
	}
}

func TestBindActionCreatorsRejectsNilArguments(t *testing.T) {
	if message := recoverMessage(t, func() {
		redux.BindActionCreators(nil, func(any) any { return nil })
	}); message != `bindActionCreators expected an object or a function, but instead received: 'null'. `+
		`Did you write "import ActionCreators from" instead of "import * as ActionCreators from"?` {
		t.Errorf("unexpected message for a nil map: %q", message)
	}

}

// Redux never inspects dispatch while binding, so `bindActionCreators(creators,
// undefined)` succeeds and the failure surfaces only when a bound creator runs.
func TestBindActionCreatorsDefersANilDispatchToCallTime(t *testing.T) {
	var bound map[string]any

	if message := recoverMessage(t, func() {
		bound = redux.BindActionCreators(map[string]any{
			"increment": func() redux.AnyAction { return redux.NewAction("INCREMENT", nil) },
		}, nil)
	}); message != "" {
		t.Fatalf("binding with a nil dispatch should not panic, got %q", message)
	}

	increment, ok := bound["increment"].(func() any)
	if !ok {
		t.Fatalf("expected a bound increment creator, got %T", bound["increment"])
	}

	if message := recoverMessage(t, func() { increment() }); message != "dispatch is not a function" {
		t.Errorf("unexpected message for a nil dispatch: %q", message)
	}
}
