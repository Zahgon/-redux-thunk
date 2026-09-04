package thunk

import (
	"fmt"
	"reflect"

	"github.com/reduxjs/redux-thunk-go/redux"
)

// TypeError is the error value panicked by this package. It stands in for the
// JavaScript TypeError that the original throws in the equivalent situations
// (destructuring `undefined`, calling a non-function `next`).
type TypeError struct {
	Message string
}

func (e *TypeError) Error() string { return e.Message }

// createThunkMiddleware is a function that accepts a potential "extra argument"
// value to be injected later, and returns an instance of the thunk middleware
// that uses that value.
//
// This is a direct port of the TypeScript original:
//
//	function createThunkMiddleware<State = any, BasicAction extends Action = AnyAction, ExtraThunkArg = undefined>(
//	  extraArgument?: ExtraThunkArg
//	) {
//	  const middleware: ThunkMiddleware<State, BasicAction, ExtraThunkArg> =
//	    ({ dispatch, getState }) =>
//	    next =>
//	    action => {
//	      if (typeof action === 'function') {
//	        return action(dispatch, getState, extraArgument)
//	      }
//	      return next(action)
//	    }
//	  return middleware
//	}
//
// The three-level currying is preserved verbatim: each level is a separately
// obtainable single-parameter function value, because the original test suite
// obtains and asserts on each level independently.
func createThunkMiddleware[S any, E any](extraArgument E) ThunkMiddleware[S, E] {
	// Standard Redux middleware definition pattern:
	// See: https://redux.js.org/tutorials/fundamentals/part-4-store#writing-custom-middleware
	return func(api *redux.MiddlewareAPI[S]) func(next redux.Next) redux.Dispatch {
		// The TypeScript version destructures `{ dispatch, getState }` from its
		// argument. Destructuring throws only when that argument is `null` or
		// `undefined`: `thunk()` throws, but `thunk({})` succeeds and yields a
		// middleware whose `dispatch` and `getState` are `undefined`. The API
		// is therefore taken by pointer, so that a nil pointer can mean
		// "argument absent" while a non-nil pointer to a zero struct means
		// "argument present but empty" -- a distinction a value receiver cannot
		// express.
		if api == nil {
			panic(&TypeError{Message: "Cannot destructure property 'dispatch' of 'undefined' as it is undefined."})
		}

		return func(next redux.Next) redux.Dispatch {
			return func(action any) any {
				// The thunk middleware looks for any functions that were
				// passed to `store.Dispatch`. If this "action" is really a
				// function, call it and return the result.
				//
				// Inject the store's `Dispatch` and `GetState` methods, as
				// well as any "extra arg".
				if result, ok := invokeThunk(action, api, extraArgument); ok {
					return result
				}

				// Otherwise, pass the action down the middleware chain as
				// usual. A nil `next` is the Go equivalent of JavaScript's
				// `undefined` next, and calling it fails the same way.
				if next == nil {
					panic(&TypeError{Message: "next is not a function"})
				}
				return next(action)
			}
		}
	}
}

// Thunk is the default thunk middleware instance, with no extra argument.
//
// It is the port of:
//
//	export const thunk = createThunkMiddleware()
//
// Like the original it is a module-level singleton constructed exactly once,
// with State defaulted to `any` and the extra argument defaulted to the Go
// equivalent of `undefined`, namely a nil `any`.
//
// Use TypedThunk when the store's state type is known; it is the analogue of
// the TypeScript cast `thunk as ThunkMiddleware<State, Actions>`.
var Thunk = WithExtraArgument[any, any](nil)

// WithExtraArgument is the factory function, exported so users can create a
// customized version with whatever "extra arg" they want to inject into their
// thunks.
//
// It is the port of:
//
//	export const withExtraArgument = createThunkMiddleware
//
// In TypeScript the export is literally the factory function itself. Go cannot
// bind a generic function to a variable without instantiating it, so this is a
// thin forwarding wrapper over the single shared implementation; there is still
// exactly one construction path.
//
// The state type parameter comes first so that it can be supplied explicitly
// while the extra-argument type is inferred from the value:
//
//	mw := thunk.WithExtraArgument[MyState](myAPIService)
func WithExtraArgument[S any, E any](extraArgument E) ThunkMiddleware[S, E] {
	return createThunkMiddleware[S, E](extraArgument)
}

