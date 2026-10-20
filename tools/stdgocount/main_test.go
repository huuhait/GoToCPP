package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/stdcount"
)

func passingCount(pkg string) *stdcount.Count {
	return &stdcount.Count{Package: pkg, Total: 10, Selected: 10, Passed: 10}
}

func TestReportOfCleanPackagesRejectsNothing(t *testing.T) {
	var out, errOut bytes.Buffer
	counts := []*stdcount.Count{passingCount("a"), passingCount("b")}

	rejected := report(&out, &errOut, []string{"a", "b"}, counts, make([]error, 2), false)

	if rejected || errOut.Len() != 0 {
		t.Fatalf("rejected = %v, stderr = %q; want a clean report", rejected, errOut.String())
	}
	if !strings.Contains(out.String(), "| total | 20 | 20 | 20 |") {
		t.Fatalf("table = %s", out.String())
	}
}

func TestReportRejectsPackageThatDidNotRun(t *testing.T) {
	var out, errOut bytes.Buffer
	counts := []*stdcount.Count{passingCount("a"), nil}
	errs := []error{nil, errors.New("gocpp test -c b: exit status 1")}

	rejected := report(&out, &errOut, []string{"a", "b"}, counts, errs, false)

	if !rejected || !strings.Contains(errOut.String(), "b: gocpp test -c b: exit status 1") {
		t.Fatalf("rejected = %v, stderr = %q", rejected, errOut.String())
	}
	if !strings.Contains(out.String(), "| a | 10 |") || strings.Contains(out.String(), "| b |") {
		t.Fatalf("table = %s", out.String())
	}
}

func TestReportRejectsUnexpectedFailure(t *testing.T) {
	var out, errOut bytes.Buffer
	count := passingCount("a")
	count.Problems = []string{"TestX: fail and not expected to (boom)"}

	rejected := report(&out, &errOut, []string{"a"}, []*stdcount.Count{count}, []error{nil}, false)

	want := "a: TestX: fail and not expected to (boom)"
	if !rejected || !strings.Contains(errOut.String(), want) {
		t.Fatalf("rejected = %v, stderr = %q", rejected, errOut.String())
	}
}

func TestReportRejectsPackageUnderBudget(t *testing.T) {
	var out, errOut bytes.Buffer
	count := &stdcount.Count{Package: "a", Total: 10, Selected: 10, Passed: 8, Failed: 2}

	rejected := report(&out, &errOut, []string{"a"}, []*stdcount.Count{count}, []error{nil},
		false)

	if !rejected || !strings.Contains(errOut.String(), "a: 80.0 % pass, under the 90 % budget") {
		t.Fatalf("rejected = %v, stderr = %q", rejected, errOut.String())
	}
}

func TestReportDetailListsFunctionsThatDoNotPass(t *testing.T) {
	var out, errOut bytes.Buffer
	count := passingCount("a")
	count.Rows = []stdcount.Row{
		{Name: "TestBad", File: "a_test.go", Outcome: "fail", Reason: "broken"},
	}

	report(&out, &errOut, []string{"a"}, []*stdcount.Count{count}, []error{nil}, true)

	if !strings.Contains(out.String(), "- TestBad (a_test.go, fail): broken") {
		t.Fatalf("output = %s", out.String())
	}
}
