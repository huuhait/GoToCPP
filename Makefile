# gocpp Makefile. "make ci" runs every required check (staticcheck is the
# separate "make staticcheck"); run it before handing over a change. The
# recipes are POSIX sh.

GO ?= go
GOFMT ?= gofmt

# The checks "make ci" runs. A later check is added here. The GCC + Clang
# matrix is part of test: the test programs run with both compilers.
CI_CHECKS = lint test build portstd-check

# In "make ci", a C++ compiler that isn't installed fails the test programs
# (outside it, they skip that compiler). GOCPP_REQUIRE_CXX= make ci turns
# this off; tests/README.md has the details.
ci: export GOCPP_REQUIRE_CXX ?= g++,clang++

# In "make ci", the debug tests (tests/debug) fail instead of skipping their
# gdb part when gdb is missing or may not trace programs (no ptrace).
# GOCPP_REQUIRE_GDB= make ci turns this off.
ci: export GOCPP_REQUIRE_GDB ?= 1

# In "make ci", the Windows tests of the runtime (runtime/windows_test.go: MinGW-w64 and Wine, which
# the dev shell has on x86-64) fail instead of skipping when either is missing.
# GOCPP_WINDOWS_SSH=user@host runs them on a real Windows PC instead (then this is not needed).
# GOCPP_REQUIRE_WINE= make ci turns this off.
ci: export GOCPP_REQUIRE_WINE ?= 1

# make ci, make test, make quick and make ci-sanitize run the test programs up to
# this phase (GOCPP_TEST_PHASE, tests/README.md), not only up to the last finished
# phase (internal/version): the features of the later phases that are already
# built (goroutines, channels, multi-package programs) have tests, and they must
# stay green. A test or example that still waits for a step of its phase says so
# with a "// pending:" line and is skipped (GOCPP_TEST_PENDING=1 runs it). Raise
# this when a later phase gets its first test. GOCPP_TEST_PHASE= make ci (empty)
# runs only the finished phases. At 10 (steps 10.2 to 10.6, the ! operator) the tests of
# Phase 09 and 10 that wait for unmerged steps carry the line.
CI_TEST_PHASE = 10
test test-programs quick ci-sanitize ci-windows: export GOCPP_TEST_PHASE ?= $(CI_TEST_PHASE)

# The paths "make lint" does not gofmt-check, as find(1) tests joined with -o:
# spikes/ (throwaway code in its own Go module) and every testdata folder
# (test inputs need not be gofmt-clean, for example Go's own parser testdata
# in Phase 01; go build and go vet skip testdata folders too).
LINT_SKIP = -path ./spikes -o -path ./.git -o -name testdata

# staticcheck is not part of lint or ci, because "go run" downloads it the
# first time. v0.7.0 is the newest release that works with Go 1.25.
STATICCHECK ?= $(GO) run honnef.co/go/tools/cmd/staticcheck@v0.7.0

.PHONY: build test lint staticcheck ci clean quick portstd-check

build:
	$(GO) build -o bin/gocpp ./cmd/gocpp

# -timeout 45m: the runtime package builds every C++ test with g++ and clang++
# in three modes, and cmd/gocpp and internal/build compile whole programs; on a
# loaded machine they pass go test's 10 minute default (and, for cmd/gocpp, 30
# minutes) when all packages run at once.
test:
	$(GO) test -timeout 45m ./...

# The test programs alone (tests/ and examples/, see tests/README.md),
# verbose so that every skip says why, and never from go test's cache.
# "make test" runs them too, as part of go test ./....
.PHONY: test-programs
test-programs:
	$(GO) test -count=1 -v ./tests/...

