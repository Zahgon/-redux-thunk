// Command extraargument ports the README's "Injecting a Custom Argument"
// section: both the single-value form and the "combine them into a single
// object" form, wired up with the named export `withExtraArgument()`.
package main

import (
	"errors"
	"fmt"

	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/async"
	"github.com/reduxjs/redux-thunk-go/redux"
)

// User is the value the injected service layer returns.
type User struct {
	ID   int
	Name string
}

// APIService ports `myCustomApiService`. The README's stated reason for the
// extra argument is "using an API service layer that could be swapped out for a
// mock service in tests", which in Go is spelled as an interface.
type APIService interface {
	FetchUser(id int) (User, error)
}

type liveAPI struct{}

func (liveAPI) FetchUser(id int) (User, error) {
	if id <= 0 {
		return User{}, errors.New("no such user")
	}
	return User{ID: id, Name: fmt.Sprintf("user-%d", id)}, nil
}

// State holds the user loaded by the thunks below.
type State struct {
	User User
}

// UserLoaded is dispatched once the injected service has answered.
type UserLoaded struct {
	User User
}

// ActionType satisfies redux.Action.
func (UserLoaded) ActionType() string { return "USER_LOADED" }

func rootReducer(state State, action any) State {
	if loaded, ok := action.(UserLoaded); ok {
		return State{User: loaded.User}
	}
	return state
}

// fetchUser ports:
//
//	function fetchUser(id) {
//	  // The `extraArgument` is the third arg for thunk functions
//	  return (dispatch, getState, api) => {
//	    // you can use api here
//	  }
//	}
func fetchUser(id int) thunk.ThunkAction[*async.Future[User], State, APIService] {
	return func(dispatch redux.Dispatch, _ func() State, api APIService) *async.Future[User] {
		return async.New(func() (User, error) {
			user, err := api.FetchUser(id)
			if err != nil {
				return User{}, err
			}
			dispatch(UserLoaded{User: user})
			return user, nil
		})
	}
}

// Extra ports the README's advice for passing more than one value:
//
//	extraArgument: {
//	  api: myCustomApiService,
//	  otherValue: 42,
//	}
type Extra struct {
	API        APIService
	OtherValue int
}

// fetchUserWithOtherValue ports:
//
//	function fetchUser(id) {
//	  return (dispatch, getState, { api, otherValue }) => {
//	    // you can use api and something else here
//	  }
//	}
//
// Go has no destructuring, so the struct is taken whole. It is taken by pointer
// because JavaScript shares the extra argument by reference: every thunk sees
// the same object, and a mutation by one is visible to the next. A struct value
// would be copied into each call and quietly break that contract.
func fetchUserWithOtherValue(id int) thunk.ThunkAction[*async.Future[User], State, *Extra] {
	return func(dispatch redux.Dispatch, _ func() State, extra *Extra) *async.Future[User] {
		return async.New(func() (User, error) {
			user, err := extra.API.FetchUser(id + extra.OtherValue)
			if err != nil {
				return User{}, err
			}
			dispatch(UserLoaded{User: user})
			return user, nil
		})
	}
}

// runSingleArgument ports:
//
//	const store = createStore(reducer, applyMiddleware(withExtraArgument(api)))
func runSingleArgument() {
	store := redux.CreateStore(rootReducer,
		redux.WithEnhancer(redux.ApplyMiddleware(
			thunk.WithExtraArgument[State, APIService](liveAPI{}).AsMiddleware(),
		)))

	user, err := thunk.DispatchThunk(store.Dispatch, fetchUser(7)).Await()
	if err != nil {
		fmt.Println("fetchUser failed:", err)
		return
	}
	fmt.Println("fetched:", user.Name)
	fmt.Println("state:", store.GetState().User.Name)

	if _, err := thunk.DispatchThunk(store.Dispatch, fetchUser(-1)).Await(); err != nil {
		fmt.Println("fetchUser failed:", err)
	}
}

// runCombinedArgument wires the same store with the combined-object extra.
func runCombinedArgument() {
	store := redux.CreateStore(rootReducer,
		redux.WithEnhancer(redux.ApplyMiddleware(
			thunk.WithExtraArgument[State](&Extra{API: liveAPI{}, OtherValue: 42}).AsMiddleware(),
		)))

	user, err := thunk.DispatchThunk(store.Dispatch, fetchUserWithOtherValue(0)).Await()
	if err != nil {
		fmt.Println("fetchUser failed:", err)
		return
	}
	fmt.Println("fetched with otherValue:", user.Name)
}

func main() {
	runSingleArgument()
	runCombinedArgument()
}
