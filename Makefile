# gocpp Makefile. "make ci" runs every required check (staticcheck is the
# separate "make staticcheck"); run it before handing over a change. The
# recipes are POSIX sh.

GO ?= go
GOFMT ?= gofmt

# The checks "make ci" runs. A later check is added here. The GCC + Clang
# matrix is part of test: the test programs run with both compilers.
CI_CHECKS = lint test build portstd-check bench-smoke

# In "make ci", a C++ compiler that isn't installed fails the test programs
# (outside it, they skip that compiler). GOCPP_REQUIRE_CXX= make ci turns
# this off; tests/README.md has the details.
ci: export GOCPP_REQUIRE_CXX ?= g++,clang++

# In "make ci", the tests of the documentation site that compare programs with the real go command
# (tools/docsite) fail instead of skipping when there is no go command. GOCPP_REQUIRE_GO= make ci turns this off.
ci: export GOCPP_REQUIRE_GO ?= 1

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
# Phase 09 and 10 that wait for unmerged steps carry the line. The ! forms, macros and
# #Derive have no gate since step 10.12 (ADR-095): their tests name the latest gate they
# meet, so those with phase 05 run in a plain `go test ./tests` too.
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

.PHONY: build test lint staticcheck ci clean quick portstd-check conformance conformance-check conformance-report
.PHONY: bench bench-smoke bench-check bench-baseline bench-history

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

# The sanitizer job (Phase 03 exit criteria; the nightly job of step 15.11, make sanitize-nightly, adds
# canaries, UBSan alone, tests/run in full and a bounded leak audit, and runs this target as its deep
# stage): the test programs that run code (tests/run, tests/diff, tests/interop, tests/zoo and examples/) with gocpp run -asan
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

# The race detector job (Phase 08 step 8.12, docs/guides/race.md): the programs of tests/race (a program with
# a data race must be reported with Go's report and exit code 66, one without must be silent) with
# gocpp run -race with both compilers, the concurrency tests of tests/run (every program that starts
# a goroutine or uses a channel or select: the names are listed by tests/race/concurrency.sh) with
# GOCPP_FLAGS=-race, which must all stay silent (a report fails the test, because the test expects no
# stderr), the same programs checked against real Go's race detector (go build -race), and the
# runtime's ThreadSanitizer unit tests. -parallel 2, because a ThreadSanitizer build is big.
# It is a nightly job: make ci runs one compiler's tests/race only (TestRace is part of ./tests).
.PHONY: ci-race
ci-race: export GOCPP_REQUIRE_CXX ?= g++,clang++
ci-race: export GOCPP_TEST_PHASE ?= $(CI_TEST_PHASE)
ci-race: export GOCPP_RACE_COMPILERS = all
ci-race:
	$(GO) test -count=1 -timeout 60m -parallel 2 -run '^TestRace$$' ./tests/...
	GOCPP_RUN_VS_GO=1 $(GO) test -count=1 -timeout 30m -parallel 2 -run '^TestRaceVsGo$$' ./tests/...
	GOCPP_FLAGS="$(GOCPP_FLAGS) -race" $(GO) test -count=1 -timeout 120m -parallel 2 \
		-run "^TestRun/($$(sh tests/race/concurrency.sh))$$" ./tests/...
	$(GO) test -count=1 -timeout 60m -parallel 2 -run 'TSan' ./runtime/...
	$(GO) test -count=1 -timeout 60m -run '^TestRace' ./cmd/gocpp/...
	@echo "make ci-race: all checks passed"

# The nightly sanitizer job (step 15.11, tools/sanitize, docs/guides/sanitizers.md): every sanitizer
# over tests/run with g++ and clang++ (AddressSanitizer, LeakSanitizer and UndefinedBehaviorSanitizer
# with -asan, UBSan alone, ThreadSanitizer with -race over the concurrency programs), the canaries
# (programs that are wrong on purpose: a sanitizer that does not report them is blind and fails the job),
# and the leak audit of tools/sanitize/leaks.txt. Any report fails it. Hours of CPU time: it is not part
# of ci, run it nightly (tools/sanitize/nightly.sh, .github/workflows/sanitizers-nightly.yml) and on a
# shared machine through heavy.sh. It runs every phase of tests/run (not only CI_TEST_PHASE): programs
# above -phase are counted in its output. SANITIZEFLAGS passes flags to the tool: "-shard 0/4", "-j 8",
# "-deep" (adds make ci-sanitize and make ci-race), "-stages asan,leaks", "-cxx g++", "-phase 10".
SANITIZEFLAGS ?=
.PHONY: sanitize-nightly sanitize-smoke sanitize-leaks sanitize-selftest
sanitize-nightly:
	$(GO) run ./tools/sanitize nightly $(SANITIZEFLAGS)

# The same job on the 14 programs of tools/sanitize/sample.txt: about two minutes. It proves that the
# job works on this machine (canaries, leak audit, all three modes, both compilers).
sanitize-smoke:
	$(GO) run ./tools/sanitize nightly -sample $(SANITIZEFLAGS)

# The leak audit alone: every "// lsan: cycle" program with LeakSanitizer ON, under its ceiling.
sanitize-leaks:
	$(GO) run ./tools/sanitize leaks -cxx g++,clang++ $(SANITIZEFLAGS)

# The tests of the job itself with the real sanitizers: injected UBSan, ASan, LeakSanitizer and TSan
# errors must fail the job, the canaries must be reported, the leak audit must fail on a leak on top of a
# cycle and on a stale marker. A few minutes; GOCPP_SANITIZE_E2E_CXX=g++,clang++ runs both compilers.
sanitize-selftest:
	GOCPP_SANITIZE_E2E=1 $(GO) test -count=1 -timeout 60m -v ./tools/sanitize

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

