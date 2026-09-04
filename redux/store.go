package redux

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// Internal action types dispatched by the store itself. Redux suffixes these
// with a random string so that user reducers cannot accidentally match on them;
// the same trick is reproduced here.
var (
	actionTypeInit    = "@@redux/INIT" + randomSuffix()
	actionTypeReplace = "@@redux/REPLACE" + randomSuffix()
)

func randomSuffix() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is not recoverable and not worth surfacing for a
		// cosmetic suffix; fall back to a fixed value.
		return ".0.0.0.0"
	}
	return "." + hex.EncodeToString(b[:])
}

// Config holds the optional arguments of CreateStore. It exists because Go has
// no function overloading, whereas redux's `createStore` accepts
// `(reducer)`, `(reducer, preloadedState)`, `(reducer, enhancer)` and
// `(reducer, preloadedState, enhancer)`.
type Config[S any] struct {
	preloadedState S
	enhancer       Enhancer[S]
	// enhancerSet distinguishes "no enhancer supplied" from "an enhancer was
	// supplied and it was nil". Redux ignores the first and throws on the
	// second; without this flag both look identical in Go.
	enhancerSet bool
}

// Option configures CreateStore.
type Option[S any] func(*Config[S])

// WithPreloadedState supplies the store's initial state, matching the
// `preloadedState` argument of `createStore`.
func WithPreloadedState[S any](state S) Option[S] {
	return func(c *Config[S]) { c.preloadedState = state }
}

// WithEnhancer supplies a store enhancer, matching the `enhancer` argument of
// `createStore`. ApplyMiddleware produces one.
func WithEnhancer[S any](enhancer Enhancer[S]) Option[S] {
	return func(c *Config[S]) {
		c.enhancer = enhancer
		c.enhancerSet = true
	}
}

// CreateStore is the Go analogue of redux's `createStore`.
//
//	const store = createStore(rootReducer, applyMiddleware(thunk))
//
// becomes
//
//	store := redux.CreateStore(rootReducer,
//	    redux.WithEnhancer(redux.ApplyMiddleware(thunk.TypedThunk[State]().AsMiddleware())))
//
// The reducer is invoked once during construction with an internal init action
// so that the store holds the reducer's initial state, exactly as Redux does.
func CreateStore[S any](reducer Reducer[S], opts ...Option[S]) Store[S] {
	if reducer == nil {
		panic(&Error{Message: "Expected the root reducer to be a function. Instead, received: 'undefined'"})
	}

	var cfg Config[S]
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	base := StoreCreator[S](func(r Reducer[S], preloaded S) Store[S] {
		return newStore(r, preloaded)
	})

	if cfg.enhancerSet && cfg.enhancer == nil {
		panic(&Error{Message: "Expected the enhancer to be a function. Instead, received: 'null'"})
	}
	if cfg.enhancer != nil {
		return cfg.enhancer(base)(reducer, cfg.preloadedState)
	}
	return base(reducer, cfg.preloadedState)
}

// Error is the error type panicked by this package. It mirrors the JavaScript
// `Error` objects thrown by Redux, preserving their messages verbatim.
type Error struct {
	Message string
}

func (e *Error) Error() string { return e.Message }

type store[S any] struct {
	// dispatchMu serialises whole dispatches. JavaScript gets this for free
	// from the event loop; without it two goroutines could interleave reducer
	// runs and lose an update.
	dispatchMu sync.Mutex

	mu      sync.Mutex
	reducer Reducer[S]
	state   S
	// listeners is a slice, not a map, because Redux stores them in a JavaScript
	// Map and notifies in insertion order. Ranging over a Go map would randomise
	// that order, breaking listeners that depend on registration sequence.
	listeners     []listenerEntry
	nextListenerI int
	// reducerG is the goroutine currently inside the reducer, or 0. See goID.
	reducerG uint64
}

type listenerEntry struct {
	id int
	fn func()
}

// inReducer reports whether the calling goroutine is the one currently running
// the reducer, which is the precise condition Redux's guards mean to catch.
func (s *store[S]) inReducer() bool {
	s.mu.Lock()
	running := s.reducerG
	s.mu.Unlock()
	// goID is only reached when a reducer really is running somewhere, which
	// keeps its cost off the uncontended path. See goroutine.go.
	return running != 0 && running == goID()
}

