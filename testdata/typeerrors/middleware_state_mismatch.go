// want: cannot use
//
// Documents the divergence behind TypedThunk. TypeScript re-types the existing
// `thunk` singleton structurally:
//
//	applyMiddleware(thunk as ThunkMiddleware<State, Actions>)
//
// Go has no such cast: Thunk is a ThunkMiddleware[any, any] and cannot be
// applied to a store whose state is State. TypedThunk[State]() constructs the
// correctly specialised instance instead.

package typeerrors

import (
	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/redux"
)

type State struct{ Foo string }

var Enhancer = redux.ApplyMiddleware[State](thunk.Thunk.AsMiddleware())
