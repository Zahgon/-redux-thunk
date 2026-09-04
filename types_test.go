package thunk_test

import (
	"testing"

	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/async"
	"github.com/reduxjs/redux-thunk-go/redux"
)

// This file is the port of typescript_test/index.test-d.ts from redux-thunk
// v3.1.0. The original is a pure type test: it compiles fixtures and asserts on
// their inferred types with `expectTypeOf`, never running anything.
//
// Go has no equivalent of `expectTypeOf`, because Go's type system offers no way
// to observe an inferred type from within the language. The assertions are
// therefore split in two:
//
//   - Positive assertions (`toBeCallableWith`, `toHaveProperty`, `resolves`)
//     become ordinary code. If the port's types were wrong this file would not
//     compile, which is exactly the guarantee the original provides. The
//     fixtures additionally run, so the port is checked more strictly than the
//     original: the types must be right AND the values must be correct.
//
//   - Negative assertions (`.not.toMatchTypeOf`, `@ts-expect-error`) cannot be
//     written as compiling code by definition. They live in testdata/typeerrors
//     and are checked by TestTypeErrors in typeerrors_test.go, which compiles
//     each fixture and requires it to fail with the expected message.

// State is the port of `type State = { foo: string }`.
type State struct {
	Foo string
}

// Actions is the port of `type Actions = { type: 'FOO' } | { type: 'BAR'; result: number }`.
//
// Go has no union types. The idiomatic encoding of a closed union is an
// interface with an unexported marker method, which no type outside this
// package can implement. That gives the same exhaustiveness guarantee the
// TypeScript union provides: the compiler rejects any action that is not a
// member, which is what the original's `.not.toMatchTypeOf({ type: 'BAZ' })`
// assertions are checking.
type Actions interface {
	redux.Action
	isActions()
}

// FooAction is `{ type: 'FOO' }`.
type FooAction struct{}

func (FooAction) ActionType() string { return "FOO" }
func (FooAction) isActions()         {}

// BarAction is `{ type: 'BAR'; result: number }`.
type BarAction struct{ Result int }

func (BarAction) ActionType() string { return "BAR" }
func (BarAction) isActions()         {}

// BazAction is `{ type: 'BAZ' }`, which the original notes is deliberately not
// a member of Actions. It implements redux.Action but not the Actions marker,
// so it can be dispatched through the erased runtime dispatch (as the original
// does with `store.dispatch({ type: 'BAZ' })`) yet cannot be passed anywhere
// that statically demands an Actions.
type BazAction struct{}

func (BazAction) ActionType() string { return "BAZ" }

// ThunkResult is `type ThunkResult<R> = ThunkAction<R, State, undefined, Actions>`.
//
// Go 1.22 has no generic type aliases, so the instantiations the original
// actually uses are spelled out individually. The `undefined` extra argument
// becomes `any`, whose zero value nil is Go's closest analogue, and the
// BasicAction parameter is dropped (see ThunkAction's documentation).
type (
	ThunkResultString      = thunk.ThunkAction[string, State, any]
	ThunkResultVoid        = thunk.ThunkAction[any, State, any]
	ThunkResultFutureBool  = thunk.ThunkAction[*async.Future[bool], State, any]
	ThunkResultFutureVoid  = thunk.ThunkAction[*async.Future[any], State, any]
	ThunkResultWithExtraTS = thunk.ThunkAction[any, State, string]
)

var initialState = State{Foo: "foo"}

func fakeReducer(state State, _ any) State { return state }

// newStore is the port of:
//
//	const store = createStore(fakeReducer, applyMiddleware(thunk as ThunkMiddleware<State, Actions>))
//
// The TypeScript cast becomes TypedThunk[State](), which constructs the same
// middleware specialised to the state type. A constructor function is used
// rather than a package-level variable so that each subtest gets an independent
// store, matching the original's per-file module scope without sharing mutable
// state between Go subtests that may run in parallel.
func newStore() redux.Store[State] {
	return redux.CreateStore[State](
		fakeReducer,
		redux.WithPreloadedState(initialState),
		redux.WithEnhancer(redux.ApplyMiddleware(
			thunk.TypedThunk[State]().AsMiddleware(),
		)),
	)
}

