package format

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/diff"
)

// The tests in this file and in stdlib_test.go compare Source with the
// real gofmt: the one of the Go that runs the tests.

// goroot returns the GOROOT of the go command in PATH, like the round-trip
// test of internal/syntax: go test puts $GOROOT/bin first in PATH, so it
// is the Go that runs the test.
func goroot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			t.Fatalf("go env GOROOT: %v\n%s", err, exit.Stderr)
		}
		t.Fatalf("go env GOROOT: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		t.Fatal("go env GOROOT printed nothing")
	}
	return dir
}

// realGofmt returns the real gofmt, $GOROOT/bin/gofmt, or "" if it is not
// there. Every Go install has it.
func realGofmt(t *testing.T) string {
	t.Helper()
	name := filepath.Join(goroot(t), "bin", "gofmt")
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if _, err := os.Stat(name); err != nil {
		return ""
	}
	return name
}

// runGofmt runs gofmt with args and returns what it prints and its exit
// code. It fails the test if gofmt can't be started.
func runGofmt(t *testing.T, gofmt string, args ...string) (stdout, stderr []byte, code int) {
	t.Helper()
	stdout, stderr, code, err := gofmtRun(gofmt, args...)
	if err != nil {
		t.Fatalf("%s: %v", gofmt, err)
	}
	return stdout, stderr, code
}

// gofmtRun is runGofmt for a goroutine that is not the test's: it returns
// the error if gofmt can't be started.
func gofmtRun(gofmt string, args ...string) (stdout, stderr []byte, code int, err error) {
	var out, errOut bytes.Buffer
	cmd := exec.Command(gofmt, args...)
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code, err = exit.ExitCode(), nil
	}
	return out.Bytes(), errOut.Bytes(), code, err
}

// readFile returns the contents of the file name, or fails the test.
func readFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// showDiff returns a diff from want to got, for a failure message.
func showDiff(want, got []byte) string {
	return string(diff.Diff("want", want, "got", got))
}

// TestGofmtInputs formats the badly formatted Go files in testdata/gofmt
// (*.input). The result must be what the real gofmt prints for them: the
// .golden file of the same name, which mkgolden.sh makes with the real
// gofmt. When the real gofmt is there, the test also runs it, so a
// .golden file that is out of date fails the test too.
func TestGofmtInputs(t *testing.T) {
	inputs, err := filepath.Glob("testdata/gofmt/*.input")
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no testdata/gofmt/*.input files (%v)", err)
	}
	gofmt := realGofmt(t)
	if gofmt == "" {
		t.Log("there is no real gofmt: the test compares with the .golden files only")
	}
	for _, in := range inputs {
		golden := strings.TrimSuffix(in, ".input") + ".golden"
		t.Run(filepath.Base(in), func(t *testing.T) {
			src, want := readFile(t, in), readFile(t, golden)
			got, err := Source(in, src)
			if err != nil {
				t.Fatalf("Source: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("Source gives a result that is not %s:\n%s", golden, showDiff(want, got))
			}
			// gofmt's output is gofmt-formatted.
			if again, err := Source(golden, want); err != nil || !bytes.Equal(again, want) {
				t.Errorf("Source changes %s (error %v):\n%s", golden, err, showDiff(want, again))
			}
			if gofmt == "" {
				return
			}
			out, errOut, code := runGofmt(t, gofmt, in)
			if code != 0 {
				t.Fatalf("gofmt %s: exit status %d\n%s", in, code, errOut)
			}
			if !bytes.Equal(out, want) {
				t.Errorf("%s is not what gofmt prints now; run internal/format/mkgolden.sh:\n%s", golden, showDiff(want, out))
			}
		})
	}
}

// TestGofmtTestdata runs the test cases of gofmt itself, from
// $GOROOT/src/cmd/gofmt/testdata: each .input file must give its .golden
// file. It leaves out the cases for gofmt's flags -r and -s, which gocpp
// fmt doesn't have, and it formats the -stdin cases with Fragment, as
// gofmt's test does. Like gofmt's test, it also checks that the .golden
// file stays as it is.
func TestGofmtTestdata(t *testing.T) {
	dir := filepath.Join(goroot(t), "src", "cmd", "gofmt", "testdata")
	inputs, err := filepath.Glob(filepath.Join(dir, "*.input"))
	if err != nil || len(inputs) == 0 {
		t.Skipf("no gofmt test cases in %s (%v)", dir, err)
	}
	ran := 0
	for _, in := range inputs {
		src := readFile(t, in)
		flags := gofmtFlags(src)
		if slices.ContainsFunc(strings.Fields(flags), func(f string) bool { return f == "-s" || strings.HasPrefix(f, "-r") }) {
			continue
		}
		golden := strings.TrimSuffix(in, ".input") + ".golden"
		ran++
		t.Run(filepath.Base(in), func(t *testing.T) {
			formatFunc := Source
			if flags == "-stdin" {
				formatFunc = Fragment
			}
			want := readFile(t, golden)
			for _, file := range []struct {
				name string
				src  []byte
			}{{in, src}, {golden, want}} {
				got, err := formatFunc(file.name, file.src)
				if err != nil {
					t.Fatalf("%s: %v", file.name, err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("%s gives a result that is not %s:\n%s", file.name, golden, showDiff(want, got))
				}
			}
		})
	}
	// Go 1.25.14 has 16 such cases.
	if ran < 10 {
		t.Errorf("ran only %d of gofmt's test cases in %s, want at least 10", ran, dir)
	}
}

// gofmtFlags returns the flags of a gofmt test case: the text after
// "//gofmt " in a comment line in the first 20 lines, like gofmtFlags in
// cmd/gofmt/gofmt_test.go.
func gofmtFlags(src []byte) string {
	lines := strings.SplitN(string(src), "\n", 21)
	for _, line := range lines[:min(len(lines), 20)] {
		if flags, ok := strings.CutPrefix(strings.TrimSpace(line), "//gofmt "); ok {
			return strings.TrimSpace(flags)
		}
	}
	return ""
}
