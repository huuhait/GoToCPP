package main

import (
	"bytes"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseTestArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		pkgs     []string
		binArgs  []string
		extra    []string
		timeout  time.Duration
		v, c     bool
		out      string
		wantCode int // 0: ok
	}{
		{name: "none", timeout: 10 * time.Minute},
		{name: "package", args: []string{"./a", "./b"}, pkgs: []string{"./a", "./b"}, timeout: 10 * time.Minute},
		{name: "v and run", args: []string{"-v", "-run", "TestA|B", "."}, pkgs: []string{"."},
			binArgs: []string{"-test.v=true", "-test.run=TestA|B"}, v: true, timeout: 10 * time.Minute},
		{name: "equals", args: []string{"-run=X", "-count=2", "-short"}, binArgs: []string{"-test.run=X", "-test.count=2", "-test.short=true"},
			timeout: 10 * time.Minute},
		{name: "double dash and test prefix", args: []string{"--run", "X", "-test.v"}, binArgs: []string{"-test.run=X", "-test.v=true"},
			v: true, timeout: 10 * time.Minute},
		{name: "flags after packages", args: []string{"./a", "-bench", ".", "./b", "-benchtime=10x"}, pkgs: []string{"./a", "./b"},
			binArgs: []string{"-test.bench=.", "-test.benchtime=10x"}, timeout: 10 * time.Minute},
		{name: "timeout", args: []string{"-timeout", "30s"}, binArgs: []string{"-test.timeout=30s"}, timeout: 30 * time.Second},
		{name: "bool with value", args: []string{"-v=false", "-failfast=true"}, binArgs: []string{"-test.v=false", "-test.failfast=true"},
			timeout: 10 * time.Minute},
		{name: "args ends the flags", args: []string{".", "-args", "-v", "x", "-run"}, pkgs: []string{"."}, extra: []string{"-v", "x", "-run"},
			timeout: 10 * time.Minute},
		{name: "unknown flags go to the binary", args: []string{"-mystery", "."}, pkgs: []string{"."}, binArgs: []string{"-mystery"},
			timeout: 10 * time.Minute},
		{name: "compile only", args: []string{"-c", "-o", "x.test", "."}, pkgs: []string{"."}, c: true, out: "x.test", timeout: 10 * time.Minute},
		{name: "build flags", args: []string{"-asan", "-nochecks", "-x", "."}, pkgs: []string{"."}, timeout: 10 * time.Minute},
		{name: "vet is accepted", args: []string{"-vet=off", "."}, pkgs: []string{"."}, timeout: 10 * time.Minute},
		{name: "v is the test flag, not the build flag", args: []string{"-v", "."}, pkgs: []string{"."}, binArgs: []string{"-test.v=true"}, v: true, timeout: 10 * time.Minute},
		{name: "missing value", args: []string{"-run"}, wantCode: 2},
		{name: "bad timeout", args: []string{"-timeout", "soon"}, wantCode: 2},
		{name: "json", args: []string{"-json"}, wantCode: 2},
		{name: "race", args: []string{"-race", "."}, wantCode: 2},
		{name: "cover", args: []string{"-cover"}, wantCode: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			o, code := parseTestArgs(tt.args, &stderr)
			if tt.wantCode != 0 {
				if o != nil || code != tt.wantCode {
					t.Fatalf("parseTestArgs = %v, %d; want nil, %d", o, code, tt.wantCode)
				}
				if stderr.Len() == 0 {
					t.Error("no message")
				}
				return
			}
			if o == nil {
				t.Fatalf("parseTestArgs failed with %d: %s", code, stderr.String())
			}
			if !reflect.DeepEqual(o.pkgs, tt.pkgs) {
				t.Errorf("pkgs = %q, want %q", o.pkgs, tt.pkgs)
			}
			if !reflect.DeepEqual(o.binArgs, tt.binArgs) {
				t.Errorf("binArgs = %q, want %q", o.binArgs, tt.binArgs)
			}
			if !reflect.DeepEqual(o.args, tt.extra) {
				t.Errorf("args = %q, want %q", o.args, tt.extra)
			}
			if o.timeout != tt.timeout || o.v != tt.v || o.c != tt.c || o.out != tt.out {
				t.Errorf("timeout %v v %v c %v out %q; want %v %v %v %q", o.timeout, o.v, o.c, o.out, tt.timeout, tt.v, tt.c, tt.out)
			}
		})
	}
}

func TestParseTestArgsBuildFlags(t *testing.T) {
	o, _ := parseTestArgs([]string{"-asan", "-nochecks", "-x", "-cpperrors", "full", "-work", "."}, new(bytes.Buffer))
	if o == nil || !o.build.asan || !o.build.nochecks || !o.build.x || !o.build.work || o.build.cppErrors != "full" {
		t.Fatalf("build flags = %+v", o)
	}
}

func TestParseTestArgsListAndBench(t *testing.T) {
	o, _ := parseTestArgs([]string{"-list", "Test", "-bench", "."}, new(bytes.Buffer))
	if o == nil || !o.list || !o.bench {
		t.Fatalf("options = %+v", o)
	}
	o, _ = parseTestArgs([]string{"-list="}, new(bytes.Buffer))
	if o == nil || o.list {
		t.Fatalf("an empty -list is no list: %+v", o)
	}
}

// exitError returns the error of a command that exited with status 1.
func exitError(t *testing.T) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit 1").Run()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Skipf("no sh: %v", err)
	}
	return err
}

func TestWriteTestResult(t *testing.T) {
	failure := exitError(t)
	tests := []struct {
		name string
		err  error
		data string
		kill string
		want string
	}{
		{name: "ok", want: "ok  \texample.com/m\t0.003s\n"},
		{name: "ok with output", data: "PASS\n", want: "ok  \texample.com/m\t0.003s\n"},
		{name: "ok output without newline", data: "PASS", want: "\nok  \texample.com/m\t0.003s\n"},
		{name: "no tests to run", data: "testing: warning: no tests to run\nPASS\n", want: "ok  \texample.com/m\t0.003s [no tests to run]\n"},
		{name: "no tests to run later", data: "x\ntesting: warning: no tests to run\nPASS\n", want: "ok  \texample.com/m\t0.003s [no tests to run]\n"},
		{name: "fail with output", err: failure, data: "--- FAIL: TestX (0.00s)\nFAIL\n", want: "FAIL\texample.com/m\t0.003s\n"},
		{name: "fail output without newline", err: failure, data: "FAIL", want: "\nFAIL\texample.com/m\t0.003s\n"},
		{name: "fail without output", err: failure, want: "exit status 1\nFAIL\texample.com/m\t0.003s\n"},
		{name: "killed", err: failure, data: "x\n", kill: "quit", want: "*** Test killed with quit: ran too long (11m0s).\nFAIL\texample.com/m\t0.003s\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			writeTestResult(&out, "example.com/m", "0.003s", tt.err, []byte(tt.data), tt.kill, 11*time.Minute)
			if out.String() != tt.want {
				t.Errorf("wrote %q, want %q", out.String(), tt.want)
			}
		})
	}
	// A program that did not exit (killed by a signal) prints the error even with output.
	t.Run("signal", func(t *testing.T) {
		err := exec.Command("sh", "-c", "kill -9 $$").Run()
		if err == nil {
			t.Skip("no signal")
		}
		var out bytes.Buffer
		writeTestResult(&out, "p", "1.000s", err, []byte("partial\n"), "", 0)
		if !strings.Contains(out.String(), "signal: killed\nFAIL\tp\t1.000s\n") {
			t.Errorf("wrote %q", out.String())
		}
	})
}
