// want: cannot use
//
// Ports the `@ts-expect-error` in the `thunk actions` block:
//
//	// Without a global module overload, this should fail
//	// @ts-expect-error
//	const actions: ActionDispatches = bindActionCreators({ ... }, store.dispatch)
//
// Redux's own bindActionCreators cannot express ThunkActionDispatch, so the
// assignment is rejected. redux.BindActionCreators has the same limitation for
// the same reason: it is reflective and erases every bound result to `any`.
// The port's Bind0..Bind3 helpers are the working alternative.

package typeerrors

import (
	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/redux"
)

type State struct{ Foo string }

func AnotherThunkAction() thunk.ThunkAction[string, State, any] {
	return func(redux.Dispatch, func() State, any) string { return "hello" }
}

type ActionDispatches struct {
	AnotherThunkAction func() string
}

func BindWithRedux(dispatch redux.Dispatch) ActionDispatches {
	bound := redux.BindActionCreators(map[string]any{
		"anotherThunkAction": AnotherThunkAction,
	}, dispatch)

	return ActionDispatches{AnotherThunkAction: bound["anotherThunkAction"]}
}
