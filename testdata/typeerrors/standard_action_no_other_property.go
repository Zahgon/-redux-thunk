// want: Other undefined
//
// Ports the negative assertion:
//
//	expectTypeOf(actions.standardAction()).not.toHaveProperty('other')
//
// The bound plain action creator returns the action itself, which has a type
// but no `other` field.

package typeerrors

import (
	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/redux"
)

type FooAction struct{}

func (FooAction) ActionType() string { return "FOO" }

func StandardAction() FooAction { return FooAction{} }

func ActionHasNoOtherProperty(dispatch redux.Dispatch) {
	bound := thunk.BindPlain0(dispatch, StandardAction)
	_ = bound().Other
}
