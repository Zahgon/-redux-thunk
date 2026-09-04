package async_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/reduxjs/redux-thunk-go/async"
)

var errBoom = errors.New("boom")

func TestResolved(t *testing.T) {
	f := async.Resolved(42)

	if !f.Settled() {
		t.Error("expected Resolved to produce an already-settled Future")
	}

	val, err := f.Await()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Errorf("expected 42, got %d", val)
	}
}

func TestRejected(t *testing.T) {
	f := async.Rejected[int](errBoom)

	val, err := f.Await()
	if !errors.Is(err, errBoom) {
		t.Errorf("expected errBoom, got %v", err)
	}
	if val != 0 {
		t.Errorf("expected the zero value on rejection, got %d", val)
	}
}

func TestNewRunsTheFunction(t *testing.T) {
	f := async.New(func() (string, error) { return "hello", nil })

	val, err := f.Await()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != "hello" {
		t.Errorf("expected %q, got %q", "hello", val)
	}
}

func TestAwaitIsIdempotentAcrossGoroutines(t *testing.T) {
	f := async.New(func() (int, error) { return 7, nil })

	const goroutines = 32
	var wg sync.WaitGroup
	results := make([]int, goroutines)

	wg.Add(goroutines)
	for i := range results {
		go func() {
			defer wg.Done()
			val, _ := f.Await()
			results[i] = val
		}()
	}
	wg.Wait()

	for i, got := range results {
		if got != 7 {
			t.Fatalf("goroutine %d observed %d, expected 7", i, got)
		}
	}
}

func TestThenChains(t *testing.T) {
	f := async.Then(async.Resolved(2), func(n int) (string, error) {
		if n != 2 {
			t.Errorf("expected the continuation to receive 2, got %d", n)
		}
		return "two", nil
	})

	val, err := f.Await()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != "two" {
		t.Errorf("expected %q, got %q", "two", val)
	}
}

func TestThenSkipsContinuationOnFailure(t *testing.T) {
	called := false

	f := async.Then(async.Rejected[int](errBoom), func(int) (string, error) {
		called = true
		return "unreachable", nil
	})

	_, err := f.Await()
	if !errors.Is(err, errBoom) {
		t.Errorf("expected errBoom to propagate, got %v", err)
	}
	if called {
		t.Error("expected the continuation not to run after a failure")
	}
}

func TestAllCollectsInOrder(t *testing.T) {
	f := async.All(
		async.New(func() (int, error) { return 1, nil }),
		async.Resolved(2),
		async.New(func() (int, error) { return 3, nil }),
	)

	values, err := f.Await()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, want := range []int{1, 2, 3} {
		if values[i] != want {
			t.Errorf("index %d: expected %d, got %d", i, want, values[i])
		}
	}
}

func TestAllReportsTheFirstFailure(t *testing.T) {
	f := async.All(
		async.Resolved(1),
		async.Rejected[int](errBoom),
		async.Resolved(3),
	)

	values, err := f.Await()
	if !errors.Is(err, errBoom) {
		t.Errorf("expected errBoom, got %v", err)
	}
	if values != nil {
		t.Errorf("expected no values on failure, got %v", values)
	}
}

func TestDoneChannelSelects(t *testing.T) {
	f := async.Resolved("ready")

	select {
	case <-f.Done():
	default:
		t.Fatal("expected Done to be closed for a settled Future")
	}
}
