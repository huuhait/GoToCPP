# gocpp Makefile. "make ci" runs every check; run it before handing over a
# change. The recipes are POSIX sh.

GO ?= go
GOFMT ?= gofmt

# The checks "make ci" runs. A later check is added here, for example the
# GCC + Clang matrix: CI_CHECKS = lint test build cxx-matrix
CI_CHECKS = lint test build

# The paths "make lint" does not gofmt-check, as find(1) tests joined with -o:
# spikes/ (throwaway code in its own Go module) and every testdata folder
# (test inputs need not be gofmt-clean, for example Go's own parser testdata
# in Phase 01; go build and go vet skip testdata folders too).
LINT_SKIP = -path ./spikes -o -path ./.git -o -name testdata

# staticcheck is not part of lint or ci, because "go run" downloads it the
# first time. v0.7.0 is the newest release that works with Go 1.25.
STATICCHECK ?= $(GO) run honnef.co/go/tools/cmd/staticcheck@v0.7.0

.PHONY: build test lint staticcheck ci clean

build:
	$(GO) build -o bin/gocpp ./cmd/gocpp

test:
	$(GO) test ./...

# Fails if gofmt lists a Go file outside LINT_SKIP, then runs go vet.
lint:
	@bad=$$(find . \( $(LINT_SKIP) \) -prune -o -type f -name '*.go' -exec $(GOFMT) -l {} +) || exit 1; \
	if [ -n "$$bad" ]; then \
		echo "gofmt: these files need formatting (gofmt -w):"; echo "$$bad"; exit 1; \
	fi
	$(GO) vet ./...

staticcheck:
	$(STATICCHECK) ./...

ci: $(CI_CHECKS)
	@echo "make ci: all checks passed"

clean:
	rm -rf bin
