#!/bin/sh
# clang-matrix.sh [CLANG++ ...]: runs the typed interop tests once for each clang++, the matrix of
# ADR-108 (Phase 12 step 12.12). The probes parse clang's JSON dump, which is not stable across
# majors (risk row of the Phase 12 README), so the tests that talk to a real clang run against each
# version that is installed:
#
#	nix develop -c sh tools/clang-matrix.sh                       # every clang++-17 .. clang++-21 in PATH
#	nix develop -c sh tools/clang-matrix.sh clang++-17 /opt/llvm19/bin/clang++
#
# Without arguments the programs clang++-17 to clang++-21 that are in PATH are used, or clang++ when
# there is none of them. GOCPP_CLANG tells gocpp which clang++ to probe with (probe.FindClang); the
# test helper tsupport.Clang follows the same rule, and TestProbeTestsUseTheClangOfTheMatrix proves
# with a logging wrapper that the probe tests really call the program named. The build compiler stays
# g++ and clang++ as in every test. The dev shell has clang 21 only (the policy: the minimum is 17,
# then the supported majors in between and the newest), so only clang 21 has been run.
#
# The tests: the probe, checker and mapper packages (internal/typedinterop/...), the driver's typed
# tests (cmd/gocpp) and the check of the whole corpus in typed mode (tests, TestTypedCheckCorpus).
# One summary line per version; the exit status is 1 when any version fails. -parallel 4 and
# GOCPP_TEST_TIMEOUT=6m keep a busy machine from failing a test for time.
set -u
GO=${GO:-go}
if [ $# -eq 0 ]; then
	for v in 17 18 19 20 21; do
		if command -v "clang++-$v" >/dev/null 2>&1; then
			set -- "$@" "clang++-$v"
		fi
	done
fi
if [ $# -eq 0 ]; then
	set -- clang++
fi
export GOCPP_TEST_TIMEOUT=${GOCPP_TEST_TIMEOUT:-6m}
failed=0
for prog in "$@"; do
	version=$("$prog" --version 2>/dev/null | head -n 1)
	if [ -z "$version" ]; then
		echo "clang-matrix: $prog: not found or does not run" >&2
		failed=1
		continue
	fi
	major=$(echo "$version" | sed -n 's/.*clang version \([0-9][0-9]*\)\..*/\1/p')
	echo "clang-matrix: $prog ($version)"
	ok=1
	# TestClangInUse and TestMatrixClangReachesTheDriver fail when a test cleared GOCPP_CLANG.
	export GOCPP_EXPECT_CLANG_MAJOR=$major
	GOCPP_CLANG=$prog $GO test -count=1 -timeout 45m -parallel 4 ./internal/typedinterop/... || ok=0
	GOCPP_CLANG=$prog $GO test -count=1 -timeout 45m -parallel 4 -run 'TestTyped|TestMatrix' ./cmd/gocpp || ok=0
	GOCPP_CLANG=$prog GOCPP_TEST_PHASE=${GOCPP_TEST_PHASE:-15} $GO test -count=1 -timeout 45m -parallel 4 \
		-run '^TestTypedCheckCorpus$' ./tests || ok=0
	if [ $ok = 1 ]; then
		echo "clang-matrix: $prog: ok"
	else
		echo "clang-matrix: $prog: FAILED"
		failed=1
	fi
done
exit $failed
