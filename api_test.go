package thunk_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/redux"
)

// The upstream suites (test/index.test.ts and typescript_test/index.test-d.ts)
// only exercise the surface TypeScript exposes. The helpers this port had to add
// because Go lacks overloading, variadic generics and structural typing have no
// upstream counterpart, and neither do the failure modes of the reflective
// invocation path. This file covers both.

type apiState struct {
	Log []string
}

type apiRecorded struct {
	Payload string
}

func (apiRecorded) ActionType() string { return "RECORDED" }

func apiReducer(state apiState, action any) apiState {
	recorded, ok := action.(apiRecorded)
	if !ok {
		return state
	}
	return apiState{Log: append(append([]string{}, state.Log...), recorded.Payload)}
}

func newAPIStore() redux.Store[apiState] {
	return redux.CreateStore(apiReducer,
		redux.WithEnhancer(redux.ApplyMiddleware(thunk.TypedThunk[apiState]().AsMiddleware())))
}

func recoverTypeError(t *testing.T, fn func()) string {
	t.Helper()

	var message string
	func() {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			typeErr, ok := r.(*thunk.TypeError)
			if !ok {
				t.Fatalf("expected a *thunk.TypeError panic, got %T: %v", r, r)
			}
			message = typeErr.Error()
		}()
		fn()
	}()
	return message
}

func TestTypeErrorIsAnError(t *testing.T) {
	var err error = &thunk.TypeError{Message: "boom"}

	if err.Error() != "boom" {
		t.Fatalf("Error() = %q, want %q", err.Error(), "boom")
	}
}

