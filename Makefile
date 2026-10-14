# gocpp Makefile. "make ci" runs every required check (staticcheck is the
# separate "make staticcheck"); run it before handing over a change. The
# recipes are POSIX sh.

GO ?= go
GOFMT ?= gofmt

# The checks "make ci" runs. A later check is added here. The GCC + Clang
# matrix is part of test: the test programs run with both compilers.
CI_CHECKS = lint test build

# In "make ci", a C++ compiler that isn't installed fails the test programs
# (outside it, they skip that compiler). GOCPP_REQUIRE_CXX= make ci turns
# this off; tests/README.md has the details.
ci: export GOCPP_REQUIRE_CXX ?= g++,clang++

# In "make ci", the debug tests (tests/debug) fail instead of skipping their
# gdb part when gdb is missing or may not trace programs (no ptrace).
# GOCPP_REQUIRE_GDB= make ci turns this off.
ci: export GOCPP_REQUIRE_GDB ?= 1

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

# The tests of the VS Code grammar (editors/vscode/README.md). Not part of ci,
# because npm downloads the test tools the first time. npm ci runs when
# node_modules is missing or older than package-lock.json.
VSCODE_DIR = editors/vscode
.PHONY: vscode-test
vscode-test: $(VSCODE_DIR)/node_modules/.package-lock.json
	cd $(VSCODE_DIR) && npm test

$(VSCODE_DIR)/node_modules/.package-lock.json: $(VSCODE_DIR)/package-lock.json
	cd $(VSCODE_DIR) && npm ci --no-audit --no-fund

ci: $(CI_CHECKS)
	@echo "make ci: all checks passed"

clean:
	rm -rf bin