// anotherThunkAction is the port of:
//
//	function anotherThunkAction(): ThunkResult<string> {
//	  return (dispatch, getState) => {
//	    expectTypeOf(dispatch).toBeCallableWith({ type: 'FOO' })
//	    return 'hello'
//	  }
//	}
func anotherThunkAction() ThunkResultString {
	return func(dispatch redux.Dispatch, _ func() State, _ any) string {
		dispatch(FooAction{})

		return "hello"
	}
}

// promiseThunkAction is the port of:
//
//	function promiseThunkAction(): ThunkResult<Promise<boolean>> {
//	  return async (dispatch, getState) => {
//	    expectTypeOf(dispatch).toBeCallableWith({ type: 'FOO' })
//	    return false
//	  }
//	}
func promiseThunkAction() ThunkResultFutureBool {
	return func(dispatch redux.Dispatch, _ func() State, _ any) *async.Future[bool] {
		return async.New(func() (bool, error) {
			dispatch(FooAction{})

			return false, nil
		})
	}
}

// standardAction is the port of `const standardAction = () => ({ type: 'FOO' })`.
func standardAction() FooAction { return FooAction{} }

func TestTypeTests(t *testing.T) {
	t.Run("store.dispatch", func(t *testing.T) {
		store := newStore()

		ran := false
		thunk.DispatchThunk(store.Dispatch, ThunkResultVoid(
			func(dispatch redux.Dispatch, _ func() State, _ any) any {
				ran = true

				// expectTypeOf(dispatch).toBeCallableWith({ type: 'FOO' })
				thunk.DispatchAction[Actions](dispatch, FooAction{})

				// expectTypeOf(dispatch).toBeCallableWith({ type: 'BAR', result: 5 })
				thunk.DispatchAction[Actions](dispatch, BarAction{Result: 5})

				// The original notes that
				// `expectTypeOf(store.dispatch).toBeCallableWith({ type: 'BAZ' })`
				// does not work in this case, and falls back to an untyped
				// dispatch. The erased runtime dispatch is the same escape
				// hatch here.
				store.Dispatch(BazAction{})

				return nil
			}))

		if !ran {
			t.Error("expected the thunk to have been run")
		}
	})

	t.Run("getState", func(t *testing.T) {
		store := newStore()

		ran := false
		voidThunk := func() ThunkResultVoid {
			return func(dispatch redux.Dispatch, getState func() State, _ any) any {
				ran = true

				state := getState()

				// expectTypeOf(state).toHaveProperty('foo')
				if state.Foo != initialState.Foo {
					t.Errorf("expected state.Foo to be %q, got %q", initialState.Foo, state.Foo)
				}

				thunk.DispatchAction[Actions](dispatch, FooAction{})
				thunk.DispatchAction[Actions](dispatch, BarAction{Result: 5})

				// Can dispatch another thunk action.
				if got := thunk.DispatchThunk(dispatch, anotherThunkAction()); got != "hello" {
					t.Errorf("expected the nested thunk to return %q, got %q", "hello", got)
				}

				return nil
			}
		}

		thunk.DispatchThunk(store.Dispatch, voidThunk())
		if !ran {
			t.Error("expected the thunk to have been run")
		}

		thunk.DispatchAction[Actions](store.Dispatch, FooAction{})
		thunk.DispatchAction[Actions](store.Dispatch, BarAction{Result: 5})
	})

	t.Run("issue #248: Need a union overload to handle generic dispatched types", func(t *testing.T) {
		// https://github.com/reduxjs/redux-thunk/issues/248
		//
		// The original declares a wrapper whose parameter is the union
		// `Action | ThunkAction<any, any, unknown, AnyAction>` and checks that
		// dispatching it type-checks thanks to the third overload. Go has no
		// unions, so the parameter degenerates to `any` and DispatchEither is
		// the corresponding entry point.
		store := newStore()
		dispatch := redux.Dispatch(store.Dispatch)

		dispatchWrap := func(actionOrThunk any) any {
			// Should not have an error here thanks to the extra union overload.
			return thunk.DispatchEither(dispatch, actionOrThunk)
		}

		// The union really does admit both arms at runtime.
		if got := dispatchWrap(FooAction{}); got != (FooAction{}) {
			t.Errorf("expected a plain action to be returned unchanged, got %#v", got)
		}
		if got := dispatchWrap(anotherThunkAction()); got != "hello" {
			t.Errorf("expected the thunk arm to return %q, got %#v", "hello", got)
		}
	})

	t.Run("store thunk arg", func(t *testing.T) {
		storeThunkArg := redux.CreateStore[State](
			fakeReducer,
			redux.WithPreloadedState(initialState),
			redux.WithEnhancer(redux.ApplyMiddleware(
				thunk.WithExtraArgument[State]("bar").AsMiddleware(),
			)),
		)

		thunk.DispatchAction[Actions](storeThunkArg.Dispatch, FooAction{})

		ran := false
		thunk.DispatchThunk(storeThunkArg.Dispatch, ThunkResultWithExtraTS(
			func(dispatch redux.Dispatch, _ func() State, extraArg string) any {
				ran = true

				// expectTypeOf(extraArg).toBeString()
				if extraArg != "bar" {
					t.Errorf("expected extraArg to be %q, got %q", "bar", extraArg)
				}

				thunk.DispatchAction[Actions](dispatch, FooAction{})

				// The original notes that
				// `expectTypeOf(store.dispatch).toBeCallableWith({ type: 'BAR' })`
				// does not work in this case, because BarAction requires its
				// `result` field. The untyped dispatch is the fallback.
				dispatch(BarAction{})

				thunk.DispatchAction[Actions](dispatch, BarAction{Result: 5})

				dispatch(BazAction{})

				return nil
			}))

		if !ran {
			t.Error("expected the thunk to have been run")
		}
	})

	t.Run("call dispatch async with any action", func(t *testing.T) {
		store := newStore()

		callDispatchAsyncAnyAction := func(dispatch redux.Dispatch) {
			asyncThunk := func() ThunkResultFutureVoid {
				return func(redux.Dispatch, func() State, any) *async.Future[any] {
					return async.Resolved[any](nil)
				}
			}

			future := thunk.DispatchThunk(dispatch, asyncThunk())
			if _, err := future.Await(); err != nil {
				t.Errorf("unexpected error awaiting the async thunk: %v", err)
			}
		}

		callDispatchAsyncAnyAction(store.Dispatch)
	})

	t.Run("call dispatch async with specific actions", func(t *testing.T) {
		// In TypeScript this block differs from the previous one only in the
		// BasicAction type argument of ThunkDispatch (`any` versus `Actions`).
		// That parameter was dropped in the port, so the two blocks are
		// necessarily identical here. It is kept for 1:1 correspondence with the
		// original suite.
		store := newStore()

		callDispatchAsyncSpecificActions := func(dispatch redux.Dispatch) {
			asyncThunk := func() ThunkResultFutureVoid {
				return func(redux.Dispatch, func() State, any) *async.Future[any] {
					return async.Resolved[any](nil)
				}
			}

			future := thunk.DispatchThunk(dispatch, asyncThunk())
			if _, err := future.Await(); err != nil {
				t.Errorf("unexpected error awaiting the async thunk: %v", err)
			}
		}

		callDispatchAsyncSpecificActions(store.Dispatch)
	})

	t.Run("call dispatch any", func(t *testing.T) {
		store := newStore()

		callDispatchAny := func(dispatch redux.Dispatch) {
			// `const asyncThunk = (): any => () => ({}) as Promise<void>`
			asyncThunk := func() any {
				return func(redux.Dispatch, func() State, any) *async.Future[any] {
					return async.Resolved[any](nil)
				}
			}

			// `dispatch(asyncThunk()).then(...)` -- the result is `any`, so the
			// caller must assert before chaining, which is the divergence
			// DispatchEither documents.
			result := thunk.DispatchEither(dispatch, asyncThunk())

			future, ok := result.(*async.Future[any])
			if !ok {
				t.Fatalf("expected a *async.Future[any], got %T", result)
			}

			done := async.Then(future, func(any) (string, error) { return "done", nil })
			if got, err := done.Await(); err != nil || got != "done" {
				t.Errorf("expected the continuation to produce %q, got %q (err %v)", "done", got, err)
			}
		}

		callDispatchAny(store.Dispatch)
	})

	t.Run("thunk actions", func(t *testing.T) {
		store := newStore()

		// The original builds this record with redux's `bindActionCreators` and
		// marks the assignment `@ts-expect-error`, because without a global
		// module overload the bound results are typed as the action creators'
		// return values rather than the thunks' return values.
		//
		// redux.BindActionCreators has the same shortcoming for the same reason
		// -- it is reflective and erases results to `any` -- so the compile
		// failure is reproduced in testdata/typeerrors/bind_action_creators.go.
		// The arity-indexed Bind helpers are the port's working alternative and
		// give exactly the types ThunkActionDispatch describes.
		actions := struct {
			AnotherThunkAction func() string
			PromiseThunkAction func() *async.Future[bool]
			StandardAction     func() FooAction
		}{
			AnotherThunkAction: thunk.Bind0(store.Dispatch, anotherThunkAction),
			PromiseThunkAction: thunk.Bind0(store.Dispatch, promiseThunkAction),
			StandardAction:     thunk.BindPlain0(store.Dispatch, standardAction),
		}

		// expectTypeOf(actions.anotherThunkAction()).toBeString()
		if got := actions.AnotherThunkAction(); got != "hello" {
			t.Errorf("expected anotherThunkAction to return %q, got %q", "hello", got)
		}

		// expectTypeOf(actions.promiseThunkAction()).resolves.toBeBoolean()
		resolved, err := actions.PromiseThunkAction().Await()
		if err != nil {
			t.Errorf("unexpected error awaiting promiseThunkAction: %v", err)
		}
		if resolved {
			t.Errorf("expected promiseThunkAction to resolve to false, got %v", resolved)
		}

		// expectTypeOf(actions.standardAction()).toHaveProperty('type').toBeString()
		if got := actions.StandardAction().ActionType(); got != "FOO" {
			t.Errorf("expected standardAction().ActionType() to be %q, got %q", "FOO", got)
		}

		// const untypedStore = createStore(fakeReducer, applyMiddleware(thunk))
		//
		// The untyped store is created at State = any so that the default Thunk
		// singleton applies without specialisation, mirroring the original's use
		// of the bare `thunk` export. Thunks written against the concrete State
		// type still run: the middleware adapts the `func() any` getState it
		// holds to the `func() State` the thunk declares.
		untypedStore := redux.CreateStore[any](
			func(state any, _ any) any { return state },
			redux.WithPreloadedState[any](initialState),
			redux.WithEnhancer(redux.ApplyMiddleware(thunk.Thunk.AsMiddleware())),
		)

		if got := untypedStore.Dispatch(anotherThunkAction()); got != "hello" {
			t.Errorf("expected the untyped store to return %q, got %#v", "hello", got)
		}

		untypedResult := untypedStore.Dispatch(promiseThunkAction())
		untypedFuture, ok := untypedResult.(*async.Future[bool])
		if !ok {
			t.Fatalf("expected a *async.Future[bool], got %T", untypedResult)
		}
		if _, err := untypedFuture.Await(); err != nil {
			t.Errorf("unexpected error awaiting promiseThunkAction on the untyped store: %v", err)
		}
	})
}
