package interop

import (
	"os"
	"testing"

	"github.com/huuhait/gocpp/internal/racetest"
)

// A fake compiler that must crash with a signal (a probe's compiler that is killed by SEGV) runs
// the test binary again as its program (crashBody): TestMain makes it die at once, of the signal
// with the default action, without a core dump. A shell script that kills itself with
// `kill -SEGV $$` leaves a core dump, and with a core_pattern that pipes to systemd-coredump
// (RLIMIT_CORE 0 does not stop that) the dying process waits for the handler, for minutes under
// load. PR_SET_DUMPABLE 0 does stop it, but a shell can't set it.
func TestMain(m *testing.M) {
	crashIfAsked()
	racetest.ExitChildrenQuickly() // the processes of TestDiskCacheProcesses re-run this binary
	os.Exit(m.Run())
}
