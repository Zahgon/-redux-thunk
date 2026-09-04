// want: cannot use
//
// Ports the positive assertion `expectTypeOf(state).toHaveProperty('foo')` by
// its contrapositive: a thunk declared against the store's state type cannot
// receive a getState that returns some other state shape.

package typeerrors

import (
	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/redux"
)

type State struct{ Foo string }

type OtherState struct{ Bar int }

var WrongStateThunk thunk.ThunkAction[any, State, any] = func(
	dispatch redux.Dispatch,
	getState func() OtherState,
	_ any,
) any {
	return getState().Bar
}
