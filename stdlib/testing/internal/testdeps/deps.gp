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
// and the like (a native std::regex bridge until package regexp is ported in step 9.4) and the
// import path of the package under test.
package testdeps

import "errors"

// ImportPath is the import path of the package under test. The generated main package sets it.
var ImportPath string

// TestDeps is an implementation of the testing.testDeps interface,
// suitable for passing to [testing.MainStart].
type TestDeps struct{}

// ImportPath returns the import path of the package under test.
func (TestDeps) ImportPath() string {
	return ImportPath
}

// MatchString reports whether the regular expression pat matches somewhere in str.
func (TestDeps) MatchString(pat, str string) (bool, error) {
	ok, msg := matchString(pat, str)
	if msg != "" {
		return false, errors.New(msg)
	}
	return ok, nil
}

// SetExitCode records the exit code of the test run (Go's TestDeps keeps it for the coverage
// report).
func (TestDeps) SetExitCode(code int) {
	exitCode = code
}

var exitCode int

//gocpp:native gp::testing::match_string
func matchString(pat, str string) (bool, string)

// Exit ends the program with the code that the last [testing.M.Run] recorded (SetExitCode), 0 if
// there was none: the generated main package calls it where Go's calls os.Exit.
func Exit() {
	exit(exitCode)
}

//gocpp:native gp::testing::exit
func exit(code int)
