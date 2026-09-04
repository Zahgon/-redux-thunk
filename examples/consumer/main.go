// Command consumer ports the README's first sample, the Redux Toolkit setup:
//
//	const store = configureStore({
//	  reducer: {
//	    todos: todosReducer,
//	    filters: filtersReducer,
//	  },
//	})
//
//	// The thunk middleware was automatically added
//
// It lives in its own module so that building it exercises the library exactly
// as an external consumer would.
package main

import (
	"fmt"
	"strings"

	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/async"
	"github.com/reduxjs/redux-thunk-go/redux"
)

// Todo is one entry of the todos slice.
type Todo struct {
	Text string
	Done bool
}

// Filters is the filters slice.
type Filters struct {
	ShowCompleted bool
}

// RootState ports the `{ todos, filters }` reducer map: Redux Toolkit builds
// the root state from the shape of that object, and Go declares it directly.
type RootState struct {
	Todos   []Todo
	Filters Filters
}

// TodoAdded is dispatched by the async thunk below.
type TodoAdded struct {
	Text string
}

// ActionType satisfies redux.Action.
func (TodoAdded) ActionType() string { return "todos/todoAdded" }

// ShowCompletedToggled is dispatched against the filters slice.
type ShowCompletedToggled struct{}

// ActionType satisfies redux.Action.
func (ShowCompletedToggled) ActionType() string { return "filters/showCompletedToggled" }

func todosReducer(state []Todo, action any) []Todo {
	if added, ok := action.(TodoAdded); ok {
		return append(append([]Todo{}, state...), Todo{Text: added.Text})
	}
	return state
}

func filtersReducer(state Filters, action any) Filters {
	if _, ok := action.(ShowCompletedToggled); ok {
		return Filters{ShowCompleted: !state.ShowCompleted}
	}
	return state
}

// rootReducer is the `combineReducers` that configureStore performs implicitly
// when it is handed a map of slice reducers.
func rootReducer(state RootState, action any) RootState {
	return RootState{
		Todos:   todosReducer(state.Todos, action),
		Filters: filtersReducer(state.Filters, action),
	}
}

// configureStore ports Redux Toolkit's store factory down to the single
// behaviour the README sample demonstrates: the thunk middleware is installed
// by default, so there is nothing for the caller to add.
func configureStore(reducer redux.Reducer[RootState]) redux.Store[RootState] {
	return redux.CreateStore(reducer,
		redux.WithEnhancer(redux.ApplyMiddleware(
			thunk.TypedThunk[RootState]().AsMiddleware(),
		)))
}

// addTodoAsync is the async thunk that proves the middleware really was added.
func addTodoAsync(text string) thunk.ThunkAction[*async.Future[int], RootState, any] {
	return func(dispatch redux.Dispatch, getState func() RootState, _ any) *async.Future[int] {
		return async.New(func() (int, error) {
			dispatch(TodoAdded{Text: text})
			return len(getState().Todos), nil
		})
	}
}

func run() {
	store := configureStore(rootReducer)

	count, err := thunk.DispatchThunk(store.Dispatch, addTodoAsync("write the port")).Await()
	if err != nil {
		fmt.Println("addTodoAsync failed:", err)
		return
	}
	fmt.Println("todo count:", count)

	store.Dispatch(ShowCompletedToggled{})

	texts := make([]string, 0, len(store.GetState().Todos))
	for _, todo := range store.GetState().Todos {
		texts = append(texts, todo.Text)
	}
	fmt.Println("todos:", strings.Join(texts, ", "))
	fmt.Println("showCompleted:", store.GetState().Filters.ShowCompleted)
}

func main() {
	run()
}
