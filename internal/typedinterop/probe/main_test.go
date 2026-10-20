package probe

import (
	"os"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// initialRacyWindow is the window the program starts with, before the tests make it 0.
var initialRacyWindow time.Duration

func TestMain(m *testing.M) {
	if isFakeCompiler() {
		os.Exit(fakeCompiler(os.Args[1:]))
	}
	// A header that a test writes is not racy for the cache (the tests of the rule set the window).
	initialRacyWindow = racyWindow
	racyWindow = 0
	// The tests of the cache count the runs of fake compilers: a precompiled header would be one
	// more (pch_test.go turns it on for the tests that are about it).
	os.Setenv(EnvPCH, "off")
	lockMargin = time.Second
	lockWaitCap = 8 * time.Second
	tsupport.Main(m)
}
