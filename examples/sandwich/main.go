// Command sandwich ports the README's "Composition" section in full: the
// sandwich-shop promise-composition sample, the server-side-rendering wait at
// its end, and the React `connect` component that follows it.
package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/async"
	"github.com/reduxjs/redux-thunk-go/redux"
)

// SandwichesState ports the `sandwiches` slice of the store. The README reads
// it in two shapes -- `getState().sandwiches.isShopOpen` and
// `state.sandwiches.join('mustard')` -- because it is illustrative pseudo-code.
// Go needs one concrete shape, so both readings live on one struct.
type SandwichesState struct {
	IsShopOpen bool
	Made       []string
}

// State is the root state of the sandwich shop store.
type State struct {
	Sandwiches SandwichesState
	MyMoney    int
}

// MakeSandwichAction ports `{ type: 'MAKE_SANDWICH', forPerson, secretSauce }`.
type MakeSandwichAction struct {
	ForPerson   string
	SecretSauce string
}

// ActionType satisfies redux.Action.
func (MakeSandwichAction) ActionType() string { return "MAKE_SANDWICH" }

// ApologizeAction ports `{ type: 'APOLOGIZE', fromPerson, toPerson, error }`.
type ApologizeAction struct {
	FromPerson string
	ToPerson   string
	Err        error
}

// ActionType satisfies redux.Action.
func (ApologizeAction) ActionType() string { return "APOLOGIZE" }

// WithdrawAction ports `{ type: 'WITHDRAW', amount }`.
type WithdrawAction struct {
	Amount int
}

// ActionType satisfies redux.Action.
func (WithdrawAction) ActionType() string { return "WITHDRAW" }

// makeASandwich, apologize and withdrawMoney port the README's remark that
// "These are the normal action creators you have seen so far. The actions they
// return can be dispatched without any middleware."
func makeASandwich(forPerson, secretSauce string) MakeSandwichAction {
	return MakeSandwichAction{ForPerson: forPerson, SecretSauce: secretSauce}
}

func apologize(fromPerson, toPerson string, err error) ApologizeAction {
	return ApologizeAction{FromPerson: fromPerson, ToPerson: toPerson, Err: err}
}

func withdrawMoney(amount int) WithdrawAction {
	return WithdrawAction{Amount: amount}
}

func rootReducer(state State, action any) State {
	switch typed := action.(type) {
	case MakeSandwichAction:
		made := append(append([]string{}, state.Sandwiches.Made...), typed.ForPerson)
		state.Sandwiches.Made = made
	case WithdrawAction:
		state.MyMoney -= typed.Amount
	}
	return state
}

// fetchSecretSauce ports:
//
//	function fetchSecretSauce() {
//	  return fetch('https://www.google.com/search?q=secret+sauce')
//	}
//
// The example must stay hermetic, so the network call is simulated. The empty
// person name drives the rejection branch that the README's two-argument
// `.then` exists to handle.
func fetchSecretSauce(forPerson string) *async.Future[string] {
	if forPerson == "" {
		return async.Rejected[string](errors.New("the sauce is a secret"))
	}
	return async.Resolved("secret sauce")
}

// makeASandwichWithSecretSauce ports:
//
//	function makeASandwichWithSecretSauce(forPerson) {
//	  return function (dispatch) {
//	    return fetchSecretSauce().then(
//	      sauce => dispatch(makeASandwich(forPerson, sauce)),
//	      error => dispatch(apologize('The Sandwich Shop', forPerson, error)),
//	    )
//	  }
//	}
//
// async.Then takes only a fulfilment callback, so the two-argument `.then` is
// spelled as an explicit Await plus an error branch. The control flow -- and
// the fact that the rejection handler recovers rather than propagates -- is
// unchanged.
func makeASandwichWithSecretSauce(forPerson string) thunk.ThunkAction[*async.Future[any], State, any] {
	return func(dispatch redux.Dispatch, _ func() State, _ any) *async.Future[any] {
		return async.New(func() (any, error) {
			sauce, err := fetchSecretSauce(forPerson).Await()
			if err != nil {
				return dispatch(apologize("The Sandwich Shop", forPerson, err)), nil
			}
			return dispatch(makeASandwich(forPerson, sauce)), nil
		})
	}
}

