// want: too many arguments
//
// Ports the negative assertion:
//
//	expectTypeOf(dispatch).parameter(1).not.toMatchTypeOf(42)
//
// dispatch accepts exactly one argument; there is no second parameter for 42 to
// match.

package typeerrors

import "github.com/reduxjs/redux-thunk-go/redux"

type FooAction struct{}

func (FooAction) ActionType() string { return "FOO" }

func DispatchWithTwoArguments(dispatch redux.Dispatch) {
	dispatch(FooAction{}, 42)
}
