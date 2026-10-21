//go:build linux

package rtfiles_test

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestWindowsSleepAccuracy runs test/sleep_test.cpp on a real Windows PC (GOCPP_WINDOWS_SSH) and
// checks that the timed waits of package time are not rounded up to the system timer tick (15.6 ms
// by default; ADR-132). Wine is no judge of this (its timers are not Windows'), so the test is
// skipped without the variable.
//
// The bounds are generous, for a PC that does other things: the median of a wait is at most
// medianLate after the asked time, and the 99th percentile at most p99Late. Without the fix, a
// Sleep(5ms) ends 10 ms late at the median and a Sleep(1ms) 14 ms late, far over both.
func TestWindowsSleepAccuracy(t *testing.T) {
	if os.Getenv("GOCPP_WINDOWS_SSH") == "" {
		t.Skip("needs a real Windows PC (GOCPP_WINDOWS_SSH): Wine's timers are not Windows'")
	}
	const (
		medianLate = 3 * time.Millisecond
		p99Late    = 10 * time.Millisecond
	)
	c := mingw(t)
	e := wineSetup(t)
	rt := extractRuntime(t)
	bins := c.startBuilds(t, rt, t.TempDir(), winExtraFlags, []prog{{src: "test/sleep_test.cpp", weight: 5}})
	r := e.runTimeout(t, 3*time.Minute, bins.bin(t, "test/sleep_test.cpp"), nil)
	if r.code != 0 || len(r.stderr) != 0 || !strings.HasSuffix(string(r.stdout), "done\n") {
		t.Fatalf("exit code %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
	}
	t.Logf("kind want_us runs median_us p99_us max_us (microseconds later than asked)\n%s", r.stdout)

	// The system keeps its idle tick while a process waits for long (the fine ticks are asked for
	// in the last 64 ms of a wait only), and has the fine tick all through a short wait. After
	// every wait has ended nothing holds it.
	for kind, want := range map[string]string{"short": "1", "long": "0", "end": "0"} {
		line := "holders " + kind + " " + want + "\n"
		if !strings.Contains(string(r.stdout), line) {
			t.Errorf("no line %q in the output (waits that hold fine ticks while a goroutine sleeps)", strings.TrimSpace(line))
		}
	}

	seen := 0
	for _, line := range strings.Split(strings.TrimSpace(string(r.stdout)), "\n") {
		f := strings.Fields(line)
		if len(f) != 6 {
			continue
		}
		micros := make([]int64, 5)
		for i := range micros {
			n, err := strconv.ParseInt(f[i+1], 10, 64)
			if err != nil {
				t.Fatalf("line %q: %v", line, err)
			}
			micros[i] = n
		}
		seen++
		name := f[0] + "-" + strconv.FormatInt(micros[0]/1000, 10) + "ms"
		median := time.Duration(micros[2]) * time.Microsecond
		p99 := time.Duration(micros[3]) * time.Microsecond
		t.Run(name, func(t *testing.T) {
			if median > medianLate {
				t.Errorf("median %v later than asked, want at most %v", median, medianLate)
			}
			if p99 > p99Late {
				t.Errorf("99th percentile %v later than asked, want at most %v", p99, p99Late)
			}
		})
	}
	if seen != 11 {
		t.Errorf("%d result lines, want 11:\n%s", seen, r.stdout)
	}
}
