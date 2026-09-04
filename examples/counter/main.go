// Command counter ports the two "Motivation" samples from the redux-thunk
// README: an action creator that returns a function to perform asynchronous
// dispatch, and one that returns a function to perform conditional dispatch.
// It also ports the README's "Manual Setup" store wiring.
package main

import (
	"fmt"
	"time"

	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/async"
	"github.com/reduxjs/redux-thunk-go/redux"
)

// incrementCounter ports `const INCREMENT_COUNTER = 'INCREMENT_COUNTER'`.
const incrementCounter = "INCREMENT_COUNTER"

// State ports the implicit store shape `{ counter }` that incrementIfOdd reads.
type State struct {
	Counter int
}

// IncrementAction ports `{ type: INCREMENT_COUNTER }`.
type IncrementAction struct{}

// ActionType satisfies redux.Action.
func (IncrementAction) ActionType() string { return incrementCounter }

// increment ports:
//
//	function increment() {
//	  return {
//	    type: INCREMENT_COUNTER,
//	  }
//	}
func increment() IncrementAction {
	return IncrementAction{}
}

// rootReducer is the `rootReducer` imported from './reducers/index' in the
// README's manual-setup sample, which the README itself never shows.
func rootReducer(state State, action any) State {
	if _, ok := action.(IncrementAction); ok {
		return State{Counter: state.Counter + 1}
	}
	return state
}

// incrementAsync ports:
//
//	function incrementAsync() {
//	  return dispatch => {
//	    setTimeout(() => {
//	      // Yay! Can invoke sync or async actions with `dispatch`
//	      dispatch(increment())
//	    }, 1000)
//	  }
//	}
//
// The JavaScript thunk returns undefined, so its caller has no way to observe
// completion. The port returns a Future instead, which is the convention the
// README's own "Composition" section recommends and which makes the example
// testable: Go has no fake-timer facility to fall back on.
func incrementAsync(delay time.Duration) thunk.ThunkAction[*async.Future[any], State, any] {
	return func(dispatch redux.Dispatch, _ func() State, _ any) *async.Future[any] {
		return async.New(func() (any, error) {
			time.Sleep(delay)
			return dispatch(increment()), nil
		})
	}
}

// incrementIfOdd ports:
//
//	function incrementIfOdd() {
//	  return (dispatch, getState) => {
//	    const { counter } = getState()
//
//	    if (counter % 2 === 0) {
//	      return
//	    }
//
//	    dispatch(increment())
//	  }
//	}
func incrementIfOdd() thunk.ThunkAction[any, State, any] {
	return func(dispatch redux.Dispatch, getState func() State, _ any) any {
		counter := getState().Counter

		if counter%2 == 0 {
			return nil
		}

		dispatch(increment())
		return nil
	}
}

// run ports the README's manual setup:
//
//	const store = createStore(rootReducer, applyMiddleware(thunk))
func run() {
	store := redux.CreateStore(rootReducer,
		redux.WithEnhancer(redux.ApplyMiddleware(
			thunk.TypedThunk[State]().AsMiddleware(),
		)))

	store.Dispatch(increment())
	fmt.Println("increment:", store.GetState().Counter)

	store.Dispatch(incrementIfOdd())
	fmt.Println("incrementIfOdd on odd counter:", store.GetState().Counter)

	store.Dispatch(incrementIfOdd())
	fmt.Println("incrementIfOdd on even counter:", store.GetState().Counter)

	if _, err := thunk.DispatchThunk(store.Dispatch, incrementAsync(time.Millisecond)).Await(); err != nil {
		fmt.Println("incrementAsync failed:", err)
		return
	}
	fmt.Println("incrementAsync:", store.GetState().Counter)
}

func main() {
	run()
}