func newStore[S any](reducer Reducer[S], preloadedState S) *store[S] {
	s := &store[S]{
		reducer: reducer,
		state:   preloadedState,
	}
	// Redux dispatches an INIT action so that every reducer returns its
	// initial state.
	s.Dispatch(AnyAction{TypeKey: actionTypeInit})
	return s
}

// GetState returns the current state tree.
//
// Redux forbids this while the reducer is executing, and so does this port. The
// guard does not interfere with thunks: Redux's `isDispatching` is true only
// for the duration of the reducer call, and the middleware chain — where thunks
// run and legitimately call getState — executes before the reducer is reached.
func (s *store[S]) GetState() S {
	if s.inReducer() {
		panic(&Error{Message: "You may not call store.getState() while the reducer is executing. " +
			"The reducer has already received the state as an argument. " +
			"Pass it down from the top reducer instead of reading it from the store."})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Dispatch runs the action through the reducer and notifies subscribers.
func (s *store[S]) Dispatch(action any) any {
	assertPlainAction(action)

	// The goroutine identity is read at most once per dispatch and reused for
	// both the reentrancy check and the reducer marker, because obtaining it is
	// the most expensive step on this path. See goroutine.go.
	self := goID()

	s.mu.Lock()
	running := s.reducerG
	s.mu.Unlock()

	if running != 0 && running == self {
		panic(&Error{Message: "Reducers may not dispatch actions."})
	}

	s.dispatchMu.Lock()

	s.mu.Lock()
	s.reducerG = self
	reducer := s.reducer
	prev := s.state
	s.mu.Unlock()

	next, err := runReducer(reducer, prev, action)

	s.mu.Lock()
	s.reducerG = 0
	if err == nil {
		s.state = next
	}
	listeners := make([]func(), 0, len(s.listeners))
	for _, l := range s.listeners {
		listeners = append(listeners, l.fn)
	}
	s.mu.Unlock()

	// Released before listeners run so that a listener may dispatch, as Redux
	// permits.
	s.dispatchMu.Unlock()

	if err != nil {
		panic(err)
	}

	// Listeners run outside the lock so that a listener may safely call
	// GetState or Dispatch.
	for _, l := range listeners {
		l()
	}
	return action
}

// runReducer isolates reducer execution so that a panicking reducer cannot
// leave isDispatching stuck true, matching the try/finally in Redux's dispatch.
func runReducer[S any](reducer Reducer[S], prev S, action any) (next S, recovered any) {
	defer func() {
		if r := recover(); r != nil {
			next = prev
			recovered = r
		}
	}()
	return reducer(prev, action), nil
}

// Subscribe registers a change listener. The returned function removes it and
// is safe to call more than once.
func (s *store[S]) Subscribe(listener func()) func() {
	if listener == nil {
		panic(&Error{Message: "Expected the listener to be a function. Instead, received: 'undefined'"})
	}
	if s.inReducer() {
		panic(&Error{Message: "You may not call store.subscribe() while the reducer is executing. " +
			"If you would like to be notified after the store has been updated, subscribe from a " +
			"component and invoke store.getState() in the callback to access the latest state. " +
			"See https://redux.js.org/api/store#subscribelistener for more details."})
	}
	s.mu.Lock()
	id := s.nextListenerI
	s.nextListenerI++
	s.listeners = append(s.listeners, listenerEntry{id: id, fn: listener})
	s.mu.Unlock()

	var once sync.Once
	return func() {
		if s.inReducer() {
			panic(&Error{Message: "You may not unsubscribe from a store listener while the reducer is executing. " +
				"See https://redux.js.org/api/store#subscribelistener for more details."})
		}
		once.Do(func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			for i, entry := range s.listeners {
				if entry.id == id {
					s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
					return
				}
			}
		})
	}
}

// ReplaceReducer swaps the store's reducer and re-initialises state.
func (s *store[S]) ReplaceReducer(reducer Reducer[S]) {
	if reducer == nil {
		// The missing closing quote is faithful to Redux 5.0.1, which builds this
		// message as `Expected the nextReducer to be a function. Instead, received: '${kindOf(nextReducer)}`.
		panic(&Error{Message: "Expected the nextReducer to be a function. Instead, received: 'undefined"})
	}
	s.mu.Lock()
	s.reducer = reducer
	s.mu.Unlock()
	s.Dispatch(AnyAction{TypeKey: actionTypeReplace})
}
