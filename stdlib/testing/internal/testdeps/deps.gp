// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testdeps provides access to dependencies needed by test execution.
//
// This package is imported by the generated main package, which passes
// TestDeps into testing.MainStart. This allows tests to use packages at run time
// without making those packages direct dependencies of package testing.
// Direct dependencies of package testing are harder to write tests for.
//
// In gocpp it holds what package testing cannot do itself: the regular expressions of -test.run
// and the like (package regexp, as in Go) and the import path of the package under test.
package testdeps

import (
	"regexp"
	"sync"
)

// ImportPath is the import path of the package under test. The generated main package sets it.
var ImportPath string

// TestDeps is an implementation of the testing.testDeps interface,
// suitable for passing to [testing.MainStart].
type TestDeps struct{}

// ImportPath returns the import path of the package under test.
func (TestDeps) ImportPath() string {
	return ImportPath
}

// Go keeps the last compiled pattern. gocpp compiles 3 to 16 times slower than Go, and a -run pattern with / alternates between the
// patterns of its elements (every subtest asks for each of them), so the compiled patterns are kept in a small map (at most
// matchCacheSize entries, emptied when it is full), and a pattern that does not compile is kept with its error. The mutex is
// there because gocpp's -test.parallel can match from several goroutines.
const matchCacheSize = 64

type matchEntry struct {
	re  *regexp.Regexp
	err error
}

var matchMu sync.Mutex
var matchCache = map[string]matchEntry{}

// Compiles counts the calls of regexp.Compile that MatchString made (for the test of the cache).
var Compiles int

// MatchString reports whether the regular expression pat matches somewhere in str.
func (TestDeps) MatchString(pat, str string) (result bool, err error) {
	matchMu.Lock()
	e, ok := matchCache[pat]
	if !ok {
		if len(matchCache) >= matchCacheSize {
			clear(matchCache)
		}
		Compiles++
		e.re, e.err = regexp.Compile(pat)
		matchCache[pat] = e
	}
	matchMu.Unlock()
	if e.err != nil {
		return false, e.err
	}
	return e.re.MatchString(str), nil
}

// SetExitCode records the exit code of the test run (Go's TestDeps keeps it for the coverage
// report).
func (TestDeps) SetExitCode(code int) {
	exitCode = code
}

var exitCode int

// Exit ends the program with the code that the last [testing.M.Run] recorded (SetExitCode), 0 if
// there was none: the generated main package calls it where Go's calls os.Exit.
func Exit() {
	exit(exitCode)
}
