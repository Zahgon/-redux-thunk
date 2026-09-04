// want: does not implement Actions
//
// Ports the negative assertion:
//
//	expectTypeOf(dispatch).parameter(0).not.toMatchTypeOf({ type: 'BAZ' })
//
// `{ type: 'BAZ' }` is not a member of the Actions union, so TypeScript rejects
// it. The Go encoding of the union is a marker interface, and BazAction does not
// implement the marker.

package typeerrors

import (
	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/redux"
)

type Actions interface {
	redux.Action
	isActions()
}

type FooAction struct{}

func (FooAction) ActionType() string { return "FOO" }
func (FooAction) isActions()         {}

type BazAction struct{}

func (BazAction) ActionType() string { return "BAZ" }

func DispatchBaz(dispatch redux.Dispatch) {
	thunk.DispatchAction[Actions](dispatch, BazAction{})
}
