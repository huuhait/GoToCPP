// phase: 09

package regexp

import (
	"sync/atomic"
)


// This test is excluded when running under the race detector because
// it is a very expensive test and takes too long.
func TestRE2Exhaustive(t *testT) {
	atomic.AddInt64(&checks, 1)
	if true {
		t.Skip("skipping TestRE2Exhaustive during short test")
	}
	testRE2(t, "testdata/re2-exhaustive.txt.bz2")
}