// dispatchSandwich is the Go spelling of
// `dispatch(makeASandwichWithSecretSauce(person))`. In TypeScript the injected
// dispatch is a ThunkDispatch, whose first overload already returns the thunk's
// Promise; Go's dispatch is type-erased, so DispatchThunk recovers the type.
func dispatchSandwich(dispatch redux.Dispatch, forPerson string) *async.Future[any] {
	return thunk.DispatchThunk(dispatch, makeASandwichWithSecretSauce(forPerson))
}

// makeSandwichesForEverybody ports:
//
//	function makeSandwichesForEverybody() {
//	  return function (dispatch, getState) {
//	    if (!getState().sandwiches.isShopOpen) {
//	      return Promise.resolve()
//	    }
//	    return dispatch(makeASandwichWithSecretSauce('My Grandma'))
//	      .then(() => Promise.all([
//	        dispatch(makeASandwichWithSecretSauce('Me')),
//	        dispatch(makeASandwichWithSecretSauce('My wife')),
//	      ]))
//	      .then(() => dispatch(makeASandwichWithSecretSauce('Our kids')))
//	      .then(() => dispatch(
//	        getState().myMoney > 42
//	          ? withdrawMoney(42)
//	          : apologize('Me', 'The Sandwich Shop'),
//	      ))
//	  }
//	}
//
// A chain of `.then` calls becomes sequential Awaits inside one Future, which
// is the same ordering guarantee expressed with Go control flow.
func makeSandwichesForEverybody() thunk.ThunkAction[*async.Future[any], State, any] {
	return func(dispatch redux.Dispatch, getState func() State, _ any) *async.Future[any] {
		if !getState().Sandwiches.IsShopOpen {
			return async.Resolved[any](nil)
		}

		return async.New(func() (any, error) {
			if _, err := dispatchSandwich(dispatch, "My Grandma").Await(); err != nil {
				return nil, err
			}

			both := async.All(
				dispatchSandwich(dispatch, "Me"),
				dispatchSandwich(dispatch, "My wife"),
			)
			if _, err := both.Await(); err != nil {
				return nil, err
			}

			if _, err := dispatchSandwich(dispatch, "Our kids").Await(); err != nil {
				return nil, err
			}

			if getState().MyMoney > 42 {
				return dispatch(withdrawMoney(42)), nil
			}
			return dispatch(apologize("Me", "The Sandwich Shop", nil)), nil
		})
	}
}

// renderToString stands in for `ReactDOMServer.renderToString(<MyApp />)` and
// for the component's `render()`, both of which the README writes as
// `this.props.sandwiches.join('mustard')`.
func renderToString(sandwiches []string) string {
	return "<p>" + strings.Join(sandwiches, "mustard") + "</p>"
}

// SandwichShop ports the React class component at the end of the README. Go has
// no React, so the component is reduced to the contract the sample actually
// demonstrates: a view that dispatches a thunk when it mounts, dispatches again
// whenever its forPerson prop changes, and renders its sandwiches prop.
type SandwichShop struct {
	Dispatch   redux.Dispatch
	ForPerson  string
	Sandwiches []string
}

// ComponentDidMount ports:
//
//	componentDidMount() {
//	  this.props.dispatch(makeASandwichWithSecretSauce(this.props.forPerson))
//	}
func (s *SandwichShop) ComponentDidMount() {
	dispatchSandwich(s.Dispatch, s.ForPerson).Await()
}

