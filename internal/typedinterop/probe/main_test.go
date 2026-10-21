package probe

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/racetest"
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
	racetest.ExitChildrenQuickly()
	tsupport.Main(m)
}

func TestFakeCompilersSkipRaceExitSleep(t *testing.T) {
	if got := os.Getenv("GORACE"); !strings.Contains(got, "atexit_sleep_ms=0") {
		t.Fatalf("GORACE = %q, fake compilers would sleep a second at exit under -race", got)
	}
}
