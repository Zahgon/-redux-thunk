// want: cannot use
//
// Ports the positive assertion `expectTypeOf(extraArg).toBeString()` by its
// contrapositive: a middleware built with a string extra argument is not a
// middleware whose extra argument is an int.

package typeerrors

import thunk "github.com/reduxjs/redux-thunk-go"

type State struct{ Foo string }

var WrongExtra thunk.ThunkMiddleware[State, int] = thunk.WithExtraArgument[State]("bar")
