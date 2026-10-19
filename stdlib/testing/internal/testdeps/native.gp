// The one native of package testdeps, in a file of its own so that the Go twin of the harness (tests/twin_test.go) can replace it.

package testdeps

// exit flushes C's standard streams and ends the process with the code, like os.Exit.
//
//gocpp:native gp::testing::exit
func exit(code int)
