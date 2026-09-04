# Redux Thunk (Go)

Thunk [middleware](https://redux.js.org/tutorials/fundamentals/part-4-store#middleware) for Redux. It allows writing functions with logic inside that can interact with a Redux store's `Dispatch` and `GetState` methods.

This is a complete Go port of [redux-thunk](https://github.com/reduxjs/redux-thunk) v3.1.0. Every runtime behaviour of the original is reproduced; the type-level API is adapted where Go and TypeScript genuinely differ. See [`docs/MIGRATION.md`](docs/MIGRATION.md) for the full divergence table.

For the concepts, the original [Redux docs **Writing Logic with Thunks** page](https://redux.js.org/usage/writing-logic-thunks) still applies verbatim.

## Installation and Setup

```sh
go get github.com/reduxjs/redux-thunk-go
```

The original declares `redux` as a peer dependency. Go has no such concept and there is no canonical Redux port for Go, so this module ships a minimal `redux` subpackage containing exactly the pieces redux-thunk's README examples need: `CreateStore`, `ApplyMiddleware`, `Compose`, `BindActionCreators` and the `Action` / `Dispatch` / `Middleware` / `Store` types.

### Store setup

The thunk middleware is a package-level variable, the direct counterpart of the original's named export:

```go
import (
	thunk "github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/redux"
)

store := redux.CreateStore(rootReducer,
	redux.WithEnhancer(redux.ApplyMiddleware(
		thunk.TypedThunk[State]().AsMiddleware(),
	)))
```

`thunk.Thunk` is the untyped singleton, matching `State = any` in the original. `thunk.TypedThunk[State]()` is the Go analogue of the TypeScript cast `thunk as ThunkMiddleware<State, Actions>` — Go's function types are nominal, so the specialised middleware must be constructed rather than re-typed.

`AsMiddleware()` converts a `ThunkMiddleware[S, E]` to a `redux.Middleware[S]`. TypeScript's structural typing makes the two interchangeable; Go needs the explicit seam.

### Injecting a Custom Argument

Redux Thunk supports injecting a custom argument into the thunk middleware. This is typically useful for cases like using an API service layer that could be swapped out for a mock service in tests.

```go
store := redux.CreateStore(rootReducer,
	redux.WithEnhancer(redux.ApplyMiddleware(
		thunk.WithExtraArgument[State, APIService](myCustomAPIService).AsMiddleware(),
	)))

// later
func fetchUser(id int) thunk.ThunkAction[*async.Future[User], State, APIService] {
	// The extra argument is the third parameter of every thunk function
	return func(dispatch redux.Dispatch, getState func() State, api APIService) *async.Future[User] {
		// you can use api here
	}
}
```

If you need to pass in multiple values, combine them into a single struct:

```go
type Extra struct {
	API        APIService
	OtherValue int
}

store := redux.CreateStore(rootReducer,
	redux.WithEnhancer(redux.ApplyMiddleware(
		thunk.WithExtraArgument[State](Extra{API: myCustomAPIService, OtherValue: 42}).AsMiddleware(),
	)))

// later
func fetchUser(id int) thunk.ThunkAction[*async.Future[User], State, Extra] {
	return func(dispatch redux.Dispatch, getState func() State, extra Extra) *async.Future[User] {
		// you can use extra.API and extra.OtherValue here
	}
}
```

Go has no destructuring, so the struct is taken whole instead of being unpacked in the parameter list. The runnable version is [`examples/extraargument`](examples/extraargument).

## Why Do I Need This?

With a plain basic Redux store, you can only do simple synchronous updates by dispatching an action. Middleware extends the store's abilities, and lets you write async logic that interacts with the store.

Thunks are the recommended middleware for basic Redux side effects logic, including complex synchronous logic that needs access to the store, and simple async logic like AJAX requests.

For more details on why thunks are useful, see:

- **Redux docs: Writing Logic with Thunks**
  https://redux.js.org/usage/writing-logic-thunks
- **Stack Overflow: Dispatching Redux Actions with a Timeout**
  http://stackoverflow.com/questions/35411423/how-to-dispatch-a-redux-action-with-a-timeout/35415559#35415559
- **Stack Overflow: Why do we need middleware for async flow in Redux?**
  http://stackoverflow.com/questions/34570758/why-do-we-need-middleware-for-async-flow-in-redux/34599594#34599594
- **What the heck is a "thunk"?**
  https://daveceddia.com/what-is-a-thunk/
- **Thunks in Redux: The Basics**
  https://medium.com/fullstack-academy/thunks-in-redux-the-basics-85e538a3fe60

You may also want to read the **[Redux FAQ entry on choosing which async middleware to use](https://redux.js.org/faq/actions#what-async-middleware-should-i-use-how-do-you-decide-between-thunks-sagas-observables-or-something-else)**.

## Motivation

Redux Thunk middleware allows you to write action creators that return a function instead of an action. The thunk can be used to delay the dispatch of an action, or to dispatch only if a certain condition is met. The inner function receives the store methods `dispatch` and `getState` as parameters.

An action creator that returns a function to perform asynchronous dispatch:

```go
const incrementCounter = "INCREMENT_COUNTER"

type IncrementAction struct{}

func (IncrementAction) ActionType() string { return incrementCounter }

func increment() IncrementAction {
	return IncrementAction{}
}

func incrementAsync(delay time.Duration) thunk.ThunkAction[*async.Future[any], State, any] {
	return func(dispatch redux.Dispatch, _ func() State, _ any) *async.Future[any] {
		return async.New(func() (any, error) {
			time.Sleep(delay)
			// Yay! Can invoke sync or async actions with dispatch
			return dispatch(increment()), nil
		})
	}
}
```

An action creator that returns a function to perform conditional dispatch:

```go
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
```

The runnable version is [`examples/counter`](examples/counter).

## What's a thunk?!

A [thunk](https://en.wikipedia.org/wiki/Thunk) is a function that wraps an expression to delay its evaluation.

```go
// calculation of 1 + 2 is immediate
// x == 3
x := 1 + 2

// calculation of 1 + 2 is delayed
// foo can be called later to perform the calculation
// foo is a thunk!
foo := func() int { return 1 + 2 }
```

The term [originated](https://en.wikipedia.org/wiki/Thunk#cite_note-1) as a humorous past-tense version of "think".

## Composition

Any return value from the inner function will be available as the return value of `Dispatch` itself. This is convenient for orchestrating an asynchronous control flow with thunk action creators dispatching each other and waiting for each other's completion:

```go
store := redux.CreateStore(rootReducer,
	redux.WithEnhancer(redux.ApplyMiddleware(
		thunk.TypedThunk[State]().AsMiddleware(),
	)))

// These are the normal action creators you have seen so far.
// The actions they return can be dispatched without any middleware.
// However, they only express "facts" and not the "async flow".

func makeASandwich(forPerson, secretSauce string) MakeSandwichAction { ... }
func apologize(fromPerson, toPerson string, err error) ApologizeAction { ... }
func withdrawMoney(amount int) WithdrawAction { ... }

// Even without middleware, you can dispatch an action:
store.Dispatch(withdrawMoney(100))

// A thunk in this context is a function that can be dispatched to perform async
// activity and can dispatch actions and read state.
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

// Thunk middleware lets me dispatch thunk async actions as if they were actions,
// and it returns the thunk's own value from the dispatch, so I can chain as long
// as I return it. DispatchThunk recovers the static return type from `any`.
future := thunk.DispatchThunk(store.Dispatch, makeASandwichWithSecretSauce("Me"))
if _, err := future.Await(); err == nil {
	fmt.Println("Done!")
}
```

`Promise` has no Go equivalent, so this module ships [`async.Future`](async), a one-shot, concurrency-safe container for a value that becomes available later:

| JavaScript          | Go             |
| ------------------- | -------------- |
| `new Promise(...)`  | `async.New`    |
| `Promise.resolve(v)`| `async.Resolved` |
| `Promise.reject(e)` | `async.Rejected` |
| `await p`           | `f.Await()`    |
| `p.then(fn)`        | `async.Then`   |
| `Promise.all([...])`| `async.All`    |

The two-argument `p.then(onFulfilled, onRejected)` becomes an explicit `Await` plus an error branch, as above.

The full sandwich-shop example — including the composed `makeSandwichesForEverybody`, the server-side-rendering wait, and the `connect`ed React component reduced to its Go essentials — is in [`examples/sandwich`](examples/sandwich).

## Dispatching thunks

TypeScript expresses `ThunkDispatch` as three overloads. Go has neither overloading nor union types, so the three are provided as three generic functions:

| TypeScript overload | Go |
| --- | --- |
| `<ReturnType>(thunkAction) => ReturnType` | `thunk.DispatchThunk` |
| `<Action extends BasicAction>(action) => Action` | `thunk.DispatchAction` |
| `(action \| thunkAction) => Action \| ReturnType` | `thunk.DispatchEither` |

`store.Dispatch` alone still works for every case at runtime — it just returns `any`. The helpers exist to recover the static type.

Similarly, `ThunkActionDispatch<ActionCreator>` — which binds an action creator to a dispatch — needs variadic generics that Go lacks, so it is an arity-indexed family: `thunk.Bind0` through `thunk.Bind3`, plus `thunk.BindPlain0` / `thunk.BindPlain1` for plain action creators.

## Writing thunks

A thunk is any function value. As in JavaScript, it may declare fewer parameters than are supplied, and a variadic parameter collects whatever the fixed ones did not take:

```go
store.Dispatch(func(dispatch redux.Dispatch, getState func() State, extra any) any { ... })
store.Dispatch(func(dispatch redux.Dispatch) any { ... })
store.Dispatch(func(rest ...any) any { return len(rest) }) // 3
```

Two rules have no JavaScript counterpart and are worth knowing:

- **Return at most one value.** A JavaScript function returns exactly one, and so does `dispatch`; a multi-result thunk is rejected rather than having its `error` silently dropped. Return a struct or an `*async.Future` to carry more.
- **Declare the store's state type.** A thunk written against `func() State` receives the store's own `getState` by identity. One written against `func() any` receives an equivalent adapter instead, because Go function types are nominal. Both are correct; only the first is free.

For a mutable extra argument, use a pointer or an interface — JavaScript shares the extra argument by reference, and a Go struct value would be copied into every call.

## Concurrency

JavaScript is single-threaded, so the original never had to define what happens when two dispatches overlap. Go does. This port's store serialises whole dispatches under a mutex and scopes Redux's reentrancy guards ("Reducers may not dispatch actions.", "You may not call store.subscribe() while the reducer is executing.") to the goroutine actually running the reducer, so they keep their original meaning instead of firing spuriously on concurrent traffic. Store listeners run outside the lock, so a listener may dispatch, exactly as Redux permits.

`concurrency_test.go` exercises this under `go test -race`.

## Development

```sh
make test    # go test -race ./...
make lint    # go vet + gofmt check
make cover   # coverage report
make check   # everything, including the external-consumer module build
```

`cmd/qcprobe` is the Go half of a differential harness: it prints the observable
behaviour of the middleware — handler arity, argument identity, return values,
and the error raised when the API is missing — in a form that can be compared
byte-for-byte against the original TypeScript running under Node. See
[`docs/MIGRATION.md`](docs/MIGRATION.md) for what it proves and how to run both
halves.

## License

MIT
