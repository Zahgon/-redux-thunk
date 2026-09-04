// Package redux is a minimal Go port of the parts of the `redux@^5` package
// that redux-thunk depends on.
//
// The original TypeScript redux-thunk declares `redux` as a *peer dependency*
// and imports only three type-level symbols from it:
//
//	import type { Action, AnyAction, Middleware } from 'redux'
//
// Its runtime test suite never touches Redux itself: test/index.test.ts calls
// the middleware directly with stub dispatch and getState functions.
// `createStore` and `applyMiddleware` appear only in typescript_test, which is
// type-checked but never executed. What genuinely needs a store is the README --
// nearly every example builds one -- and the ported examples must compile and
// run. Because there is no canonical Redux port for Go, this package supplies
// exactly those pieces and no more.
//
// This is therefore *new* code, not migrated code, and it is the part of the
// repository least protected by the upstream suite. It is covered by its own
// tests, and its divergences from redux@5 are listed in docs/MIGRATION.md.
//
// Fidelity notes versus the JavaScript original:
//
//   - Error strings raised by the store are reproduced verbatim so that
//     behaviour-sensitive consumers see the same messages.
//   - JavaScript is single-threaded; Go is not. The store therefore guards its
//     state with a mutex. See the package documentation on Store for the
//     concurrency contract.
package redux

// Action is the Go analogue of the redux `Action` interface:
//
//	interface Action<T extends string = string> { type: T }
//
// TypeScript uses structural typing, so any object with a `type: string` field
// satisfies `Action`. Go has no structural typing for structs, so the contract
// is expressed as a one-method interface. Implementations return their action
// type string from ActionType.
type Action interface {
	ActionType() string
}

// AnyAction is the Go analogue of redux's `AnyAction`:
//
//	interface AnyAction extends Action { [extraProps: string]: any }
//
// The TypeScript type is an object with a `type` field plus an index signature
// permitting arbitrary extra properties of type `any`. The closest faithful Go
// representation is a dynamic string-keyed map. The action type is stored under
// the TypeKey field name, matching the JavaScript `type` property.
type AnyAction map[string]any

// UnknownAction is the Go analogue of redux v5's `UnknownAction`, which is
// identical to AnyAction except that extra properties are typed `unknown`
// rather than `any`. Go draws no such distinction (both erase to `any`), so
// this is an alias retained for naming parity with the source package.
type UnknownAction = AnyAction

// TypeKey is the map key that holds an AnyAction's action type. It mirrors the
// `type` property of a JavaScript Redux action object.
const TypeKey = "type"

// ActionType implements Action. It returns the value stored under TypeKey, or
// the empty string when the key is absent or does not hold a string. This
// mirrors JavaScript, where reading a missing property yields `undefined`
// rather than throwing.
func (a AnyAction) ActionType() string {
	t, ok := a[TypeKey].(string)
	if !ok {
		return ""
	}
	return t
}

// NewAction builds an AnyAction with the given type and extra properties. It is
// the Go equivalent of an object literal such as `{ type: 'BAR', result: 5 }`.
func NewAction(actionType string, props map[string]any) AnyAction {
	a := make(AnyAction, len(props)+1)
	for k, v := range props {
		a[k] = v
	}
	a[TypeKey] = actionType
	return a
}

// Dispatch is the store's dispatch function.
//
// The TypeScript signature is `(action: A) => A`, but the thunk middleware
// widens it so that dispatching a thunk returns whatever the thunk returns.
// Since Go has neither function overloading nor union return types, the runtime
// signature is fully erased to `func(any) any`. Static typing is recovered at
// the call site by the generic helpers in the thunk package (DispatchThunk,
// DispatchAction, DispatchEither).
type Dispatch func(action any) any

// Next is the `next` link in a middleware chain: the dispatch function of the
// next middleware, or the raw store dispatch for the last middleware.
//
// It is a distinct named type from Dispatch purely for documentation value;
// the two share an underlying type and are freely convertible.
type Next func(action any) any

// Reducer is the Go analogue of `Reducer<S, A>`:
//
//	(state: S | undefined, action: A) => S
//
// The action parameter is `any` rather than a constrained type parameter
// because the middleware chain erases action types (see Dispatch). Reducers are
// expected to type-switch on the concrete action, exactly as a JavaScript
// reducer switches on `action.type`.
type Reducer[S any] func(state S, action any) S

// MiddlewareAPI is the object handed to a middleware's outermost function. It
// corresponds to:
//
//	interface MiddlewareAPI<D extends Dispatch, S> { dispatch: D; getState(): S }
//
// The TypeScript thunk middleware destructures this value:
//
//	({ dispatch, getState }) => next => action => ...
//
// Destructuring throws a TypeError in JavaScript only when the value being
// destructured is `null` or `undefined`; an empty object is destructured
// happily into `undefined` fields. Middleware therefore receives *MiddlewareAPI
// rather than MiddlewareAPI, so that a nil pointer models the absent argument
// while a zero struct models the empty object.
type MiddlewareAPI[S any] struct {
	Dispatch Dispatch
	GetState func() S
}

// Middleware is the Go analogue of redux's `Middleware` type. The TypeScript
// declaration carries a phantom `_DispatchExt` parameter used only to widen the
// store's dispatch type; Go has no equivalent mechanism, so it is dropped and
// the widening is provided by the thunk package's generic dispatch helpers.
type Middleware[S any] func(api *MiddlewareAPI[S]) func(next Next) Dispatch

// StoreCreator matches redux's `StoreEnhancerStoreCreator`: the function an
// enhancer wraps in order to intercept store construction.
type StoreCreator[S any] func(reducer Reducer[S], preloadedState S) Store[S]

// Enhancer is the Go analogue of `StoreEnhancer`. ApplyMiddleware returns one.
type Enhancer[S any] func(next StoreCreator[S]) StoreCreator[S]

// Store is the Go analogue of redux's `Store` interface.
//
// Concurrency contract (a Go-only concern with no JavaScript counterpart):
// every method is safe for concurrent use. GetState returns the state value as
// of the moment of the call. Dispatch serialises reducer execution. Calling
// Dispatch from inside a reducer panics, mirroring Redux's own reentrancy
// guard.
type Store[S any] interface {
	// Dispatch sends an action through the middleware chain and reducer.
	Dispatch(action any) any
	// GetState returns the current state tree.
	GetState() S
	// Subscribe registers a change listener and returns a function that
	// removes it. Calling the returned function more than once is a no-op.
	Subscribe(listener func()) (unsubscribe func())
	// ReplaceReducer swaps the reducer currently used by the store.
	ReplaceReducer(reducer Reducer[S])
}
