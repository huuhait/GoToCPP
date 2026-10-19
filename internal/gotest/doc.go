// Package gotest finds the tests of a package and writes the main package of its test binary, for
// "gocpp test" (Phase 09 step 9.9; docs/guides/testing.md).
//
// Like go test, gocpp test builds one program for each package under test: the package with its
// _test.gp files (the test variant), its external test package (package x_test), and a generated main
// package that registers the tests with the standard package testing:
//
//	var tests = []testing.InternalTest{{"TestFoo", _test.TestFoo}}
//	func main() { m := testing.MainStart(testdeps.TestDeps{}, tests, ...); m.Run(); testdeps.Exit() }
//
// Scan reads the declarations of the test files (no type checking: the signatures are checked
// by their syntax, as go test does), and Main writes the source of the main package. The loader
// (load.Config.TestMain) loads it in the import graph of the test.
package gotest
