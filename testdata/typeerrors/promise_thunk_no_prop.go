// want: Prop undefined
//
// Ports the negative assertion:
//
//	expectTypeOf(actions.promiseThunkAction()).not.toHaveProperty('prop')
//
// The bound creator returns a *async.Future[bool], which has no Prop field.

package typeerrors

import (
	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/async"
	"github.com/reduxjs/redux-thunk-go/redux"
)

type State struct{ Foo string }

func PromiseThunkAction() thunk.ThunkAction[*async.Future[bool], State, any] {
	return func(redux.Dispatch, func() State, any) *async.Future[bool] {
		return async.Resolved(false)
	}
}

func FutureHasNoProp(dispatch redux.Dispatch) {
	bound := thunk.Bind0(dispatch, PromiseThunkAction)
	_ = bound().Prop
}
