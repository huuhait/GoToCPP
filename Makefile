# gocpp Makefile. "make ci" runs every check; run it before handing over a
# change. The recipes are POSIX sh.

GO ?= go
GOFMT ?= gofmt

# The checks "make ci" runs. A later check is added here. The GCC + Clang
# matrix is part of test: the test programs run with both compilers.
CI_CHECKS = lint test build

# In "make ci", a C++ compiler that isn't installed fails the test programs
# (outside it, they skip that compiler). GOCPP_REQUIRE_CXX= make ci turns
# this off; tests/README.md has the details.
ci: export GOCPP_REQUIRE_CXX ?= g++,clang++

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

# The test programs alone (tests/ and examples/, see tests/README.md),
# verbose so that every skip says why, and never from go test's cache.
# "make test" runs them too, as part of go test ./....
.PHONY: test-programs
test-programs:
	$(GO) test -count=1 -v ./tests/...

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
