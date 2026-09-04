package thunk

import (
	"fmt"

	"github.com/reduxjs/redux-thunk-go/redux"
)

// This file supplies the three ThunkDispatch overloads from src/types.ts as
// generic free functions, because Go has neither function overloading nor
// method-level type parameters on interfaces.
//
// The original carries this comment on the interface, and the ordering rationale
// it describes is preserved here as the ordering of these three helpers:
//
//	When the thunk middleware is added, `store.dispatch` now has three overloads
//	(NOTE: the order here matters for correct behavior and is very fragile - do
//	not reorder these!)

// DispatchThunk is overload 1:
//
//	<ReturnType>(thunkAction: ThunkAction<ReturnType, State, ExtraThunkArg, BasicAction>): ReturnType
//
// It accepts a thunk function, runs it through the middleware chain, and
// returns whatever the thunk itself returns, statically typed as R.
//
// Panics with a *TypeError if the middleware chain returns a value that is not
// assignable to R. A store without the thunk middleware never reaches that
// point: redux.Store rejects a function action outright, with Redux's own
// "Actions must be plain objects" message naming the missing middleware.
//
// A nil result means the thunk returned nothing, which is JavaScript's
// `undefined`; it maps to the zero value of R.
func DispatchThunk[R any, S any, E any](dispatch redux.Dispatch, thunkAction ThunkAction[R, S, E]) R {
	out := dispatch(thunkAction)
	if out == nil {
		var zero R
		return zero
	}
	typed, ok := out.(R)
	if !ok {
		var zero R
		panic(&TypeError{Message: fmt.Sprintf(
			"DispatchThunk: dispatch returned %T, expected %T; is the thunk middleware installed on this store?",
			out, zero)})
	}
	return typed
}

// DispatchAction is overload 2:
//
//	<Action extends BasicAction>(action: Action): Action
//
// It accepts a standard action value, and returns that action value.
//
// The TypeScript constraint `Action extends BasicAction` has no Go counterpart
// because BasicAction was dropped from ThunkAction (see its documentation).
// Constrain A to redux.Action at the call site if you want the action-shape
// guarantee.
//
// Panics with a *TypeError when a middleware replaced the result with something
// that is not an A. JavaScript returns the replacement and lets the static type
// be a lie; Go cannot, and returning the input instead would be a silent lie of
// its own -- the caller would receive a value the store never produced.
func DispatchAction[A any](dispatch redux.Dispatch, action A) A {
	out := dispatch(action)
	typed, ok := out.(A)
	if !ok {
		panic(&TypeError{Message: fmt.Sprintf(
			"DispatchAction: dispatch returned %T, expected %T; a middleware replaced the action result",
			out, action)})
	}
	return typed
}

// DispatchEither is overload 3:
//
//	<ReturnType, Action extends BasicAction>(
//	  action: Action | ThunkAction<ReturnType, State, ExtraThunkArg, BasicAction>
//	): Action | ReturnType
//
// The original notes that this overload exists purely to work around a
// TypeScript inference problem:
//
//	A union of the other two overloads. This overload exists to work around a
//	problem with TS inference (see https://github.com/microsoft/TypeScript/issues/14107)
//
// and it fixes https://github.com/reduxjs/redux-thunk/issues/248, where a caller
// holds a value that is statically `Action | ThunkAction<...>` and needs to
// dispatch it without narrowing first.
//
// KNOWN DIVERGENCE: Go has no union types, so the return type degenerates to
// `any`. The runtime behaviour is identical -- the thunk middleware inspects the
// value dynamically and does the right thing either way -- but the static
// result must be type-asserted by the caller. This is the single largest
// fidelity gap in the port and is documented in docs/MIGRATION.md.
func DispatchEither(dispatch redux.Dispatch, actionOrThunk any) any {
	return dispatch(actionOrThunk)
}
