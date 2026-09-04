package thunk_test

import (
	"reflect"
	"testing"

	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/redux"
)

// This file is a 1:1 port of test/index.test.ts from redux-thunk v3.1.0.
// The describe/it nesting is reproduced with subtests and the assertion text is
// carried over verbatim so that a failure here names the same case as upstream.

// Module-level fixtures, matching the original:
//
//	const doDispatch = () => {}
//	const doGetState = () => 42
//	const nextHandler = thunkMiddleware({ dispatch: doDispatch, getState: doGetState })
//
// The middleware is instantiated at State = any because the JavaScript fixtures
// are untyped; that keeps doGetState's type `func() any`, exactly as stored.
var (
	doDispatch redux.Dispatch = func(action any) any { return nil }

	doGetState = func() any { return 42 }

	nextHandler = thunk.Thunk(&redux.MiddlewareAPI[any]{
		Dispatch: doDispatch,
		GetState: doGetState,
	})
)

// sameRef is the stand-in for Vitest's `expect(actual).toBe(expected)` on
// reference values. Go cannot compare function or map values with ==, so
// identity is checked through the underlying pointer. For functions this is the
// code pointer, which is sufficient here because each fixture is a distinct
// literal: if the middleware wrapped a fixture instead of forwarding it, the
// pointer would differ and the assertion would fail, which is the property the
// original test exists to protect.
func sameRef(a, b any) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	if !va.IsValid() || !vb.IsValid() {
		return false
	}
	return va.Pointer() == vb.Pointer()
}

// arity is the equivalent of reading `.length` on a JavaScript function.
func arity(fn any) int {
	t := reflect.TypeOf(fn)
	if t == nil || t.Kind() != reflect.Func {
		return -1
	}
	return t.NumIn()
}

func isFunction(v any) bool {
	t := reflect.TypeOf(v)
	return t != nil && t.Kind() == reflect.Func
}

func TestThunkMiddleware(t *testing.T) {
	t.Run("must return a function to handle next", func(t *testing.T) {
		if !isFunction(nextHandler) {
			t.Fatalf("expected nextHandler to be a Function, got %T", nextHandler)
		}
		if got := arity(nextHandler); got != 1 {
			t.Errorf("expected nextHandler.length to be 1, got %d", got)
		}
	})

	t.Run("handle next", func(t *testing.T) {
		t.Run("must return a function to handle action", func(t *testing.T) {
			// The original calls nextHandler() with no argument at all. The Go
			// equivalent of the resulting `undefined` next is a nil redux.Next.
			actionHandler := nextHandler(nil)

			if !isFunction(actionHandler) {
				t.Fatalf("expected actionHandler to be a Function, got %T", actionHandler)
			}
			if got := arity(actionHandler); got != 1 {
				t.Errorf("expected actionHandler.length to be 1, got %d", got)
			}
		})

		t.Run("handle action", func(t *testing.T) {
			t.Run("must run the given action function with dispatch and getState", func(t *testing.T) {
				actionHandler := nextHandler(nil)

				ran := false
				actionHandler(func(dispatch redux.Dispatch, getState func() any) {
					ran = true
					if !sameRef(dispatch, doDispatch) {
						t.Errorf("expected dispatch to be doDispatch")
					}
					if !sameRef(getState, doGetState) {
						t.Errorf("expected getState to be doGetState")
					}
				})

				if !ran {
					t.Error("expected the action function to have been run")
				}
			})

			t.Run("must pass action to next if not a function", func(t *testing.T) {
				actionObj := redux.AnyAction{}

				ran := false
				actionHandler := nextHandler(func(action any) any {
					ran = true
					if !sameRef(action, actionObj) {
						t.Errorf("expected action to be actionObj, got %#v", action)
					}
					return nil
				})
				actionHandler(actionObj)

				if !ran {
					t.Error("expected next to have been called")
				}
			})

			t.Run("must return the return value of next if not a function", func(t *testing.T) {
				expected := "redux"

				actionHandler := nextHandler(func(any) any { return expected })
				outcome := actionHandler(nil)

				if outcome != expected {
					t.Errorf("expected outcome to be %q, got %#v", expected, outcome)
				}
			})

			t.Run("must return value as expected if a function", func(t *testing.T) {
				expected := "rocks"

				actionHandler := nextHandler(nil)
				outcome := actionHandler(func() string { return expected })

				if outcome != expected {
					t.Errorf("expected outcome to be %q, got %#v", expected, outcome)
				}
			})

			t.Run("must be invoked synchronously if a function", func(t *testing.T) {
				actionHandler := nextHandler(nil)

				mutated := 0
				actionHandler(func() { mutated++ })

				if mutated != 1 {
					t.Errorf("expected mutated to be 1, got %d", mutated)
				}
			})
		})
	})

	t.Run("handle errors", func(t *testing.T) {
		t.Run("must throw if argument is non-object", func(t *testing.T) {
			// The original calls thunkMiddleware() with no argument, so the
			// middleware destructures `undefined` and throws a TypeError. A nil
			// *MiddlewareAPI is the Go equivalent of that missing argument;
			// a pointer to a zero struct would be `thunk({})`, which JavaScript
			// accepts.
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("expected a panic when the middleware argument is missing")
				}
				if _, ok := r.(*thunk.TypeError); !ok {
					t.Errorf("expected a *thunk.TypeError, got %T: %v", r, r)
				}
			}()

			thunk.Thunk(nil)
		})
	})

	t.Run("withExtraArgument", func(t *testing.T) {
		t.Run("must pass the third argument", func(t *testing.T) {
			type extra struct{ Lol bool }
			extraArg := &extra{Lol: true}

			ran := false
			thunk.WithExtraArgument[any](extraArg)(&redux.MiddlewareAPI[any]{
				Dispatch: doDispatch,
				GetState: doGetState,
			})(nil)(func(dispatch redux.Dispatch, getState func() any, arg *extra) {
				ran = true
				if !sameRef(dispatch, doDispatch) {
					t.Errorf("expected dispatch to be doDispatch")
				}
				if !sameRef(getState, doGetState) {
					t.Errorf("expected getState to be doGetState")
				}
				if arg != extraArg {
					t.Errorf("expected arg to be extraArg, got %#v", arg)
				}
			})

			if !ran {
				t.Error("expected the action function to have been run")
			}
		})
	})
}