# The fuzzing campaigns (docs/guides/fuzzing.md, tools/fuzz). Not part of ci. fuzz-short runs every
# target for FUZZTIME (a minute); fuzz-long TARGET=Check,Emit TIME=20m runs the named targets.
# At most 4 workers, at the lowest priority; what they find goes to fuzz-findings/ and
# fuzz-findings.md. make fuzz-seeds runs only the seed corpora (what go test does).
FUZZTIME ?= 60s
TIME ?= 20m
.PHONY: fuzz-short fuzz-long fuzz-seeds
fuzz-short:
	$(GO) run ./tools/fuzz/campaign -time $(FUZZTIME)

fuzz-long:
	$(GO) run ./tools/fuzz/campaign -time $(TIME) $(if $(TARGET),-targets $(TARGET))

fuzz-seeds:
	$(GO) test -count=1 -run '^(Fuzz|TestGenerated)' ./tools/fuzz ./cmd/gocpp ./internal/load/gocppmod

ci: $(CI_CHECKS)
	@echo "make ci: all checks passed"

# The benchmark suite (bench/, tools/gobench, docs/benchmarks/README.md): every program in
# bench/programs built by gocpp (g++ and clang++), by real Go and, for the compute kernels, as a
# C++20 twin; outputs compared; times, CPU time and peak memory measured.
#
# bench-smoke: tiny sizes; builds and runs every program with gocpp's O2 flavor (g++ and clang++), Go
# and the twins, and compares the outputs (about 1 minute). Part of make ci: it is the check that
# every benchmark still builds and agrees. The default (-O0) flavor is left out of ci (the test
# suites cover it): make bench-smoke BENCHFLAGS="-flavors default,O2,release" adds it. Its timings
# mean nothing.
#
# bench: the full run (an hour or more; run it on a quiet machine: it prints the load average and
# marks a noisy run). Writes bench/results/ (gitignored), docs/benchmarks/runtime.md and a line of
# bench/history/<machine>.jsonl (commit that line with the change it measures; "make bench-history"
# prints the table). BENCHFLAGS passes flags: BENCHFLAGS="-match '^cpu/' -reps 5".
#
# bench-check: the regression gate. Measures like bench and compares with bench/baseline.json:
# fails (exit 1) when a gocpp time is more than 5 % over the baseline beyond noise and the slowdown
# reproduces in two more measurements; "inconclusive" (exit 0) on a busy machine or another
# machine; BENCHFLAGS=-require-quiet makes that a failure. Not part of ci (timing needs a quiet,
# fixed machine). bench-baseline: measure and write bench/baseline.json (a deliberate act: say why
# in the commit message).
BENCHFLAGS ?=
bench-smoke:
	$(GO) run ./tools/gobench run -smoke -flavors O2 -out= $(BENCHFLAGS)

bench:
	$(GO) run ./tools/gobench run -compile -md docs/benchmarks/runtime.md -history auto $(BENCHFLAGS)

bench-check:
	$(GO) run ./tools/gobench check $(BENCHFLAGS)

bench-baseline:
	$(GO) run ./tools/gobench run -out bench/results/baseline-run.json $(BENCHFLAGS)
	cp bench/results/baseline-run.json bench/baseline.json

bench-history:
	$(GO) run ./tools/benchhist

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

# Go's own test/ folder as a conformance suite (step 15.10, tools/gotest,
# docs/guides/conformance.md). Opt-in and heavy: not part of ci (the tool's
# own tests are, as part of "make test"). conformance-check runs the tests of
# tools/gotest/passing.txt and fails when one of them no longer passes;
# conformance runs every applicable test (GOTESTFLAGS="-shard 0/4 -j 2" splits
# it) and writes $(CONFORMANCE_OUT); conformance-report prints the counts and the
# failure clusters of the results files. GOCPP_OFFLINE=1 makes the tool use
# the cached tarball only (the first run downloads it: go run ./tools/gotest fetch).
CONFORMANCE_OUT ?= conformance-results.json
GOTESTFLAGS ?=

conformance-check:
	$(GO) run ./tools/gotest gate $(GOTESTFLAGS)

conformance:
	$(GO) run ./tools/gotest run -out $(CONFORMANCE_OUT) $(GOTESTFLAGS)

conformance-report:
	$(GO) run ./tools/gotest report $(CONFORMANCE_OUT)

# The packaging checks (step 15.13, docs/guides/packaging.md), not part of ci: the packaging tests
# (go install of a tagged version from a local module proxy, the GoReleaser config, the flake's
# vendorHash stamp), then what the tools that are installed say: goreleaser check and a snapshot
# release into dist/ (nothing is published), and nix build of the flake package (it builds gocpp,
# wraps it with g++ and clang++, and its install check builds and runs a program with both).
.PHONY: package-check
package-check:
	GOCPP_REQUIRE_PACKAGING=1 $(GO) test -count=1 ./internal/packaging
	goreleaser release --snapshot --clean --skip=publish
	nix build .#gocpp --no-link -L

# The documentation site (step 15.15, docs/guides/docs-site.md): docs/ and the standard library
# reference as a static site in site/ (git-ignored), after every link and anchor was checked. No
# network and no dependency (tools/docsite). docs-check only checks. The code of the tour and the
# guides is checked by go test ./tools/docsite, which is part of make test.
.PHONY: docs docs-check clean-docs
docs:
	$(GO) run ./tools/docsite -out site

docs-check:
	$(GO) run ./tools/docsite -check

clean-docs:
	rm -rf site
