package interop

import (
	"os"
	"testing"
	"time"
)

// GOCPP_PROBE_TIMEOUT only raises the limit of a probe run: without it, with a shorter or an
// unreadable value the limit is probeTimeout.
func TestProbeLimit(t *testing.T) {
	for _, c := range []struct {
		env  string // "unset": the variable is not there
		want time.Duration
	}{
		{"unset", probeTimeout},
		{"", probeTimeout},
		{"300ms", probeTimeout},
		{"1m", probeTimeout},
		{"5m", 5 * time.Minute},
		{"90s", 90 * time.Second},
		{"garbage", probeTimeout},
		{"-5m", probeTimeout},
	} {
		t.Setenv("GOCPP_PROBE_TIMEOUT", c.env)
		if c.env == "unset" {
			os.Unsetenv("GOCPP_PROBE_TIMEOUT")
		}
		if got := probeLimit(); got != c.want {
			t.Errorf("GOCPP_PROBE_TIMEOUT=%q: probeLimit() = %v, want %v", c.env, got, c.want)
		}
	}
}
