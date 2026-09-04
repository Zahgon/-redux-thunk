# Migration notes: redux-thunk (TypeScript) → redux-thunk-go

This document records exactly how [redux-thunk](https://github.com/reduxjs/redux-thunk) v3.1.0 was ported to Go, what is behaviourally identical, and where the two languages forced a divergence.

The guiding rule: **runtime behaviour is non-negotiable and is reproduced exactly. Type-level API is adapted only where Go cannot express the TypeScript construct.**

## 1. File map

| TypeScript source | Go |
| --- | --- |
| `src/index.ts` | `thunk.go` |
| `src/types.ts` | `types.go`, `dispatch.go`, `bind.go` |
| _(peer dependency `redux@^5`)_ | `redux/` |
| _(host `Promise`)_ | `async/` |
| `test/index.test.ts` | `thunk_test.go` |
| `typescript_test/index.test-d.ts` | `types_test.go` + `testdata/typeerrors/` + `typeerrors_test.go` |
| `README.md` samples | `examples/`, `example_test.go` |
| `package.json` scripts | `Makefile` |
| `tsup.config.ts`, `vitest.config.mts`, `tsconfig*.json` | _(none needed — the Go toolchain covers build, test and type-check)_ |
| `.github/workflows/test.yml` | `.github/workflows/test.yml` |
| `.eslintrc`, `.prettierrc.json` | `.golangci.yml`, `gofmt` |
| `scripts/writeGitVersion.mts` | `scripts/version.sh` |

## 2. The runtime, line by line

The entire original runtime is thirteen lines:

```ts
function createThunkMiddleware<State = any, BasicAction extends Action = AnyAction, ExtraThunkArg = undefined>(
  extraArgument?: ExtraThunkArg,
) {
  const middleware: ThunkMiddleware<State, BasicAction, ExtraThunkArg> =
    ({ dispatch, getState }) => next => action => {
      if (typeof action === 'function') {
        return action(dispatch, getState, extraArgument)
      }
      return next(action)
    }
  return middleware
}

export const thunk = createThunkMiddleware()
export const withExtraArgument = createThunkMiddleware
```

Each behaviour is preserved:

| # | Invariant | Where it is enforced | Where it is tested |
| --- | --- | --- | --- |
| 1 | The middleware is a three-level curried function | `createThunkMiddleware` | `thunk_test.go` tests 1–2 |
| 2 | Level 2 and level 3 each take exactly one parameter | same | `arity` assertions |
| 3 | Destructuring an *absent* API object throws a `TypeError`; an empty one does not | `api == nil` panic at level 1 | test 8, `TestMiddlewareAcceptsAnEmptyAPI` |
| 4 | A function action is called, a non-function action is passed to `next` | `invokeThunk` | tests 3–5 |
| 5 | The thunk receives the *store's own* `dispatch` and `getState`, not copies | fast-path type switch preserves identity — **for `getState`, only when the thunk declares the middleware's own state type**; see below | test 3 (pointer identity), `TestGetStateIdentityHoldsOnlyWhenTheSignaturesMatch` |
| 6 | The thunk's return value is the return value of `dispatch` | level 3 returns it unchanged | test 6 |
| 7 | The thunk is invoked synchronously | no goroutine anywhere on the path | test 7 |
| 8 | The extra argument is passed as the third parameter | `WithExtraArgument` closure | test 9 |
| 9 | `thunk` is a module-level singleton constructed once | `var Thunk = WithExtraArgument[any, any](nil)` | — |
| 10 | `withExtraArgument` is the factory itself | `WithExtraArgument` forwards to the single implementation | test 9 |
| 11 | Non-function actions reach `next` untouched, by identity | `next(action)` | test 4 |

### Thunk arity

JavaScript calls every thunk with exactly three arguments, and the parameter list decides what the thunk sees. `invokeThunk` reproduces all four cases:

| Thunk signature | JavaScript | This port |
| --- | --- | --- |
| `(dispatch, getState, extra)` and shorter | receives that prefix | same |
| `(a, b, c, d)` | `d` is `undefined` | `d` is its zero value |
| `(...rest)` | `rest.length === 3` | the variadic slot receives all three |
| `(dispatch, ...rest)` | `rest.length === 2` | fixed parameters consume their share first, the rest go to the slot |

The only rejected shape is a thunk with more than one result, which has no JavaScript counterpart: a JS function returns exactly one value, `dispatch` can propagate exactly one value, and dropping the others would silently discard an `error`.

The common shapes are matched by a type switch, which avoids reflection. Anything else falls through to a reflection path that adapts parameter types where necessary.

### Where identity is and is not preserved

`dispatch` always reaches the thunk by identity: `redux.Dispatch` is one concrete type on every path.

`getState` reaches the thunk by identity only when the thunk declares the same state type the middleware was instantiated with. A `ThunkAction[R, any, E]` dispatched into a `TypedThunk[State]` store asks for a `func() any` while the middleware holds a `func() State`; those are distinct Go types, and no single value inhabits both, so the reflective path builds a forwarding adapter. TypeScript has no equivalent problem because `getState` is one runtime function regardless of its static type.

Practical consequence: declare thunks against the store's state type (`ThunkAction[R, State, E]`) and the fast path is taken. The adapter is correct either way — it forwards arguments and results, mapping a nil (`undefined`) state to the zero value — but it is not the same function value.

## 3. Divergence table

| # | TypeScript construct | Why Go cannot express it | Go substitute | Runtime impact |
| --- | --- | --- | --- | --- |
| 1 | `ThunkAction`'s fourth type parameter `BasicAction` | It exists only to constrain the `ThunkDispatch` overloads. Go's `Dispatch` is `func(any) any`, so there is nothing to constrain. | Dropped. Action typing is enforced at the reducer (type switch) and at the call site (`DispatchAction`). | None |
| 2 | `Middleware`'s `_DispatchExt` widening parameter | Go cannot re-type an existing value through a wrapper. | `DispatchThunk` / `DispatchAction` / `DispatchEither` recover the static type at the call site. | None |
| 3 | The three `ThunkDispatch` overloads | Go has no function overloading. | Three generic functions. The original's warning — *"the order here matters for correct behavior and is very fragile - do not reorder these!"* — is preserved as documentation on `ThunkDispatch`. | None |
| 4 | Overload 3's union return `Action \| ReturnType` | Go has no union types. | `DispatchEither` returns `any`; the caller type-asserts. | None at runtime; **this is the largest static-fidelity gap**. It is the workaround for [TS#14107](https://github.com/microsoft/TypeScript/issues/14107) / [redux-thunk#248](https://github.com/reduxjs/redux-thunk/issues/248), a problem that does not exist in Go. |
| 5 | `ThunkActionDispatch<AC>` = `(...args: Parameters<AC>) => ReturnType<ReturnType<AC>>` | Go has no variadic generics and no `Parameters<T>`. | `Bind0`…`Bind3`, `BindPlain0`, `BindPlain1`. | None |
| 6 | Default type parameters (`State = any`, `ExtraThunkArg = undefined`) | Go generics have no defaults. | `ThunkActionAny`, and the `Thunk` singleton instantiated at `[any, any]`. | None |
| 7 | Structural assignability of `ThunkMiddleware` to `Middleware` | Go function types are nominal. | `AsMiddleware()` method. | None |
| 8 | `thunk as ThunkMiddleware<State, Actions>` | Go cannot re-type a value. | `TypedThunk[S]()` constructs an identical middleware at the required type. | None |
| 9 | Generic type aliases (`type ThunkResult<R> = ThunkAction<R, State, undefined, Actions>`) | Requires Go 1.23; this module targets 1.22. | Concrete aliases per instantiation, e.g. `ThunkResultString`. | None |
| 10 | Discriminated unions (`{ type: 'FOO' } \| { type: 'BAR' }`) | Go has no untagged unions. | Closed interface with an unexported marker method. | None; membership is checked at compile time, as in TypeScript. |
| 11 | `Promise` | Not in the Go standard library. | `async.Future`. | None; `Await` blocks where `await` suspends. |
| 12 | `throw new TypeError(...)` | Go has no exceptions. | `panic(&thunk.TypeError{...})` with the message string reproduced verbatim. | Equivalent: both unwind, both are recoverable (`try`/`catch` ⇄ `recover`). |
| 13 | `undefined` | No Go equivalent. | `nil` for the extra argument; the zero value elsewhere. | None on the paths the original exercises. |
| 14 | Single-threaded event loop | Go is concurrent. | Store serialises dispatches; reentrancy guards are goroutine-scoped. | **Additive**: the original's guarantees hold, plus new ones. See §5. |
| 15 | `getState` is one runtime value whatever its static type | Go function types are nominal, so `func() State` ≠ `func() any`. | Identity on the fast path; a `reflect.MakeFunc` adapter otherwise. | A thunk declaring a state type other than the middleware's receives an equivalent but non-identical `getState`. See §2. |
| 16 | The extra argument is shared by reference | Go passes struct values by copy. | Use a pointer or interface type for a mutable extra argument, as `examples/extraargument` does. | With a struct *value*, a mutation by one thunk is invisible to the next; with `*Extra` the JavaScript semantics hold. |
| 17 | A thunk returns exactly one value | Go functions may return several. | Multi-result thunks panic with a `*thunk.TypeError`. | Rejects a signature the original cannot express, rather than silently dropping results. |
| 18 | `let dispatch` rebound during `applyMiddleware` | A plain variable write racing with reads from other goroutines. | `atomic.Pointer[Dispatch]`. | None; removes a real race. |
| 19 | Listener `Map` iteration order | Go map iteration is randomised. | Ordered slice. | None; restores subscription order. |
| 20 | `createStore(reducer, preloadedState, enhancer)` arity overloads | Go has no overloading or `undefined`. | Options plus an `enhancerSet` flag so a nil enhancer is rejected rather than ignored. | None. |

## 4. The `redux` subpackage

The original imports only three type-level symbols from `redux`:

```ts
import type { Action, AnyAction, Middleware } from 'redux'
```

Its *runtime* suite never touches Redux at all — `test/index.test.ts` calls the middleware directly with stub `dispatch` and `getState` functions. `createStore` and `applyMiddleware` appear only in `typescript_test/`, which is type-checked but never executed. What genuinely needs a store is the README: nearly every example builds one, and the ported examples must compile and run. There is no canonical Redux port for Go, so `redux/` supplies exactly those pieces and nothing more.

This makes `redux/` **new code, not migrated code** — the part of this repository least protected by the upstream suite, and the part most likely to diverge. It has its own tests, and every known divergence is listed below.

Fidelity details:

- Every error message the store raises is reproduced verbatim, including `"Expected the root reducer to be a function."`, `"Reducers may not dispatch actions."` and `"Dispatching while constructing your middleware is not allowed. Other middleware would not be applied to this dispatch."`
- `Dispatch` applies Redux's two entry guards before touching the reducer, in Redux's order. The first is the one a thunk user meets most often: dispatching a function into a store that lacks the middleware raises `"Actions must be plain objects. Instead, the actual type was: 'function'. You may need to add middleware … such as 'redux-thunk' to handle dispatching functions …"`. The second raises `"Actions may not have an undefined \"type\" property. You may have misspelled an action type string constant."`. "Plain object" maps to a value implementing `redux.Action`, a string-keyed map, or a struct; everything else — functions above all — is rejected. `redux/action_test.go` pins both.
- A nil entry in `ApplyMiddleware`'s argument list panics rather than being skipped, because JavaScript reaches `middlewares.map(mw => mw(api))` and throws. Skipping would return a store that silently lacks the middleware the caller asked for.
- Listeners are held in a slice and notified in subscription order. Redux stores them in a JavaScript `Map`, which iterates in insertion order; ranging over a Go map would randomise it.
- `GetState` and unsubscribe are rejected while the reducer runs, with Redux's messages. This does **not** affect thunks: Redux's `isDispatching` is true only for the duration of the reducer call, and the middleware chain runs before the reducer is reached, so `getState` inside a thunk is legal in both implementations. `TestGetStateIsAllowedFromTheMiddlewareChain` pins that.
- `WithEnhancer(nil)` panics with `"Expected the enhancer to be a function. Instead, received: 'null'"`. Go cannot tell "argument omitted" from "argument supplied as nil", so `Config` carries an explicit `enhancerSet` flag.
- `BindActionCreators` **skips** map entries whose value is not a function, matching Redux's `if (typeof actionCreator === 'function')`. The singular `BindActionCreator` still panics, matching Redux's internal helper.
- All three of Redux's dispatch guards are present, in Redux's order: not-a-plain-object, undefined `type`, and non-string `type`. An empty-string `type` is legal, as upstream.
- The composed dispatch is published through an `atomic.Pointer`. JavaScript rebinds a plain `let`; in Go that is a genuine data race, since a middleware may hand the forwarding closure to another goroutine before construction finishes.
- The `@@redux/INIT` and `@@redux/REPLACE` action types carry a random suffix, as upstream does.
- `ApplyMiddleware` hands each middleware a *forwarding closure* over the composed `dispatch`, not the raw store dispatch — the subtle detail that lets a middleware re-dispatch through the whole chain. `redux/middleware_test.go` pins this down.
- `Compose` applies right to left.
- `BindActionCreator` uses reflection to preserve the action creator's parameter list (including variadics) while erasing its result to `any`, which is the closest Go gets to `(...args: Parameters<AC>) => ...`.

### Where the `kindOf` strings come from

`redux/action.go` names the offending value in JavaScript's vocabulary — `'function'`, `'string'`, `'number'`, `'boolean'`, `'array'`, `'undefined'`, `'null'` — because those strings are interpolated into error messages that are reproduced verbatim.

They are not invented here. They are the return values of `miniKindOf` in `redux@5.0.1` (`dist/cjs/redux.cjs`), which returns `typeof val` for `boolean`/`string`/`number`/`symbol`/`function`, `"undefined"` and `"null"` for the two empty values, `"array"` for arrays, and a constructor name otherwise. A static scan will not find them anywhere in the scraped TypeScript tree, because `redux` is a *peer* dependency: it is declared, never vendored, and only ever imported `import type`. The literals were checked against an installed `redux@5.0.1` rather than transcribed from memory.

`kindOf` maps the Go kinds that can actually reach it and falls back to `reflect.Type.String()` for the rest, since Go has kinds — channels, complex numbers — that JavaScript has no word for.

### Cost of the goroutine-scoped guards

`Dispatch` needs the calling goroutine's identity to keep Redux's reentrancy guards meaningful (see §5), and Go exposes no cheap way to get it. `runtime.Stack` unwinds the entire stack regardless of buffer size, so the price scales with call depth — which matters, because thunks dispatch from inside middleware chains.

Measured on an M1 Pro (`go test ./redux/ -bench Dispatch`):

| Operation | ns/op | allocs |
| --- | --- | --- |
| `Dispatch` (shallow) | ~2,600 | 0 |
| `Dispatch` at stack depth 64 | ~26,000 | 0 |
| `GetState` | ~31 | 0 |

The identity is therefore read **at most once per dispatch** and threaded through, never recomputed; `GetState`, `Subscribe` and unsubscribe skip it entirely unless a reducer is actually running; the buffer is pooled and the digits parsed in place; and action validation avoids `reflect` on the common shapes. `redux/bench_test.go` guards all of this against regression.

The alternative — a plain mutex with no identity — would make a reducer that dispatches deadlock instead of raising `"Reducers may not dispatch actions."`, trading a clear upstream error for a hang. That trade was judged not worth it.

## 5. Concurrency

This is the one place where the port is deliberately *more* defined than the original, because it has to be.

Redux's `isDispatching` flag is a plain boolean, which is sound only because JavaScript runs one thing at a time. Copying it literally into Go made two concurrent dispatches from different goroutines panic at each other with `"Reducers may not dispatch actions."` — a false positive that inverts the guard's meaning.

The fix, in `redux/goroutine.go` and `redux/store.go`:

- A `dispatchMu` mutex serialises whole dispatches, supplying what the JavaScript event loop supplies for free.
- The `isDispatching` boolean becomes `reducerG uint64`, the id of the goroutine currently inside the reducer. The guards fire only when the *calling* goroutine is that goroutine — which is precisely the original condition, expressed in a concurrent world.
- Listeners are invoked after `dispatchMu` is released, so a listener may dispatch, as Redux permits.

`concurrency_test.go` covers concurrent plain dispatches, concurrent thunk dispatches, `getState` from other goroutines, an async thunk resolving off the dispatch goroutine, and concurrent `Subscribe`/`Dispatch`. All run under `-race`.

None of these tests has a counterpart upstream; they exist because the question they answer cannot arise in JavaScript.

## 6. Test porting

### `test/index.test.ts` → `thunk_test.go`

All nine tests are ported one-to-one, with the `describe`/`it` tree reproduced as nested `t.Run` names so the two suites can be diffed by eye.

Two mappings are worth stating:

- `expect(dispatch).toBe(doDispatch)` asserts *identity*. Go cannot compare functions with `==`, so the tests compare `reflect.ValueOf(x).Pointer()`. This is exact for the single function literals used as fixtures, and it correctly fails if the implementation ever wraps them — which is the entire point of the assertion.
- `nextHandler()` is called with no arguments in JavaScript, so `next` is `undefined`. The Go port passes a `nil` `redux.Next`. Thunk actions never touch it; a non-thunk action panics with `"next is not a function"`, exactly as JavaScript throws. This is faithful, not a divergence.

### `typescript_test/index.test-d.ts` → `types_test.go` + `testdata/typeerrors/`

`expectTypeOf` assertions split in two:

- **Positive** assertions ("this expression has this type") become ordinary Go code. If it compiles, the assertion holds; the code is then also *run*, so the port gets behavioural coverage the original never had.
- **Negative** assertions (`@ts-expect-error`, "does not work in this case") become compile-failure fixtures in `testdata/typeerrors/`. Each file starts with a `// want: <substring>` directive. `typeerrors_test.go` compiles each one in a scratch directory inside the module and fails if the build *succeeds* or if the error does not contain the wanted substring. This is the Go equivalent of `tsc --noEmit` type tests.

All eight blocks of the original are represented. The block *"call dispatch async with specific actions"* is necessarily identical to *"call dispatch async with any action"* in Go, because `BasicAction` was dropped (divergence 1); the test records this explicitly rather than silently omitting it.

### Tests with no upstream counterpart

`api_test.go` covers what the divergences added: `Bind1`/`Bind2`/`Bind3`/`BindPlain1`, `DispatchEither`'s two arms, `DispatchAction`'s rejection of a rewritten result, `IsThunk`, the destructuring guard in both of its forms (absent vs empty API), and the nine canonical thunk signatures the middleware invokes without reflection.

It also pins every corner of the reflective path, because that path is where a Go port can silently diverge from JavaScript and no TypeScript test can catch it:

- the variadic spread, in both the `(...rest)` and `(dispatch, ...rest)` shapes;
- a four-parameter thunk running with its extra parameter zero-filled;
- a nil function value producing the *same* error on the fast and reflective paths;
- a nil (`undefined`) state reaching a concretely typed thunk as the zero value rather than panicking;
- `getState` identity holding on the fast path and demonstrably *not* holding through the adapter;
- a multi-result thunk and an unrelated parameter type being rejected.

`redux/*_test.go` covers the vendored store, and `concurrency_test.go` covers the concurrency model, as described above.

### The differential harness (`cmd/qcprobe`)

A ported test proves the Go code satisfies what the test *says*. It cannot prove the test still says what the original meant, because a port rewrites the assertion and the implementation together. The only defence against that is to run both implementations and compare their output.

`cmd/qcprobe` is a small command exposing eight probes over the observable behaviour of the middleware:

| Probe | What it prints |
| --- | --- |
| `middleware-shape` | the type and arity of the two curried handlers |
| `dispatch-identity` | whether `dispatch` and `getState` arrive by identity, and what `getState` returns |
| `plain-action` | whether a non-function action reaches `next` unchanged, and `next`'s return value |
| `thunk-return` | the value a thunk's return travels back as |
| `synchronous` | that the thunk ran before `dispatch` returned |
| `extra-argument` | whether the extra argument arrives by identity |
| `arity` | `(...rest).length`, `(dispatch, ...rest).length`, and whether a fourth parameter is absent |
| `missing-api` | the error text produced when the middleware is called with no API |

Its counterpart is `qc_probe.mts` in the *source* repository, which loads the real `src/index.ts` — not a transcription — and prints the same lines. The two programs agree byte-for-byte on the merged `stdout`+`stderr` stream and on the exit code, for all eight probes plus `--help`, an unknown probe name, and a missing argument.

That agreement is what produced the numbers quoted throughout this document: `(...rest).length === 3`, a fourth parameter arriving as `undefined`, `thunk({})` running instead of throwing, and the exact wording of the destructuring `TypeError`. Each was measured against Node before it was implemented in Go.

The Go half is exercised by `cmd/qcprobe/main_test.go`, which pins the exact output of every probe, so the contract cannot drift silently on this side.

## 7. Tooling translation

| npm script | Make target |
| --- | --- |
| `yarn build` (`tsup`) | `make build` (`go build ./...`) |
| `yarn test` (`vitest --run --typecheck`) | `make test` (`go test -race ./...`, which includes the type-error suite) |
| `yarn type-tests` | included in `make test`; `go vet` in `make lint` |
| `yarn lint` (`eslint`) | `make lint` (`go vet` + `gofmt -l` + `golangci-lint` when installed) |
| `yarn format` (`prettier`) | `make fmt` (`gofmt -w`) |
| `scripts/writeGitVersion.mts` | `scripts/version.sh` |

The upstream CI workflow's `test-published-artifact` matrix — which installs the built package into eight different app templates — has one meaningful Go analogue: building an external module that consumes the library. That is `examples/consumer`, a separate module with its own `go.mod`, built by `make check`.

`are-the-types-wrong` has no Go counterpart: there is no dual ESM/CJS packaging problem to detect.

## 8. Non-goals

- Redux Toolkit is not ported. `examples/consumer` reduces `configureStore` to the one behaviour the README sample demonstrates: the thunk middleware is installed by default.
- `react-redux` is not ported. The README's `connect`ed component is reduced in `examples/sandwich` to what the sample actually shows: a view that dispatches a thunk on mount and on prop change, and re-reads state via a store subscription.
- `async.Future` is a documentation shim, not a general-purpose futures library. Production Go code is usually better served by channels or `golang.org/x/sync/errgroup`.
