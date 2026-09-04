// Command qcprobe is the Go half of the differential harness.
//
// The TypeScript half lives in the source repository as `qc_probe.mts` and is
// driven by the same probe names through `qc_fixtures.json`. Both programs
// print the same lines when the port is faithful, so a divergence in observable
// behaviour shows up as a textual diff rather than as a judgement call. Only
// values that carry across both languages are printed: no stack traces, no
// pointer values, no ordering that either runtime is free to choose.
package main

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"

	"github.com/reduxjs/redux-thunk-go"
	"github.com/reduxjs/redux-thunk-go/redux"
)

type extraArgument struct{ Lol bool }

// doDispatch, doGetState, nextSentinel and thunkSentinel are the upstream test
// fixtures from test/index.test.ts, kept identical so the probes exercise the
// same values the ported suite does.
const (
	nextSentinel  = "redux"
	thunkSentinel = "rocks"
)

var (
	doDispatch redux.Dispatch = func(any) any { return nil }
	doGetState                = func() any { return 42 }
)

func sameRef(a, b any) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func actionHandler() redux.Dispatch {
	return thunk.Thunk(&redux.MiddlewareAPI[any]{Dispatch: doDispatch, GetState: doGetState})(nil)
}

func probeMiddlewareShape(w io.Writer) {
	nextHandler := thunk.Thunk(&redux.MiddlewareAPI[any]{Dispatch: doDispatch, GetState: doGetState})
	fmt.Fprintf(w, "next-handler-type=%s\n", "function")
	fmt.Fprintf(w, "next-handler-arity=%d\n", reflect.TypeOf(nextHandler).NumIn())

	handler := nextHandler(nil)
	fmt.Fprintf(w, "action-handler-type=%s\n", "function")
	fmt.Fprintf(w, "action-handler-arity=%d\n", reflect.TypeOf(handler).NumIn())
}

func probeDispatchIdentity(w io.Writer) {
	actionHandler()(func(dispatch redux.Dispatch, getState func() any) any {
		fmt.Fprintf(w, "dispatch-identity=%t\n", sameRef(dispatch, doDispatch))
		fmt.Fprintf(w, "getstate-identity=%t\n", sameRef(getState, doGetState))
		fmt.Fprintf(w, "getstate-value=%v\n", getState())
		return nil
	})
}

func probePlainAction(w io.Writer) {
	actionObj := redux.AnyAction{}
	var received any
	handler := thunk.Thunk(&redux.MiddlewareAPI[any]{Dispatch: doDispatch, GetState: doGetState})(
		func(action any) any {
			received = action
			return nextSentinel
		})

	result := handler(actionObj)
	fmt.Fprintf(w, "next-received-same-action=%t\n", sameRef(received, actionObj))
	fmt.Fprintf(w, "next-return=%v\n", result)
}

func probeThunkReturn(w io.Writer) {
	fmt.Fprintf(w, "thunk-return=%v\n", actionHandler()(func() any { return thunkSentinel }))
}

func probeSynchronous(w io.Writer) {
	mutated := 0
	actionHandler()(func() any {
		mutated++
		return nil
	})
	fmt.Fprintf(w, "mutated=%d\n", mutated)
}

func probeExtraArgument(w io.Writer) {
	extra := &extraArgument{Lol: true}
	handler := thunk.WithExtraArgument[any](extra)(
		&redux.MiddlewareAPI[any]{Dispatch: doDispatch, GetState: doGetState})(nil)

	handler(func(_ redux.Dispatch, _ func() any, arg *extraArgument) any {
		fmt.Fprintf(w, "extra-identity=%t\n", arg == extra)
		fmt.Fprintf(w, "extra-value=%t\n", arg.Lol)
		return nil
	})
}

func probeArity(w io.Writer) {
	handler := actionHandler()

	fmt.Fprintf(w, "rest-only=%v\n", handler(func(args ...any) any { return len(args) }))
	fmt.Fprintf(w, "dispatch-plus-rest=%v\n",
		handler(func(_ redux.Dispatch, rest ...any) any { return len(rest) }))
	fmt.Fprintf(w, "fourth-is-absent=%v\n",
		handler(func(_ redux.Dispatch, _ func() any, _ any, fourth any) any { return fourth == nil }))
}

func probeMissingAPI(w io.Writer) {
	defer func() {
		// The message is printed instead of the panic value because a Go panic
		// and a JavaScript TypeError share nothing but their text, and the text
		// is exactly what this port promises to reproduce verbatim.
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(w, "error=%v\n", recovered)
			return
		}
		fmt.Fprintln(w, "error=none")
	}()

	thunk.Thunk(nil)
}

var probes = map[string]func(io.Writer){
	"middleware-shape":  probeMiddlewareShape,
	"dispatch-identity": probeDispatchIdentity,
	"plain-action":      probePlainAction,
	"thunk-return":      probeThunkReturn,
	"synchronous":       probeSynchronous,
	"extra-argument":    probeExtraArgument,
	"arity":             probeArity,
	"missing-api":       probeMissingAPI,
}

func probeNames() []string {
	names := make([]string, 0, len(probes))
	for name := range probes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: qcprobe <probe>")
	fmt.Fprintln(w, "probes:")
	for _, name := range probeNames() {
		fmt.Fprintf(w, "  %s\n", name)
	}
}

func run(stdout, stderr io.Writer, args []string) int {
	if len(args) != 1 {
		usage(stderr)
		return 2
	}
	if args[0] == "--help" {
		usage(stdout)
		return 0
	}

	probe, known := probes[args[0]]
	if !known {
		fmt.Fprintf(stderr, "unknown probe: %s\n", args[0])
		return 2
	}
	probe(stdout)
	return 0
}

func main() {
	os.Exit(run(os.Stdout, os.Stderr, os.Args[1:]))
}
