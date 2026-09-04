// Package thunk is a Go port of redux-thunk v3.1.0.
//
// Thunk middleware for Redux. It allows writing functions with logic inside
// that can interact with a Redux store's Dispatch and GetState methods.
//
// The thunk middleware looks for any functions that were passed to
// store.Dispatch. If the "action" is really a function, it is called with the
// store's Dispatch and GetState methods (plus any configured "extra argument")
// and its result is returned. Otherwise the action is passed down the
// middleware chain as usual.
//
// # Quick start
//
//	store := redux.CreateStore(rootReducer,
//	    redux.WithEnhancer(redux.ApplyMiddleware(
//	        thunk.TypedThunk[State]().AsMiddleware(),
//	    )))
//
//	store.Dispatch(thunk.ThunkAction[any, State, any](
//	    func(dispatch redux.Dispatch, getState func() State, _ any) any {
//	        dispatch(Increment{})
//	        return nil
//	    }))
//
// # Relationship to the TypeScript original
//
// This package is a behaviour-preserving port. Every runtime guarantee of
// redux-thunk is reproduced exactly. The type-level API required adaptation
// because Go lacks function overloading, union types, variadic generics and
// default type parameters. See docs/MIGRATION.md for the full divergence table.
package thunk

import (
	"github.com/reduxjs/redux-thunk-go/redux"
)

// Dispatch re-exports redux.Dispatch for convenience, so that consumers of
// thunks need not import the redux package for the common case.
type Dispatch = redux.Dispatch

// ThunkAction is a "thunk" action: a callback function that can be dispatched
// to the Redux store.
//
// Also known as the "thunk inner function", when used with the typical pattern
// of an action creator function that returns a thunk action.
//
// TypeScript:
//
//	type ThunkAction<ReturnType, State, ExtraThunkArg, BasicAction extends Action> = (
//	  dispatch: ThunkDispatch<State, ExtraThunkArg, BasicAction>,
//	  getState: () => State,
//	  extraArgument: ExtraThunkArg,
//	) => ReturnType
//
// The fourth type parameter, BasicAction, is dropped. In TypeScript it exists
// solely to parameterise the ThunkDispatch overloads that constrain which plain
// actions the injected dispatch will accept. Go's Dispatch is type-erased
// (func(any) any), so there is nothing for BasicAction to constrain; action
// typing is instead enforced at the reducer via a type switch, and at the call
// site via DispatchAction.
//
// R is the thunk's return type, S the state type, and E the extra-argument type.
//
// A thunk need not have this exact type: the middleware accepts any function
// value, mirroring `typeof action === 'function'` in JavaScript. Any arity is
// accepted -- parameters beyond the third receive their zero value and a
// variadic slot receives whatever the fixed parameters did not consume, exactly
// as JavaScript supplies `undefined` and a rest array. A thunk may declare at
// most one result, since a JavaScript function returns exactly one value.
type ThunkAction[R any, S any, E any] func(dispatch redux.Dispatch, getState func() S, extra E) R

// ThunkActionAny is ThunkAction with every type parameter defaulted to `any`.
// It stands in for TypeScript's default type parameters
// (`State = any`, `ExtraThunkArg = undefined`), which Go does not support.
type ThunkActionAny = ThunkAction[any, any, any]

// ThunkMiddleware is the type of the middleware produced by WithExtraArgument.
//
// TypeScript:
//
//	type ThunkMiddleware<State = any, BasicAction extends Action = AnyAction, ExtraThunkArg = undefined> =
//	  Middleware<ThunkDispatch<State, ExtraThunkArg, BasicAction>, State, ThunkDispatch<State, ExtraThunkArg, BasicAction>>
//
// The `_DispatchExt` parameter of redux's Middleware exists to widen the
// store's dispatch type so that `store.dispatch(someThunk())` type-checks.
// Go cannot rewrite a value's type through a wrapper, so the widening is
// provided instead by the generic helpers DispatchThunk, DispatchAction and
// DispatchEither.
//
// E is retained as a type parameter (unlike BasicAction) because it is
// genuinely load-bearing: it types the third argument passed to every thunk.
// The API is taken by pointer so that "no argument" (nil) can be distinguished
// from "an empty object" (a pointer to the zero struct), which is the only way
// to reproduce JavaScript's destructuring semantics faithfully. See
// createThunkMiddleware.
type ThunkMiddleware[S any, E any] func(api *redux.MiddlewareAPI[S]) func(next redux.Next) redux.Dispatch

// AsMiddleware converts a ThunkMiddleware into a plain redux.Middleware so that
// it can be passed to redux.ApplyMiddleware.
//
// This method is the Go analogue of TypeScript's structural assignability,
// which lets a ThunkMiddleware be used wherever a Middleware is expected
// without any explicit conversion. Go requires named types with identical
// underlying types to be converted explicitly, so this is the seam.
func (m ThunkMiddleware[S, E]) AsMiddleware() redux.Middleware[S] {
	return redux.Middleware[S](m)
}

// ThunkDispatch documents the three-overload dispatch signature of the
// TypeScript original:
//
//	interface ThunkDispatch<State, ExtraThunkArg, BasicAction extends Action> {
//	  <ReturnType>(thunkAction: ThunkAction<ReturnType, State, ExtraThunkArg, BasicAction>): ReturnType
//	  <Action extends BasicAction>(action: Action): Action
//	  <ReturnType, Action extends BasicAction>(
//	    action: Action | ThunkAction<ReturnType, State, ExtraThunkArg, BasicAction>
//	  ): Action | ReturnType
//	}
//
// The original carries this warning, which is worth preserving:
//
//	NOTE: the order here matters for correct behavior and is very fragile -
//	do not reorder these!
//
// Go has neither overloading nor method-level type parameters on interfaces, so
// the three overloads cannot be expressed as one type. They are provided
// instead as three generic free functions in dispatch.go:
//
//	overload 1  ->  DispatchThunk
//	overload 2  ->  DispatchAction
//	overload 3  ->  DispatchEither  (degenerate: returns `any`, see its docs)
//
// The type itself is the erased runtime form actually used by the middleware.
type ThunkDispatch = redux.Dispatch
