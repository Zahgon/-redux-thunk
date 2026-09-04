# Translation of the upstream package.json scripts. See docs/MIGRATION.md §7.

GO ?= go
COVERPROFILE ?= coverage.out
CONSUMER_DIR := examples/consumer

.DEFAULT_GOAL := check

.PHONY: check build fmt lint test test-short cover cover-html consumer clean

## check: everything CI runs.
check: build lint test consumer

## build: `yarn build`.
build:
	$(GO) build ./...

## fmt: `yarn format`.
fmt:
	gofmt -w .
	@command -v gofumpt >/dev/null 2>&1 && gofumpt -w . || true

## lint: `yarn lint`. golangci-lint is optional so the target works on a bare
## Go install; go vet and the gofmt check always run.
lint:
	$(GO) vet ./...
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt reported unformatted files:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

## test: `yarn test`. The type-error suite runs as part of this, so there is no
## separate `type-tests` target.
test:
	$(GO) test -race ./...

## test-short: skips the type-error suite, which shells out to the compiler.
test-short:
	$(GO) test -short ./...

## cover: coverage report.
cover:
	$(GO) test -coverprofile=$(COVERPROFILE) -covermode=atomic ./...
	$(GO) tool cover -func=$(COVERPROFILE)

cover-html: cover
	$(GO) tool cover -html=$(COVERPROFILE) -o coverage.html

## consumer: the Go analogue of the upstream `test-published-artifact` job --
## build and test the library from a separate module.
consumer:
	cd $(CONSUMER_DIR) && $(GO) build ./... && $(GO) test -race ./...

clean:
	$(GO) clean -testcache
	rm -f $(COVERPROFILE) coverage.html