func TestIsThunkRecognisesFunctionsOnly(t *testing.T) {
	var nilFunc func(redux.Dispatch) any

	cases := []struct {
		name  string
		value any
		want  bool
	}{
		{"nil", nil, false},
		{"plain action", apiRecorded{}, false},
		{"map action", redux.AnyAction{}, false},
		{"string", "INCREMENT", false},
		{"function", func() {}, true},
		{"thunk action", thunk.ThunkAction[any, apiState, any](nil), true},
		{"typed nil function", nilFunc, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := thunk.IsThunk(tc.value); got != tc.want {
				t.Fatalf("IsThunk(%#v) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestMiddlewareRejectsAnAbsentAPI(t *testing.T) {
	message := recoverTypeError(t, func() {
		thunk.Thunk(nil)
	})

	want := "Cannot destructure property 'dispatch' of 'undefined' as it is undefined."
	if message != want {
		t.Fatalf("panic message = %q, want %q", message, want)
	}
}

// JavaScript destructures `{}` happily: `thunk({})` builds a working middleware
// whose dispatch, getState and extra argument are all `undefined`, and a thunk
// dispatched through it observes exactly that. Node, run against a verbatim
// transcription of src/index.ts, returns "undefined,undefined,undefined".
func TestMiddlewareAcceptsAnEmptyAPI(t *testing.T) {
	handler := thunk.Thunk(&redux.MiddlewareAPI[any]{})(nil)

	var got []any
	handler(func(dispatch redux.Dispatch, getState func() any, extra any) any {
		got = []any{dispatch, getState, extra}
		return nil
	})

	if len(got) != 3 {
		t.Fatalf("thunk was not invoked, got %v", got)
	}
	for i, v := range got {
		if v != nil && !reflect.ValueOf(v).IsNil() {
			t.Errorf("argument %d = %v, want the Go equivalent of undefined", i, v)
		}
	}
}

func TestMiddlewareWithAnEmptyAPIStillForwardsPlainActions(t *testing.T) {
	forwarded := false
	handler := thunk.Thunk(&redux.MiddlewareAPI[any]{})(func(action any) any {
		forwarded = true
		return action
	})

	if got := handler(apiRecorded{Payload: "x"}); got != (apiRecorded{Payload: "x"}) {
		t.Fatalf("handler returned %#v", got)
	}
	if !forwarded {
		t.Fatal("expected the action to reach next")
	}
}

func TestBind1PassesItsArgumentThrough(t *testing.T) {
	store := newAPIStore()
	creator := func(name string) thunk.ThunkAction[string, apiState, any] {
		return func(dispatch redux.Dispatch, _ func() apiState, _ any) string {
			dispatch(apiRecorded{Payload: name})
			return "hello " + name
		}
	}

	bound := thunk.Bind1(store.Dispatch, creator)

	if got := bound("world"); got != "hello world" {
		t.Fatalf("bound(\"world\") = %q, want %q", got, "hello world")
	}
	if got := store.GetState().Log; len(got) != 1 || got[0] != "world" {
		t.Fatalf("state log = %v, want [world]", got)
	}
}

func TestBind2PassesBothArguments(t *testing.T) {
	store := newAPIStore()
	creator := func(first, second string) thunk.ThunkAction[string, apiState, any] {
		return func(_ redux.Dispatch, _ func() apiState, _ any) string {
			return first + "+" + second
		}
	}

	bound := thunk.Bind2(store.Dispatch, creator)

	if got := bound("a", "b"); got != "a+b" {
		t.Fatalf("bound(\"a\", \"b\") = %q, want %q", got, "a+b")
	}
}

func TestBind3PassesAllThreeArguments(t *testing.T) {
	store := newAPIStore()
	creator := func(first, second string, third int) thunk.ThunkAction[string, apiState, any] {
		return func(_ redux.Dispatch, _ func() apiState, _ any) string {
			return first + second + string(rune('0'+third))
		}
	}

	bound := thunk.Bind3(store.Dispatch, creator)

	if got := bound("a", "b", 3); got != "ab3" {
		t.Fatalf("bound(\"a\", \"b\", 3) = %q, want %q", got, "ab3")
	}
}

func TestBindPlain1DispatchesAndReturnsTheAction(t *testing.T) {
	store := newAPIStore()
	creator := func(payload string) apiRecorded { return apiRecorded{Payload: payload} }

	bound := thunk.BindPlain1(store.Dispatch, creator)

	if got := bound("logged"); got.Payload != "logged" {
		t.Fatalf("bound(\"logged\") = %#v, want payload %q", got, "logged")
	}
	if got := store.GetState().Log; len(got) != 1 || got[0] != "logged" {
		t.Fatalf("state log = %v, want [logged]", got)
	}
}

func TestEveryCanonicalThunkShapeIsInvokedWithoutReflection(t *testing.T) {
	// The middleware recognises nine thunk signatures directly so that the
	// dispatch and getState values reach the thunk by identity, which is what
	// `expect(dispatch).toBe(doDispatch)` asserts upstream. Reflection would
	// still call each of these correctly, but it would wrap the arguments.
	var apiDispatch redux.Dispatch = func(action any) any { return action }
	apiGetState := func() apiState { return apiState{} }
	handler := thunk.TypedThunk[apiState]()(&redux.MiddlewareAPI[apiState]{
		Dispatch: apiDispatch,
		GetState: apiGetState,
	})(nil)

	sameArgs := func(d redux.Dispatch, g func() apiState) any {
		return sameRef(d, apiDispatch) && sameRef(g, apiGetState)
	}

	cases := []struct {
		name   string
		action any
		want   any
	}{
		{"ThunkAction", thunk.ThunkAction[any, apiState, any](
			func(d redux.Dispatch, g func() apiState, _ any) any { return sameArgs(d, g) }), true},
		{"dispatch, getState, extra -> any", func(d redux.Dispatch, g func() apiState, _ any) any {
			return sameArgs(d, g)
		}, true},
		{"dispatch, getState -> any", func(d redux.Dispatch, g func() apiState) any {
			return sameArgs(d, g)
		}, true},
		{"dispatch -> any", func(d redux.Dispatch) any { return sameRef(d, apiDispatch) }, true},
		{"no arguments -> any", func() any { return true }, true},
		{"dispatch, getState, extra", func(_ redux.Dispatch, _ func() apiState, _ any) {}, nil},
		{"dispatch, getState", func(_ redux.Dispatch, _ func() apiState) {}, nil},
		{"dispatch", func(_ redux.Dispatch) {}, nil},
		{"no arguments", func() {}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := handler(tc.action); got != tc.want {
				t.Fatalf("dispatch(%s) = %#v, want %#v", tc.name, got, tc.want)
			}
		})
	}
}

func TestDispatchThunkReturnsTheZeroValueWhenTheThunkReturnsNothing(t *testing.T) {
	store := newAPIStore()
	action := thunk.ThunkAction[any, apiState, any](func(_ redux.Dispatch, _ func() apiState, _ any) any {
		return nil
	})

	if got := thunk.DispatchThunk(store.Dispatch, action); got != nil {
		t.Fatalf("DispatchThunk() = %#v, want nil", got)
	}
}

// Without the middleware the store itself rejects a function action, using
// Redux's own message -- which is the diagnostic that names redux-thunk. This
// is verified against real redux@5 behaviour, not merely against this port.
func TestDispatchingAThunkWithoutTheMiddlewareReportsRedux(t *testing.T) {
	store := redux.CreateStore(apiReducer)
	action := thunk.ThunkAction[string, apiState, any](func(_ redux.Dispatch, _ func() apiState, _ any) string {
		return "never runs"
	})

	var message string
	func() {
		defer func() {
			r := recover()
			err, ok := r.(*redux.Error)
			if !ok {
				t.Fatalf("expected a *redux.Error, got %T: %v", r, r)
			}
			message = err.Message
		}()
		thunk.DispatchThunk(store.Dispatch, action)
	}()

	for _, want := range []string{
		"Actions must be plain objects. Instead, the actual type was: 'function'.",
		"such as 'redux-thunk' to handle dispatching functions",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("panic message = %q, want it to contain %q", message, want)
		}
	}
}

func TestDispatchActionRejectsARewrittenResult(t *testing.T) {
	rewrite := redux.Middleware[apiState](func(_ *redux.MiddlewareAPI[apiState]) func(redux.Next) redux.Dispatch {
		return func(next redux.Next) redux.Dispatch {
			return func(action any) any {
				next(action)
				return "rewritten"
			}
		}
	})
	store := redux.CreateStore(apiReducer, redux.WithEnhancer(redux.ApplyMiddleware(rewrite)))

	message := recoverTypeError(t, func() {
		thunk.DispatchAction(store.Dispatch, apiRecorded{Payload: "kept"})
	})

	if !strings.Contains(message, "a middleware replaced the action result") {
		t.Fatalf("panic message = %q, want it to report the rewrite", message)
	}
}

// JavaScript spreads every supplied argument into a rest parameter, so
// `(...args) => args.length` sees three and `(d, ...rest) => rest.length` sees
// two. The reflective path reproduces that rather than leaving the slot empty.
func TestVariadicThunkReceivesTheRemainingArguments(t *testing.T) {
	handler := thunk.WithExtraArgument[apiState]("extra")(&redux.MiddlewareAPI[apiState]{
		Dispatch: func(action any) any { return action },
		GetState: func() apiState { return apiState{} },
	})(nil)

	t.Run("all three land in the rest slot", func(t *testing.T) {
		var seen []any
		handler(func(rest ...any) any {
			seen = rest
			return len(rest)
		})
		if len(seen) != 3 {
			t.Fatalf("rest = %v, want three arguments", seen)
		}
		if seen[2] != "extra" {
			t.Errorf("rest[2] = %#v, want the extra argument", seen[2])
		}
	})

	t.Run("fixed parameters consume their share first", func(t *testing.T) {
		var seenRest []any
		got := handler(func(dispatch redux.Dispatch, rest ...any) any {
			if dispatch == nil {
				t.Error("dispatch was not supplied to the fixed parameter")
			}
			seenRest = rest
			return len(rest)
		})
		if got != 2 {
			t.Fatalf("thunk returned %#v, want 2", got)
		}
		if seenRest[1] != "extra" {
			t.Errorf("rest[1] = %#v, want the extra argument", seenRest[1])
		}
	})
}

// JavaScript passes `undefined` for parameters it has no argument for, so a
// four-parameter thunk runs with its fourth left undefined. The zero value is
// this port's `undefined`, and reflect can fabricate one for any type.
func TestThunkWithMoreParametersThanArgumentsIsRun(t *testing.T) {
	store := newAPIStore()

	got := store.Dispatch(func(dispatch redux.Dispatch, getState func() apiState, extra any, fourth string) any {
		if dispatch == nil || getState == nil {
			t.Error("the supplied arguments did not reach the thunk")
		}
		if extra != nil {
			t.Errorf("extra = %#v, want nil", extra)
		}
		return fourth
	})

	if got != "" {
		t.Fatalf("fourth parameter = %#v, want the zero value", got)
	}
}

func TestThunkWithMoreThanOneResultIsRejected(t *testing.T) {
	store := newAPIStore()

	message := recoverTypeError(t, func() {
		store.Dispatch(func(_ redux.Dispatch, _ func() apiState, _ any) (string, error) {
			return "", nil
		})
	})

	if !strings.Contains(message, "returns 2 values") {
		t.Fatalf("panic message = %q, want it to report the result count", message)
	}
}

// A getState that yields nil is JavaScript's `undefined`, which a thunk can
// legitimately observe. It must arrive as the zero value rather than blowing up
// inside the adapter that reshapes getState for the thunk's declared signature.
func TestNilStateReachesAConcretelyTypedThunk(t *testing.T) {
	handler := thunk.Thunk(&redux.MiddlewareAPI[any]{
		Dispatch: func(action any) any { return action },
		GetState: func() any { return nil },
	})(nil)

	got := handler(func(_ redux.Dispatch, getState func() apiState, _ any) any {
		return getState()
	})

	state, ok := got.(apiState)
	if !ok {
		t.Fatalf("getState() = %#v, want an apiState", got)
	}
	if state.Log != nil {
		t.Fatalf("getState() = %#v, want the zero state", state)
	}
}

// Identity survives only on the fast paths. A thunk declaring a getState type
// other than the one the middleware holds forces an adapter: `func() apiState`
// and `func() any` are distinct Go types and no single value inhabits both.
// Asserted here so the divergence is a tested fact rather than a claim in a
// document.
func TestGetStateIdentityHoldsOnlyWhenTheSignaturesMatch(t *testing.T) {
	getState := func() apiState { return apiState{} }
	handler := thunk.TypedThunk[apiState]()(&redux.MiddlewareAPI[apiState]{
		Dispatch: func(action any) any { return action },
		GetState: getState,
	})(nil)

	var matching, adapted any
	handler(func(_ redux.Dispatch, gs func() apiState, _ any) any { matching = gs; return nil })
	handler(func(_ redux.Dispatch, gs func() any, _ any) any { adapted = gs; return nil })

	if !sameRef(matching, getState) {
		t.Error("a thunk declaring the middleware's own state type must receive getState by identity")
	}
	if sameRef(adapted, getState) {
		t.Error("a thunk declaring a different state type cannot receive getState by identity")
	}
	if adapted == nil {
		t.Error("the adapted thunk was not invoked")
	}
}

// A nil function value is `typeof === 'function'` in spirit but cannot be
// called. Every canonical shape is listed so that the fast path cannot disagree
// with the reflective one about the error class for the same input.
func TestNilThunkIsRejectedIdenticallyOnEveryPath(t *testing.T) {
	var (
		typed       thunk.ThunkAction[any, apiState, any]
		fullResult  func(redux.Dispatch, func() apiState, any) any
		twoResult   func(redux.Dispatch, func() apiState) any
		oneResult   func(redux.Dispatch) any
		zeroResult  func() any
		full        func(redux.Dispatch, func() apiState, any)
		two         func(redux.Dispatch, func() apiState)
		one         func(redux.Dispatch)
		zero        func()
		reflective  func(redux.Dispatch, func() apiState, any) string
		reflectiveV func(...any) string
	)

	cases := map[string]any{
		"ThunkAction":                    typed,
		"dispatch/getState/extra -> any": fullResult,
		"dispatch/getState -> any":       twoResult,
		"dispatch -> any":                oneResult,
		"no parameters -> any":           zeroResult,
		"dispatch/getState/extra":        full,
		"dispatch/getState":              two,
		"dispatch":                       one,
		"no parameters":                  zero,
		"reflective":                     reflective,
		"reflective variadic":            reflectiveV,
	}

	for name, action := range cases {
		t.Run(name, func(t *testing.T) {
			store := newAPIStore()
			if message := recoverTypeError(t, func() { store.Dispatch(action) }); message != "thunk action is a nil function value" {
				t.Fatalf("panic message = %q", message)
			}
		})
	}
}

func TestNilThunkFunctionIsRejected(t *testing.T) {
	store := newAPIStore()
	var action func(redux.Dispatch, func() apiState, any) string

	message := recoverTypeError(t, func() {
		store.Dispatch(action)
	})

	if message != "thunk action is a nil function value" {
		t.Fatalf("panic message = %q, want the nil-function message", message)
	}
}

func TestThunkParameterOfAnUnrelatedTypeIsRejected(t *testing.T) {
	store := newAPIStore()

	message := recoverTypeError(t, func() {
		store.Dispatch(func(_ int) any { return nil })
	})

	if !strings.Contains(message, "cannot pass a value of type") {
		t.Fatalf("panic message = %q, want the parameter-type message", message)
	}
}

func TestGetStateIsAdaptedToTheThunkDeclaredSignature(t *testing.T) {
	store := newAPIStore()
	store.Dispatch(apiRecorded{Payload: "first"})

	// The middleware holds `func() apiState`. A thunk may declare a wider
	// signature; the shim forwards the call and zero-fills the results the
	// source function does not produce.
	got := store.Dispatch(func(_ redux.Dispatch, getState func() (apiState, error)) any {
		state, err := getState()
		if err != nil {
			return err
		}
		return state.Log
	})

	log, ok := got.([]string)
	if !ok {
		t.Fatalf("dispatch returned %T, want []string", got)
	}
	if len(log) != 1 || log[0] != "first" {
		t.Fatalf("state log = %v, want [first]", log)
	}
}

func TestConcreteGetStateIsAdaptedFromAnUntypedStore(t *testing.T) {
	store := redux.CreateStore(
		func(state any, _ any) any { return state },
		redux.WithPreloadedState[any](apiState{Log: []string{"seeded"}}),
		redux.WithEnhancer(redux.ApplyMiddleware(thunk.Thunk.AsMiddleware())),
	)

	// The untyped middleware holds `func() any`; the thunk asks for
	// `func() apiState`, so the result must be unwrapped out of the interface.
	got := store.Dispatch(func(_ redux.Dispatch, getState func() apiState) any {
		return getState().Log
	})

	log, ok := got.([]string)
	if !ok {
		t.Fatalf("dispatch returned %T, want []string", got)
	}
	if len(log) != 1 || log[0] != "seeded" {
		t.Fatalf("state log = %v, want [seeded]", log)
	}
}

func TestDispatchEitherHandlesBothArms(t *testing.T) {
	store := newAPIStore()
	action := thunk.ThunkAction[any, apiState, any](func(dispatch redux.Dispatch, _ func() apiState, _ any) any {
		return dispatch(apiRecorded{Payload: "from thunk"})
	})

	for _, value := range []any{apiRecorded{Payload: "plain"}, action} {
		if out := thunk.DispatchEither(store.Dispatch, value); out == nil {
			t.Fatalf("DispatchEither(%T) returned nil", value)
		}
	}

	want := []string{"plain", "from thunk"}
	got := store.GetState().Log
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("state log = %v, want %v", got, want)
	}
}

func TestThunkErrorsPropagateToTheCaller(t *testing.T) {
	store := newAPIStore()
	failure := errors.New("thunk failed")
	action := thunk.ThunkAction[error, apiState, any](func(_ redux.Dispatch, _ func() apiState, _ any) error {
		return failure
	})

	if got := thunk.DispatchThunk(store.Dispatch, action); !errors.Is(got, failure) {
		t.Fatalf("DispatchThunk() = %v, want %v", got, failure)
	}
}
