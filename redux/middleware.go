package redux

import "sync/atomic"

// Compose is the Go analogue of redux's `compose`:
//
//	compose(f, g, h) === (...args) => f(g(h(...args)))
//
// It composes right-to-left, which is what gives a middleware chain its
// left-to-right execution order.
func Compose(fns ...func(Dispatch) Dispatch) func(Dispatch) Dispatch {
	if len(fns) == 0 {
		return func(d Dispatch) Dispatch { return d }
	}
	if len(fns) == 1 {
		return fns[0]
	}
	return func(d Dispatch) Dispatch {
		out := d
		for i := len(fns) - 1; i >= 0; i-- {
			out = fns[i](out)
		}
		return out
	}
}

// ApplyMiddleware is the Go analogue of redux's `applyMiddleware`.
//
// It reproduces the JavaScript implementation exactly, including two details
// that matter for thunk fidelity:
//
//  1. The MiddlewareAPI handed to each middleware carries a *forwarding*
//     dispatch closure, not the raw store dispatch. This is what lets a thunk
//     dispatch back through the whole middleware chain (including other
//     thunks) rather than straight into the reducer.
//
//  2. Calling that dispatch while the chain is still being constructed panics.
//     Redux throws the same error to catch middleware that dispatches during
//     setup.
//
// Middlewares run left to right, so ApplyMiddleware(a, b, c) produces the chain
// a -> b -> c -> store.dispatch.
func ApplyMiddleware[S any](middlewares ...Middleware[S]) Enhancer[S] {
	return func(createStore StoreCreator[S]) StoreCreator[S] {
		return func(reducer Reducer[S], preloadedState S) Store[S] {
			base := createStore(reducer, preloadedState)

			// JavaScript rebinds a plain `let dispatch` here. In Go that is a
			// data race: a middleware may hand the forwarding closure to another
			// goroutine, which would read the variable while construction is
			// still writing it. The atomic pointer preserves the late-binding
			// semantics without the race.
			var dispatch atomic.Pointer[Dispatch]
			constructing := Dispatch(func(action any) any {
				panic(&Error{Message: "Dispatching while constructing your middleware is not allowed. " +
					"Other middleware would not be applied to this dispatch."})
			})
			dispatch.Store(&constructing)

			api := &MiddlewareAPI[S]{
				// Forwarding closure: reads the `dispatch` value at call time,
				// so it picks up the fully-composed chain once built.
				Dispatch: func(action any) any { return (*dispatch.Load())(action) },
				GetState: base.GetState,
			}

			chain := make([]func(Dispatch) Dispatch, 0, len(middlewares))
			for _, mw := range middlewares {
				if mw == nil {
					// JavaScript reaches `middlewares.map(middleware => middleware(api))`
					// and throws "middleware is not a function". Skipping instead would
					// hand back a store that silently lacks the middleware the
					// caller asked for.
					panic(&Error{Message: "middleware is not a function"})
				}
				link := mw(api)
				chain = append(chain, func(next Dispatch) Dispatch {
					return link(Next(next))
				})
			}

			composed := Compose(chain...)(base.Dispatch)
			dispatch.Store(&composed)

			return &enhancedStore[S]{Store: base, dispatch: composed}
		}
	}
}

// enhancedStore is the Go equivalent of `{ ...store, dispatch }`: it delegates
// every method to the base store except Dispatch, which is replaced by the
// composed middleware chain.
type enhancedStore[S any] struct {
	Store[S]
	dispatch Dispatch
}

func (e *enhancedStore[S]) Dispatch(action any) any { return e.dispatch(action) }