# The sanitizer job (Phase 03 exit criteria): the test programs that run code
# (tests/run, tests/diff, tests/interop, tests/zoo and examples/) with gocpp run -asan
# (AddressSanitizer, LeakSanitizer and UndefinedBehaviorSanitizer; a test
# marked "// lsan: cycle" leaks a reference cycle on purpose and runs without
# LeakSanitizer), then the emitter's tests, which build and run programs
# (internal/cppgen), with the same sanitizers in GOCPP_TEST_CXXFLAGS; with
# both g++ and clang++. internal/cppgen's TestFatal skips itself here (a
# sanitizer reports running out of memory and stack overflows on its own, and
# the runtime's stack-overflow handler is off under ASan). The runtime's own
# C++ tests have a sanitizer mode in make test already. -parallel 4 and 2, because
# sanitized builds and programs are big. The repo has no CI service yet
# (TODO.md, Q9), so this target is the CI job: run it next to make ci when a
# change touches the runtime, the emitter or the tests. It runs the tests up to
# CI_TEST_PHASE, like make ci; GOCPP_TEST_PHASE=09 make ci-sanitize runs the tests
# of a phase that is being built. Time (2026-10-18, shared 128-core machine at load 30-60): the
# test programs, all phases up to 08 (about 1250 programs on two compilers), took about 20 minutes
# at -parallel 4 and the cppgen tests about 28 minutes at -parallel 2 with the sanitizers.
SANITIZE_FLAGS = -fsanitize=address,undefined -fno-omit-frame-pointer -fno-sanitize-recover=all
.PHONY: ci-sanitize
ci-sanitize: export GOCPP_REQUIRE_CXX ?= g++,clang++
ci-sanitize:
	GOCPP_FLAGS="$(GOCPP_FLAGS) -asan" $(GO) test -count=1 -timeout 120m -parallel 4 \
		-run '^(TestRun|TestDiff|TestInterop|TestZoo|TestExamples)$$' ./tests/...
	GOCPP_TEST_CXXFLAGS="$(GOCPP_TEST_CXXFLAGS) $(SANITIZE_FLAGS)" \
		$(GO) test -count=1 -timeout 60m -parallel 2 ./internal/cppgen/...
	@echo "make ci-sanitize: all checks passed"

# The Windows job (Phase 06 step 6.13, tests/README.md "Windows under Wine"): the run tests of
# tests/windows_sample.txt and the programs of tests/testdata/windows, built with gocpp build -os
# windows and run under Wine in an isolated prefix, plus the canary that Wine leaves the real home
# alone. Needs wine and a MinGW-w64 compiler (the dev shell has both; GOCPP_REQUIRE_WINE=1 makes a
# missing one a failure). GOCPP_TEST_WINDOWS=all make ci-windows runs every OS-independent run test.
.PHONY: ci-windows
ci-windows: export GOCPP_REQUIRE_WINE ?= 1
ci-windows: export GOCPP_TEST_TARGET = windows
ci-windows:
	$(GO) test -count=1 -timeout 60m -parallel 2 -run '^(TestRunWindows|TestWindowsPrograms|TestWineIsolation)$$' ./tests
	@echo "make ci-windows: all checks passed"

# Fails if gofmt lists a Go file outside LINT_SKIP, then runs go vet, also for
# windows and darwin (a file with a build tag that only one system compiles).
lint:
	@bad=$$(find . \( $(LINT_SKIP) \) -prune -o -type f -name '*.go' -exec $(GOFMT) -l {} +) || exit 1; \
	if [ -n "$$bad" ]; then \
		echo "gofmt: these files need formatting (gofmt -w):"; echo "$$bad"; exit 1; \
	fi
	$(GO) vet ./...
	GOOS=windows $(GO) vet ./...
	GOOS=darwin $(GO) vet ./...

staticcheck:
	$(STATICCHECK) ./...

# The ported packages of stdlib/ are what tools/portstd writes (Phase 09 step 9.1): a hand edit of a
# ported file fails it. The tool reads the pinned Go source from its cache ($XDG_CACHE_HOME/gocpp/portstd)
# and never downloads here. Without the tarball it still compares the files with the SHA-256 of
# stdlib/PORTED.sha256 (a hand edit is found by hash) and says that the byte comparison was skipped.
# GOCPP_REQUIRE_PORTSTD=1 make ci (or make portstd-check) turns the missing tarball into an error:
# set it where the cache is filled ("go run ./tools/portstd -check -fetch" downloads it once).
portstd-check:
	$(GO) run ./tools/portstd -check

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

# The language server benchmarks (Phase 11 step 11.11, tools/lspbench): the
# reference projects are generated (never checked in), the scenarios are
# replayed against gocpp lsp and, when gopls is on the PATH, against gopls on
# the same code, and the table goes to docs/benchmarks/lsp.md (the raw figures
# to lsp.json). Run it alone, on a quiet machine (it refuses to start when the
# load average is over 30; LSP_BENCH_FLAGS="-max-load 0" overrides, for
# development; "-runs 1 -scale 0.2" is a quick look). lsp-budgets is the gate
# on the speed budgets (a Go test that is opt-in, so not part of make ci: it
# takes a minute and a half and skips itself on a loaded machine).
LSP_BENCH_PROJECTS ?= small,medium,large,cpp
.PHONY: lsp-bench lsp-budgets
lsp-bench:
	$(GO) build -o bin/gocpp ./cmd/gocpp
	nice -n 10 $(GO) run ./tools/lspbench -gocpp bin/gocpp -projects $(LSP_BENCH_PROJECTS) \
		-json docs/benchmarks/lsp.json -md docs/benchmarks/lsp.md $(LSP_BENCH_FLAGS)

lsp-budgets:
	GOCPP_LSP_BUDGETS=on $(GO) test -count=1 -v -run TestLSPBudgets ./tools/lspbench

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
