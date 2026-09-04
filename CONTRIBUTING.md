:+1: :tada: :sparkling_heart: Thanks for your interest! :sparkling_heart: :tada:
:+1:

## Tests

- Write a test for each use case.
- Try to stick to one assertion per use case.
- Too thorough is better than not thorough enough.

## Fidelity to the TypeScript original

This repository is a port of [redux-thunk](https://github.com/reduxjs/redux-thunk) v3.1.0. Runtime behaviour is not open for redesign: if a change makes the Go code diverge from the TypeScript source, it needs a row in the divergence table in [`docs/MIGRATION.md`](docs/MIGRATION.md) explaining why Go cannot express the original.

- The nine tests in `thunk_test.go` mirror `test/index.test.ts` one-to-one, including the `describe`/`it` names. Keep them aligned.
- Negative type assertions live in `testdata/typeerrors/`, one file per assertion, each starting with a `// want: <substring>` directive.
- Error message strings from Redux and from the JavaScript runtime are reproduced verbatim. Do not "improve" them.

## Before opening a pull request

```sh
make check
```

which runs the build, `go vet`, a `gofmt` check, the race-enabled test suite, and the external-consumer module build.
