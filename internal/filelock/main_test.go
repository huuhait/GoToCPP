package filelock

import (
	"os"
	"testing"

	"github.com/huuhait/gocpp/internal/racetest"
)

func TestMain(m *testing.M) {
	racetest.ExitChildrenQuickly() // the lock holder of TestLockAcrossProcesses re-runs this binary
	os.Exit(m.Run())
}
