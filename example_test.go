// The remaining README samples that are too small to deserve their own program
// under examples/. The larger ones live there: examples/counter (Motivation),
// examples/sandwich (Composition), examples/extraargument (Injecting a Custom
// Argument) and examples/consumer (Redux Toolkit setup).
package thunk_test

import (
	"fmt"

	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/redux"
)

// exampleState is the state shape shared by the samples below.
type exampleState struct {
	Counter int
}

// exampleBump is the one action they dispatch.
type exampleBump struct{}

// ActionType satisfies redux.Action.
func (exampleBump) ActionType() string { return "BUMP" }

func exampleReducer(state exampleState, action any) exampleState {
	if _, ok := action.(exampleBump); ok {
		return exampleState{Counter: state.Counter + 1}
	}
	return state
}

// exampleUntypedReducer is exampleReducer behind redux's `any` state, which is
// what TypeScript's `State = any` default gives the untyped `thunk` singleton.
func exampleUntypedReducer(state any, action any) any {
	current, _ := state.(exampleState)
	return exampleReducer(current, action)
}

// ExampleThunk ports the README's manual setup:
//
//	import { createStore, applyMiddleware } from 'redux'
//	import { thunk } from 'redux-thunk'
//	import rootReducer from './reducers/index'
//
//	const store = createStore(rootReducer, applyMiddleware(thunk))
//
// The named export becomes a package-level variable, so the two import forms
// the README distinguishes -- ES module and CommonJS -- collapse into one.
func ExampleThunk() {
	store := redux.CreateStore[any](exampleUntypedReducer,
		redux.WithPreloadedState[any](exampleState{}),
		redux.WithEnhancer(redux.ApplyMiddleware(
			thunk.Thunk.AsMiddleware(),
		)))

	store.Dispatch(thunk.ThunkActionAny(
		func(dispatch redux.Dispatch, getState func() any, _ any) any {
			dispatch(exampleBump{})
			return getState()
		}))

	fmt.Println(store.GetState())
	// Output: {1}
}

// ExampleTypedThunk shows the same wiring against a store whose state type is
// known, which is what every sample in this port's own README uses.
func ExampleTypedThunk() {
	store := redux.CreateStore(exampleReducer,
		redux.WithEnhancer(redux.ApplyMiddleware(
			thunk.TypedThunk[exampleState]().AsMiddleware(),
		)))

	counter := thunk.DispatchThunk(store.Dispatch,
		thunk.ThunkAction[int, exampleState, any](
			func(dispatch redux.Dispatch, getState func() exampleState, _ any) int {
				dispatch(exampleBump{})
				return getState().Counter
			}))

	fmt.Println(counter)
	// Output: 1
}

// ExampleWithExtraArgument ports:
//
//	const store = createStore(reducer, applyMiddleware(withExtraArgument(api)))
func ExampleWithExtraArgument() {
	store := redux.CreateStore(exampleReducer,
		redux.WithEnhancer(redux.ApplyMiddleware(
			thunk.WithExtraArgument[exampleState]("the api service").AsMiddleware(),
		)))

	api := thunk.DispatchThunk(store.Dispatch,
		thunk.ThunkAction[string, exampleState, string](
			func(_ redux.Dispatch, _ func() exampleState, api string) string {
				return api
			}))

	fmt.Println(api)
	// Output: the api service
}

// ExampleDispatchEither shows the third ThunkDispatch overload: a value that is
// statically either a plain action or a thunk, dispatched without narrowing.
func ExampleDispatchEither() {
	store := redux.CreateStore(exampleReducer,
		redux.WithEnhancer(redux.ApplyMiddleware(
			thunk.TypedThunk[exampleState]().AsMiddleware(),
		)))

	actions := []any{
		exampleBump{},
		thunk.ThunkAction[any, exampleState, any](
			func(dispatch redux.Dispatch, _ func() exampleState, _ any) any {
				return dispatch(exampleBump{})
			}),
	}

	for _, action := range actions {
		thunk.DispatchEither(store.Dispatch, action)
	}

	fmt.Println(store.GetState().Counter)
	// Output: 2
}

// Example_whatsAThunk ports the README's definition of the word:
//
//	// calculation of 1 + 2 is immediate
//	// x === 3
//	let x = 1 + 2
//
//	// calculation of 1 + 2 is delayed
//	// foo can be called later to perform the calculation
//	// foo is a thunk!
//	let foo = () => 1 + 2
func Example_whatsAThunk() {
	x := 1 + 2

	foo := func() int { return 1 + 2 }

	fmt.Println(x, foo())
	// Output: 3 3
}
