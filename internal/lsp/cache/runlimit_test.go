package cache

import (
	"os"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The limits of the server's compiler work are raised with GOCPP_PROBE_TIMEOUT in the proportion of
// the probe run's limit (probe.DefaultTimeout, 30 s), and never lowered: without the variable, with
// a shorter or an unreadable value each limit is its own.
func TestRunLimit(t *testing.T) {
	for _, c := range []struct {
		env     string // "unset": the variable is not there
		d, want time.Duration
	}{
		{"unset", 30 * time.Second, 30 * time.Second},
		{"unset", adaptTimeout, adaptTimeout},
		{"", includeDirsTimeout, includeDirsTimeout},
		{"300ms", 30 * time.Second, 30 * time.Second},
		{"300ms", includeDirsTimeout, includeDirsTimeout},
		{"30s", adaptTimeout, adaptTimeout},
		{"5m", 30 * time.Second, 5 * time.Minute},
		{"5m", adaptTimeout, 10 * time.Minute},
		{"5m", includeDirsTimeout, 100 * time.Second},
		{"garbage", 30 * time.Second, 30 * time.Second},
	} {
		t.Setenv(probe.EnvTimeout, c.env)
		if c.env == "unset" {
			os.Unsetenv(probe.EnvTimeout)
		}
		if got := runLimit(c.d); got != c.want {
			t.Errorf("%s=%q: runLimit(%v) = %v, want %v", probe.EnvTimeout, c.env, c.d, got, c.want)
		}
	}
}
