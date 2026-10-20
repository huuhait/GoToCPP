// Package goexperiment stands in for internal/goexperiment in Go's tests of the standard packages (step 9.10): the experiments of Go's build that a
// test names. gocpp has none of them, and its packages are Go's without the experiments (docs/guides/porting.md).
package goexperiment

// JSONv2 is the experiment of the new encoding/json (off).
const JSONv2 = false

// SynctestFlag is the experiment of testing/synctest (off).
const SynctestFlag = false
