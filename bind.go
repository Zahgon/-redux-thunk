package thunk

import (
	"github.com/reduxjs/redux-thunk-go/redux"
)

// This file ports `ThunkActionDispatch` from src/types.ts:
//
//	type ThunkActionDispatch<ActionCreator extends (...args: any[]) => ThunkAction<any, any, any, any>> =
//	  (...args: Parameters<ActionCreator>) => ReturnType<ReturnType<ActionCreator>>
//
// A generic type that takes a thunk action creator and returns a function
// signature which matches how it would appear after being processed using
// bindActionCreators(): a function that takes the arguments of the outer
// function, and returns the return type of the inner "thunk" function.
//
// KNOWN DIVERGENCE: this is a variadic type-level transform. Go has no variadic
// generics, so the single TypeScript type becomes an arity-indexed family of
// functions, Bind0 through Bind3. Thunk action creators taking more than three
// arguments should group them into a struct, which is idiomatic Go regardless.
// A fully dynamic, reflection-based alternative is redux.BindActionCreators,
// which preserves the parameter list but erases the result to `any`.

// Bind0 binds a zero-argument thunk action creator to a dispatch function.
//
//	actions.anotherThunkAction := thunk.Bind0(store.Dispatch, anotherThunkAction)
//	var s string = actions.anotherThunkAction()
func Bind0[R any, S any, E any](
	dispatch redux.Dispatch,
	actionCreator func() ThunkAction[R, S, E],
) func() R {
	return func() R {
		return DispatchThunk(dispatch, actionCreator())
	}
}

// Bind1 binds a one-argument thunk action creator to a dispatch function.
func Bind1[A1 any, R any, S any, E any](
	dispatch redux.Dispatch,
	actionCreator func(A1) ThunkAction[R, S, E],
) func(A1) R {
	return func(a1 A1) R {
		return DispatchThunk(dispatch, actionCreator(a1))
	}
}

// Bind2 binds a two-argument thunk action creator to a dispatch function.
func Bind2[A1 any, A2 any, R any, S any, E any](
	dispatch redux.Dispatch,
	actionCreator func(A1, A2) ThunkAction[R, S, E],
) func(A1, A2) R {
	return func(a1 A1, a2 A2) R {
		return DispatchThunk(dispatch, actionCreator(a1, a2))
	}
}

// Bind3 binds a three-argument thunk action creator to a dispatch function.
func Bind3[A1 any, A2 any, A3 any, R any, S any, E any](
	dispatch redux.Dispatch,
	actionCreator func(A1, A2, A3) ThunkAction[R, S, E],
) func(A1, A2, A3) R {
	return func(a1 A1, a2 A2, a3 A3) R {
		return DispatchThunk(dispatch, actionCreator(a1, a2, a3))
	}
}

// BindPlain0 binds a zero-argument plain action creator to a dispatch function,
// returning the action itself. It is the non-thunk counterpart of Bind0 and
// corresponds to `standardAction` in the original type tests.
func BindPlain0[A any](dispatch redux.Dispatch, actionCreator func() A) func() A {
	return func() A {
		return DispatchAction(dispatch, actionCreator())
	}
}

// BindPlain1 binds a one-argument plain action creator to a dispatch function.
func BindPlain1[A1 any, A any](dispatch redux.Dispatch, actionCreator func(A1) A) func(A1) A {
	return func(a1 A1) A {
		return DispatchAction(dispatch, actionCreator(a1))
	}
}