// TypedThunk returns the default thunk middleware specialised to a concrete
// state type, with no extra argument.
//
// It is the Go analogue of the TypeScript cast used throughout the original's
// type tests:
//
//	applyMiddleware(thunk as ThunkMiddleware<State, Actions>)
//
// TypeScript can re-type the existing `thunk` singleton because its middleware
// type is structural. Go's ThunkMiddleware[any, any] and ThunkMiddleware[S, any]
// are distinct types with different underlying types, so a new instance must be
// constructed. Behaviourally the two are identical.
func TypedThunk[S any]() ThunkMiddleware[S, any] {
	return WithExtraArgument[S, any](nil)
}

// IsThunk reports whether the middleware would treat the given action as a
// thunk. It is the exported form of the original's `typeof action === 'function'`
// check.
func IsThunk(action any) bool {
	if action == nil {
		return false
	}
	v := reflect.ValueOf(action)
	return v.IsValid() && v.Kind() == reflect.Func
}

// thunkArgumentCount is the number of arguments the middleware passes to a
// thunk: dispatch, getState and the extra argument.
const thunkArgumentCount = 3

// errNilThunk is raised for a typed nil function value. Both the fast path and
// the reflective path use it so that the same input cannot produce two
// different failure modes depending on which path happened to match.
func errNilThunk() *TypeError {
	return &TypeError{Message: "thunk action is a nil function value"}
}

// invokeThunk implements `typeof action === 'function' && action(dispatch, getState, extraArgument)`.
//
// It reports ok == false when the action is not a function, in which case the
// caller forwards to `next`.
//
// JavaScript passes all three arguments regardless of how many the thunk
// declares; a thunk written as `dispatch => {...}` simply ignores the rest, and
// one written as `(...args) => args.length` receives all three. Go requires an
// exact arity match, so the invocation is adapted:
//
//   - a thunk declaring 0..3 parameters receives that prefix of
//     (dispatch, getState, extraArgument);
//   - a thunk declaring more receives the zero value of each further parameter,
//     which is this port's mapping of JavaScript's `undefined`;
//   - a variadic thunk receives every argument the fixed parameters did not
//     consume in its variadic slot, exactly as JavaScript's rest parameter does.
//
// Fast paths handle the canonical signatures without reflection. This matters
// beyond performance: only on these paths do the `dispatch` and `getState`
// values reach the thunk *by identity*, unwrapped, which the original test
// suite asserts with `expect(dispatch).toBe(doDispatch)`. See coerce for the
// cases where identity cannot be preserved.
func invokeThunk[S any, E any](action any, api *redux.MiddlewareAPI[S], extraArgument E) (result any, ok bool) {
	if action == nil {
		return nil, false
	}

	switch fn := action.(type) {
	case ThunkAction[any, S, E]:
		if fn == nil {
			panic(errNilThunk())
		}
		return fn(api.Dispatch, api.GetState, extraArgument), true
	case func(redux.Dispatch, func() S, E) any:
		if fn == nil {
			panic(errNilThunk())
		}
		return fn(api.Dispatch, api.GetState, extraArgument), true
	case func(redux.Dispatch, func() S) any:
		if fn == nil {
			panic(errNilThunk())
		}
		return fn(api.Dispatch, api.GetState), true
	case func(redux.Dispatch) any:
		if fn == nil {
			panic(errNilThunk())
		}
		return fn(api.Dispatch), true
	case func() any:
		if fn == nil {
			panic(errNilThunk())
		}
		return fn(), true
	case func(redux.Dispatch, func() S, E):
		if fn == nil {
			panic(errNilThunk())
		}
		fn(api.Dispatch, api.GetState, extraArgument)
		return nil, true
	case func(redux.Dispatch, func() S):
		if fn == nil {
			panic(errNilThunk())
		}
		fn(api.Dispatch, api.GetState)
		return nil, true
	case func(redux.Dispatch):
		if fn == nil {
			panic(errNilThunk())
		}
		fn(api.Dispatch)
		return nil, true
	case func():
		if fn == nil {
			panic(errNilThunk())
		}
		fn()
		return nil, true
	}

	return invokeThunkReflect(action, api.Dispatch, api.GetState, extraArgument)
}

