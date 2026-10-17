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

.PHONY: build test lint staticcheck ci clean quick

build:
	$(GO) build -o bin/gocpp ./cmd/gocpp

# -timeout 30m: the runtime package builds every C++ test with g++ and clang++
# in three modes, which can pass go test's 10 minute default when all packages
# run at once on a loaded machine.
test:
	$(GO) test -timeout 30m ./...

# The test programs alone (tests/ and examples/, see tests/README.md),
# verbose so that every skip says why, and never from go test's cache.
# "make test" runs them too, as part of go test ./....
.PHONY: test-programs
test-programs:
	$(GO) test -count=1 -v ./tests/...

# The sanitizer job (Phase 03 exit criteria): the test programs that run code
# (tests/run, tests/diff, tests/interop and examples/) with gocpp run -asan
# (AddressSanitizer, LeakSanitizer and UndefinedBehaviorSanitizer; a test
# marked "// lsan: cycle" leaks a reference cycle on purpose and runs without
# LeakSanitizer), then the emitter's tests, which build and run programs
# (internal/cppgen), with the same sanitizers in GOCPP_TEST_CXXFLAGS; with
# both g++ and clang++. internal/cppgen's TestFatal skips itself here (a
# sanitizer reports running out of memory and stack overflows on its own, and
# the runtime's stack-overflow handler is off under ASan). The runtime's own
# C++ tests have a sanitizer mode in make test already. -parallel 2, because
# sanitized builds and programs are big. The repo has no CI service yet
# (TODO.md, Q9), so this target is the CI job: run it next to make ci when a
# change touches the runtime, the emitter or the tests. GOCPP_TEST_PHASE=03
# make ci-sanitize runs the tests of a phase that is being built.
SANITIZE_FLAGS = -fsanitize=address,undefined -fno-omit-frame-pointer -fno-sanitize-recover=all
.PHONY: ci-sanitize
ci-sanitize: export GOCPP_REQUIRE_CXX ?= g++,clang++
ci-sanitize:
	GOCPP_FLAGS="$(GOCPP_FLAGS) -asan" $(GO) test -count=1 -timeout 60m -parallel 2 \
		-run '^(TestRun|TestDiff|TestInterop|TestExamples)$$' ./tests/...
	GOCPP_TEST_CXXFLAGS="$(GOCPP_TEST_CXXFLAGS) $(SANITIZE_FLAGS)" \
		$(GO) test -count=1 -timeout 60m -parallel 2 ./internal/cppgen/...
	@echo "make ci-sanitize: all checks passed"

# Fails if gofmt lists a Go file outside LINT_SKIP, then runs go vet.
lint:
	@bad=$$(find . \( $(LINT_SKIP) \) -prune -o -type f -name '*.go' -exec $(GOFMT) -l {} +) || exit 1; \
	if [ -n "$$bad" ]; then \
		echo "gofmt: these files need formatting (gofmt -w):"; echo "$$bad"; exit 1; \
	fi
	$(GO) vet ./...

staticcheck:
	$(STATICCHECK) ./...

# The tests of the VS Code extension: its grammar and its code
# (editors/vscode/README.md). Not part of ci, because npm downloads the
# packages the first time. npm ci runs when node_modules is missing or older
# than package-lock.json. One test builds gocpp with go build.
VSCODE_DIR = editors/vscode
.PHONY: vscode-test
vscode-test: $(VSCODE_DIR)/node_modules/.package-lock.json
	cd $(VSCODE_DIR) && npm test

$(VSCODE_DIR)/node_modules/.package-lock.json: $(VSCODE_DIR)/package-lock.json
	cd $(VSCODE_DIR) && npm ci --no-audit --no-fund

ci: $(CI_CHECKS)
	@echo "make ci: all checks passed"

# The edit-test loop: only what differs from the merge base with dev (gofmt, vet and the tests
# of the changed Go packages, the changed test fixtures with one compiler, a small smoke set,
# and for runtime/ changes its fast tests and the debug-mode C++ tests), in a minute or two
# instead of the ten of make ci. It is not a gate: run make ci and make staticcheck before a
# change is handed over or merged, and make ci-sanitize when it touches the runtime or the
# emitter. QUICKFLAGS="-base main" or "-n" (print only) pass flags to tools/quick.
quick:
	$(GO) run ./tools/quick $(QUICKFLAGS)

clean:
	rm -rf bin
