// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import "fmt"

// InternalExample is an internal type but exported because it is cross-package;
// it is part of the implementation of the "gocpp test" command.
type InternalExample struct {
	Name      string
	F         func()
	Output    string
	Unordered bool
}

// RunExamples is an internal function but exported because it is cross-package;
// it is part of the implementation of the "gocpp test" command.
func RunExamples(matchString func(pat, str string) (bool, error), examples []InternalExample) (ok bool) {
	parseFlags()
	_, ok = runExamples(matchString, examples)
	return ok
}

func runExamples(matchString func(pat, str string) (bool, error), examples []InternalExample) (ran, ok bool) {
	ok = true

	m := newMatcher(matchString, matchPattern, "-test.run", skipPattern)

	var eg InternalExample

	for _, eg = range examples {
		_, matched, _ := m.fullName(nil, eg.Name)
		if !matched {
			continue
		}
		ran = true
		if !runExample(eg) {
			ok = false
		}
	}

	return ran, ok
}

// trimmed returns s without leading and trailing white space.
func trimmed(s string) string {
	return trimSpace(s)
}

// sortedLines returns the lines of s, sorted.
func sortedLines(s string) []string {
	lines := splitByte(s, '\n')
	sortStrings(lines)
	return lines
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (eg *InternalExample) processRunResult(stdout string, timeSpent int64, finished bool, recovered any) (passed bool) {
	passed = true
	dstr := fmtDuration(timeSpent)
	var fail string
	got := trimmed(stdout)
	want := trimmed(eg.Output)
	if eg.Unordered {
		if !equalStrings(sortedLines(got), sortedLines(want)) && recovered == nil {
			fail = fmt.Sprintf("got:\n%s\nwant (unordered):\n%s\n", stdout, eg.Output)
		}
	} else {
		if got != want && recovered == nil {
			fail = fmt.Sprintf("got:\n%s\nwant:\n%s\n", got, want)
		}
	}
	if fail != "" || !finished || recovered != nil {
		fmt.Printf("--- FAIL: %s (%s)\n%s", eg.Name, dstr, fail)
		passed = false
	} else if chatty != nil {
		fmt.Printf("--- PASS: %s (%s)\n", eg.Name, dstr)
	}

	if recovered != nil {
		// Propagate the previously recovered result, by panicking.
		panic(recovered)
	} else if !finished {
		panic(errNilPanicOrGoexit)
	}

	return
}

// runExample runs an example and compares what it printed with its output comment. Go swaps
// os.Stdout for a pipe; here the file descriptor of the standard output goes to a temporary file
// (libCaptureStart, libCaptureStop).
func runExample(eg InternalExample) (ok bool) {
	if chatty != nil {
		fmt.Printf("=== RUN   %s\n", eg.Name)
	}

	libCaptureStart()

	finished := false
	start := libNow()

	defer func() {
		timeSpent := libNow() - start

		out := libCaptureStop()

		err := recover()
		ok = eg.processRunResult(out, timeSpent, finished, err)
	}()

	// Run example.
	eg.F()
	finished = true
	return
}
