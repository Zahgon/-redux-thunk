// want: cannot use
//
// Ports the negative assertion:
//
//	expectTypeOf(actions.anotherThunkAction()).not.toBeBoolean()
//
// The bound thunk action creator returns the thunk's return type, string, so it
// cannot be used where a bool is required.

package typeerrors

import (
	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/redux"
)

type State struct{ Foo string }

func AnotherThunkAction() thunk.ThunkAction[string, State, any] {
	return func(redux.Dispatch, func() State, any) string { return "hello" }
}

func ResultIsNotBoolean(dispatch redux.Dispatch) bool {
	bound := thunk.Bind0(dispatch, AnotherThunkAction)
	return bound()
}