// invokeThunkReflect is the general path for thunks whose signature is not one
// of the canonical fast-path shapes, most importantly thunks with a concrete
// (non-`any`) return type such as ThunkAction[string, State, any].
func invokeThunkReflect(action any, dispatch redux.Dispatch, getState any, extra any) (any, bool) {
	v := reflect.ValueOf(action)
	if !v.IsValid() || v.Kind() != reflect.Func {
		return nil, false
	}
	if v.IsNil() {
		// A typed nil function is still `typeof === 'function'` in spirit, but
		// calling it would panic opaquely. Fail with a clear message instead.
		panic(errNilThunk())
	}

	t := v.Type()
	if t.NumOut() > 1 {
		// A JavaScript function returns exactly one value, so a multi-result
		// Go thunk has no source counterpart and no defensible mapping: the
		// dispatch chain can only propagate one value, and silently discarding
		// the rest would hide errors returned alongside the payload.
		panic(&TypeError{Message: fmt.Sprintf(
			"thunk action %s returns %d values; a thunk must return at most one "+
				"(return a struct or an *async.Future to carry several)", t, t.NumOut())})
	}

	fixed := t.NumIn()
	if t.IsVariadic() {
		fixed = t.NumIn() - 1
	}

	sources := [thunkArgumentCount]any{dispatch, getState, extra}
	args := make([]reflect.Value, 0, t.NumIn())
	for i := 0; i < fixed; i++ {
		var src any
		if i < len(sources) {
			src = sources[i]
		}
		args = append(args, coerceAny(src, t.In(i)))
	}
	if t.IsVariadic() {
		elem := t.In(t.NumIn() - 1).Elem()
		for i := fixed; i < len(sources); i++ {
			args = append(args, coerceAny(sources[i], elem))
		}
	}

	out := v.Call(args)
	if len(out) == 0 {
		// A thunk with no return value is the equivalent of a JavaScript
		// function returning `undefined`.
		return nil, true
	}
	return out[0].Interface(), true
}

// coerceAny converts an interface-held value into a reflect.Value of the wanted
// type, handling the nil case (Go's equivalent of `undefined`).
func coerceAny(src any, want reflect.Type) reflect.Value {
	if src == nil {
		return reflect.Zero(want)
	}
	return coerce(reflect.ValueOf(src), want)
}

// coerce adapts a reflect.Value to the wanted type.
//
// Identity preservation: when the source type already matches the wanted type
// the original value is returned untouched, so function identity survives.
// Adaptation via reflect.MakeFunc is only reached when the thunk declares a
// different state type than the middleware was instantiated with -- for
// instance a ThunkAction[R, any, E] dispatched into a TypedThunk[State] store.
// Identity cannot be preserved there by construction: `func() State` and
// `func() any` are different Go types, so no single value can inhabit both.
func coerce(v reflect.Value, want reflect.Type) reflect.Value {
	if !v.IsValid() {
		return reflect.Zero(want)
	}
	if v.Type() == want {
		return v
	}
	if v.Type().AssignableTo(want) {
		out := reflect.New(want).Elem()
		out.Set(v)
		return out
	}
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			// A nil interface is JavaScript's `undefined`, which this port maps
			// to the zero value. A getState that has not produced a state yet
			// must not turn a working thunk into a panic.
			return reflect.Zero(want)
		}
		return coerce(v.Elem(), want)
	}
	if v.Type().ConvertibleTo(want) {
		return v.Convert(want)
	}
	if want.Kind() == reflect.Func && v.Kind() == reflect.Func {
		return adaptFunc(v, want)
	}
	panic(&TypeError{Message: fmt.Sprintf(
		"cannot pass a value of type %s to a thunk parameter of type %s", v.Type(), want)})
}

// adaptFunc builds a shim of type `want` that forwards to `src`, converting
// arguments and results. It is used when a thunk declares a getState (or
// dispatch) signature that differs from the one the middleware holds, for
// example `func() State` against a middleware instantiated with `any`.
func adaptFunc(src reflect.Value, want reflect.Type) reflect.Value {
	st := src.Type()
	if st.NumIn() != want.NumIn() || st.IsVariadic() != want.IsVariadic() {
		panic(&TypeError{Message: fmt.Sprintf(
			"cannot adapt function of type %s to %s: parameter counts differ", st, want)})
	}
	return reflect.MakeFunc(want, func(in []reflect.Value) []reflect.Value {
		callArgs := make([]reflect.Value, len(in))
		for i := range in {
			callArgs[i] = coerce(in[i], st.In(i))
		}
		var got []reflect.Value
		if st.IsVariadic() {
			got = src.CallSlice(callArgs)
		} else {
			got = src.Call(callArgs)
		}
		results := make([]reflect.Value, want.NumOut())
		for i := range results {
			if i < len(got) {
				results[i] = coerce(got[i], want.Out(i))
			} else {
				results[i] = reflect.Zero(want.Out(i))
			}
		}
		return results
	})
}
