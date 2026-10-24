package server

import (
	"os"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// hangLimit is how long a test waits for an event before it fails: it only turns a hang into a
// failure, and no answer depends on it. A wait often covers real clang++ runs, which take a second
// on a quiet machine and most of a minute on one that is ten times oversubscribed
// (TestForeignHoverMember waited 47 s for its member answer with 240 busy loops on 24 processors),
// so the limit is five minutes.
const hangLimit = 5 * time.Minute

// TestMain raises the time limit of a compiler run (probe.EnvTimeout, which the server's other
// limits of compiler work follow): a clang++ run takes a second on a quiet machine and minutes
// on a full one, and a test must not fail for that. The tests of a time limit set their own.
func TestMain(m *testing.M) {
	if os.Getenv(probe.EnvTimeout) == "" {
		os.Setenv(probe.EnvTimeout, "10m") // (longer than hangLimit: the test fails first)
	}
	os.Exit(m.Run())
}