// ComponentDidUpdate ports:
//
//	componentDidUpdate(prevProps) {
//	  if (prevProps.forPerson !== this.props.forPerson) {
//	    this.props.dispatch(makeASandwichWithSecretSauce(this.props.forPerson))
//	  }
//	}
func (s *SandwichShop) ComponentDidUpdate(prevForPerson string) {
	if prevForPerson != s.ForPerson {
		dispatchSandwich(s.Dispatch, s.ForPerson).Await()
	}
}

// Render ports `render() { return <p>{this.props.sandwiches.join('mustard')}</p> }`.
func (s *SandwichShop) Render() string {
	return renderToString(s.Sandwiches)
}

// Connect ports:
//
//	export default connect(state => ({
//	  sandwiches: state.sandwiches,
//	}))(SandwichShop)
//
// react-redux subscribes to the store and re-maps state to props on every
// change; that subscription is the whole of what the sample relies on.
func Connect(store redux.Store[State], forPerson string) *SandwichShop {
	shop := &SandwichShop{Dispatch: store.Dispatch, ForPerson: forPerson}
	mapStateToProps := func() { shop.Sandwiches = store.GetState().Sandwiches.Made }
	store.Subscribe(mapStateToProps)
	mapStateToProps()
	return shop
}

func newStore() redux.Store[State] {
	return redux.CreateStore(rootReducer,
		redux.WithPreloadedState(State{
			Sandwiches: SandwichesState{IsShopOpen: true},
			MyMoney:    100,
		}),
		redux.WithEnhancer(redux.ApplyMiddleware(
			thunk.TypedThunk[State]().AsMiddleware(),
		)))
}

// sortedSandwiches makes the output deterministic. Promise.all resolves its
// inputs on JavaScript's single thread, so 'Me' and 'My wife' are always made in
// argument order; async.All genuinely runs them on separate goroutines.
func sortedSandwiches(state State) []string {
	made := append([]string{}, state.Sandwiches.Made...)
	sort.Strings(made)
	return made
}

func run() {
	store := newStore()

	// Even without middleware, you can dispatch an action:
	store.Dispatch(withdrawMoney(100))
	fmt.Println("money after withdrawal:", store.GetState().MyMoney)

	// Thunk middleware lets me dispatch thunk async actions as if they were
	// actions, and it returns the thunk's own value from the dispatch.
	dispatchSandwich(store.Dispatch, "Me").Await()
	fmt.Println("sandwiches:", store.GetState().Sandwiches.Made)

	dispatchSandwich(store.Dispatch, "My partner").Await()
	fmt.Println("Done!")

	// The rejection branch of the two-argument `.then`.
	dispatchSandwich(store.Dispatch, "").Await()
	fmt.Println("sandwiches after a failed sauce fetch:", store.GetState().Sandwiches.Made)

	shop := Connect(store, "Kid")
	fmt.Println("component render:", shop.Render())
	shop.ComponentDidMount()
	fmt.Println("component render after mount:", shop.Render())
	shop.ForPerson = "Kid's friend"
	shop.ComponentDidUpdate("Kid")
	fmt.Println("component render after update:", shop.Render())

	everybody := newStore()

	// Server-side rendering: wait until the data is available, then render.
	thunk.DispatchThunk(everybody.Dispatch, makeSandwichesForEverybody()).Await()
	fmt.Println("everybody:", sortedSandwiches(everybody.GetState()))
	fmt.Println("money after the shop run:", everybody.GetState().MyMoney)
	fmt.Println("rendered:", renderToString(sortedSandwiches(everybody.GetState())))

	closed := redux.CreateStore(rootReducer,
		redux.WithEnhancer(redux.ApplyMiddleware(
			thunk.TypedThunk[State]().AsMiddleware(),
		)))
	thunk.DispatchThunk(closed.Dispatch, makeSandwichesForEverybody()).Await()
	fmt.Println("sandwiches made while the shop was closed:", len(closed.GetState().Sandwiches.Made))
}

func main() {
	run()
}
