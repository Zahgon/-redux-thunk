// Package async provides Future, a minimal stand-in for the JavaScript Promise
// used throughout the redux-thunk documentation and type tests.
//
// The original redux-thunk README is built almost entirely around async thunks
// that return promises, and its type tests assert on `ThunkResult<Promise<T>>`.
// Go has no Promise type: the idiomatic equivalents are a goroutine plus a
// channel, or simply a blocking call. Neither is a drop-in for a value that a
// thunk returns to its caller, which is precisely what a promise-returning
// thunk does.
//
// Future closes that gap. It is a one-shot, write-once container for a value
// that becomes available later, and it is safe for concurrent use. It exists so
// that the ported examples and tests can express the same control flow as the
// originals; it is not intended as a general-purpose futures library, and
// production Go code is usually better served by plain channels or
// golang.org/x/sync/errgroup.
//
// Mapping from JavaScript:
//
//	new Promise(...)        ->  async.New
//	Promise.resolve(v)      ->  async.Resolved
//	Promise.reject(e)       ->  async.Rejected
//	await p                 ->  f.Await()
//	p.then(fn)              ->  async.Then
//	Promise.all([...])      ->  async.All
package async

import "sync"

// Future is a value that will become available at some point.
//
// The zero Future is not usable; construct one with New, Resolved or Rejected.
type Future[T any] struct {
	done chan struct{}
	once sync.Once
	val  T
	err  error
}

func newPending[T any]() *Future[T] {
	return &Future[T]{done: make(chan struct{})}
}

// settle records the outcome. Only the first call has any effect, which gives
// Future the same write-once semantics as a settled promise.
func (f *Future[T]) settle(val T, err error) {
	f.once.Do(func() {
		f.val, f.err = val, err
		close(f.done)
	})
}

// New runs fn on its own goroutine and returns a Future for its outcome. It is
// the equivalent of the Promise constructor: the work starts immediately rather
// than being deferred until the result is awaited.
//
// A panic inside fn propagates on the goroutine, matching Go's normal rules
// rather than JavaScript's implicit rejection. Return an error instead.
func New[T any](fn func() (T, error)) *Future[T] {
	f := newPending[T]()
	go func() {
		val, err := fn()
		f.settle(val, err)
	}()
	return f
}

// Resolved returns a Future that has already succeeded with val, the equivalent
// of Promise.resolve.
func Resolved[T any](val T) *Future[T] {
	f := newPending[T]()
	f.settle(val, nil)
	return f
}

// Rejected returns a Future that has already failed with err, the equivalent of
// Promise.reject.
func Rejected[T any](err error) *Future[T] {
	f := newPending[T]()
	var zero T
	f.settle(zero, err)
	return f
}

// Await blocks until the Future settles and returns its outcome. It is the
// equivalent of the `await` operator. Await may be called any number of times
// from any number of goroutines and always reports the same outcome.
func (f *Future[T]) Await() (T, error) {
	<-f.done
	return f.val, f.err
}

// Done returns a channel that is closed once the Future settles, so that a
// Future can participate in a select statement.
func (f *Future[T]) Done() <-chan struct{} {
	return f.done
}

// Settled reports whether the Future has already settled, without blocking.
func (f *Future[T]) Settled() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

// Then chains a continuation onto f, the equivalent of Promise.prototype.then.
//
// It is a free function rather than a method because Go methods cannot
// introduce new type parameters, and the continuation is what changes the
// value type.
//
// If f fails, fn is not called and the failure propagates unchanged, matching
// the way a rejected promise skips its onFulfilled handler.
func Then[A, B any](f *Future[A], fn func(A) (B, error)) *Future[B] {
	return New(func() (B, error) {
		val, err := f.Await()
		if err != nil {
			var zero B
			return zero, err
		}
		return fn(val)
	})
}

// All waits for every Future to settle and collects the values in order. It is
// the equivalent of Promise.all: the first failure in argument order is
// returned, and the value slice is nil in that case.
//
// Unlike Promise.all it always waits for every Future rather than settling as
// soon as one rejects, so that no goroutine is left running unobserved.
func All[T any](futures ...*Future[T]) *Future[[]T] {
	return New(func() ([]T, error) {
		values := make([]T, len(futures))
		var firstErr error
		for i, f := range futures {
			val, err := f.Await()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			values[i] = val
		}
		if firstErr != nil {
			return nil, firstErr
		}
		return values, nil
	})
}
