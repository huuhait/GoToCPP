package borrowfuzz

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBorrowFuzzRunner(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		relative bool
		want     string
		fail     bool
	}{
		{name: "successful executions", want: "ptr1 fake++ OK"},
		{name: "invocation relative candidate and work", relative: true, want: "ptr1 fake++ OK"},
		{name: "generation failure", scenario: "generation", want: "GENERATION-FAIL", fail: true},
		{name: "invalid generated reference", scenario: "invalid", want: "REFERENCE-FAIL", fail: true},
		{name: "candidate build failure", scenario: "candidate", want: "CANDIDATE-FAIL", fail: true},
		{name: "matching failed statuses", scenario: "both", want: "REFERENCE-FAIL", fail: true},
		{name: "stdout difference", scenario: "stdout", want: "DIFF", fail: true},
		{name: "stderr difference", scenario: "stderr", want: "DIFF", fail: true},
		{name: "sanitizer on stderr with zero exit", scenario: "asan", want: "SANITIZER", fail: true},
		{name: "sanitizer on stdout with zero exit", scenario: "ubsan", want: "SANITIZER", fail: true},
		{name: "sanitizer with failed execution", scenario: "asan-fail", want: "SANITIZER", fail: true},
		{name: "candidate timeout", scenario: "timeout", want: "CANDIDATE-FAIL", fail: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newRunnerFixture(t)
			candidate := filepath.Join(fixture, "candidate")
			work := filepath.Join(fixture, "evidence")
			if tt.relative {
				candidate = "./candidate"
				work = "evidence"
			}

			output, err := runFixture(t, fixture, candidate, work, tt.scenario, "1")
			if (err != nil) != tt.fail {
				t.Fatalf("error %v, want failure %v:\n%s", err, tt.fail, output)
			}
			if !strings.Contains(output, tt.want) {
				t.Fatalf("missing %q:\n%s", tt.want, output)
			}
			if tt.fail && strings.Contains(output, " fake++ OK") {
				t.Fatalf("failed execution was reported OK:\n%s", output)
			}

			seeds, err := filepath.Glob(filepath.Join(fixture, "evidence", "ptr1.*"))
			if err != nil || len(seeds) != 1 {
				t.Fatalf("seed evidence %v, error %v", seeds, err)
			}
			if _, err := os.Stat(filepath.Join(seeds[0], "generation.status")); err != nil {
				t.Fatalf("generation evidence not retained: %v", err)
			}
			if tt.scenario == "both" {
				if _, err := os.Stat(filepath.Join(seeds[0], "fake++.status")); !os.IsNotExist(err) {
					t.Fatalf("candidate ran after an invalid reference: %v", err)
				}
			}
		})
	}
}

func TestBorrowFuzzRunnerPropagatesWorkerFailure(t *testing.T) {
	fixture := newRunnerFixture(t)
	output, err := runFixture(t, fixture, "./candidate", "evidence", "second-seed", "2")
	if err == nil || !strings.Contains(output, "GENERATION-FAIL") {
		t.Fatalf("parallel worker failure was not propagated: %v\n%s", err, output)
	}
	if !strings.Contains(output, "ptr1 fake++ OK") {
		t.Fatalf("successful sibling seed did not finish:\n%s", output)
	}
}

func newRunnerFixture(t *testing.T) string {
	t.Helper()
	fixture := t.TempDir()
	bin := filepath.Join(fixture, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"bin/python3": `case "$FUZZ_CASE" in
  generation)
    printf 'generator failed\n' >&2
    exit 7
    ;;
  second-seed)
    if [ "$3" = 2 ]; then
      exit 8
    fi
    ;;
esac
printf 'package main\n'
`,
		"bin/go": `case "$FUZZ_CASE" in
  invalid|both)
    printf 'invalid reference source\n' >&2
    exit 1
    ;;
esac
printf 'expected\n'
`,
		"candidate": `case "$FUZZ_CASE" in
  candidate|both)
    printf 'candidate build failed\n' >&2
    exit 1
    ;;
  stdout)
    printf 'different\n'
    exit 0
    ;;
  stderr)
    printf 'unexpected diagnostic\n' >&2
    ;;
  asan)
    printf 'ERROR: AddressSanitizer: heap-use-after-free\n' >&2
    ;;
  ubsan)
    printf 'runtime error: invalid access\n'
    ;;
  asan-fail)
    printf 'ERROR: AddressSanitizer: heap-use-after-free\n' >&2
    exit 1
    ;;
  timeout)
    sleep 5
    ;;
esac
printf 'expected\n'
`,
	}
	for name, script := range files {
		path := filepath.Join(fixture, name)
		if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\nset -eu\n"+script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return fixture
}

func runFixture(t *testing.T, fixture, candidate, work, scenario, last string) (string, error) {
	t.Helper()
	script, err := filepath.Abs("run.sh")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "bash", script, "ptr", "1", last, candidate)
	command.Dir = fixture
	command.Env = append(os.Environ(),
		"PATH="+filepath.Join(fixture, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"BORROWFUZZ_WORK="+work,
		"BORROWFUZZ_JOBS=2",
		"BORROWFUZZ_N=1",
		"BORROWFUZZ_CXX=fake++",
		"BORROWFUZZ_TIMEOUT=1s",
		"FUZZ_CASE="+scenario,
	)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("synthetic harness exceeded its deadline: %v\n%s", ctx.Err(), output)
	}
	return string(output), err
}
