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
// by default; ADR-132, ADR-133). Wine is no judge of this (its timers are not Windows'), so the
// test is skipped without the variable. The PC must have high-resolution timers (Windows 10
// version 1803 or later).
//
// The bounds are generous, for a PC that does other things: the median of a wait is at most
// medianLate after the asked time, and the 99th percentile at most p99Late. Without the fix, a
// Sleep(5ms) ends 10 ms late at the median and a Sleep(1ms) 14 ms late, far over both; with the
// fine ticks of ADR-132 the median is 1 ms and fails the median bound. Measured with the
// high-resolution timers, the median is 0.01 to 0.3 ms and the 99th percentile 0.5 to 2.4 ms.
func TestWindowsSleepAccuracy(t *testing.T) {
	if os.Getenv("GOCPP_WINDOWS_SSH") == "" {
		t.Skip("needs a real Windows PC (GOCPP_WINDOWS_SSH): Wine's timers are not Windows'")
	}
	c := mingw(t)
	e := wineSetup(t)
	rt := extractRuntime(t)
	progs := []prog{{src: "test/sleep_test.cpp", weight: 5}}
	bins := c.startBuilds(t, rt, t.TempDir(), winExtraFlags, progs)
	bin := bins.bin(t, "test/sleep_test.cpp")

	t.Run("precise", func(t *testing.T) {
		r := e.runTimeout(t, 3*time.Minute, bin, nil)
		out := sleepOutput(t, r)
		if !strings.Contains(out, "precise 1\n") {
			t.Fatalf("the PC has no high-resolution timers (precise 0): the fast path is lost, " +
				"or the PC is older than Windows 10 version 1803")
		}
		checkSleepCases(t, out, 800*time.Microsecond, 5*time.Millisecond, true)
		// No wait asks the system for fine ticks.
		checkHolders(t, out, "0", "0", "0")
		checkPreciseCosts(t, out)
	})

	// The fallback of ADR-132, run on this PC by switching the timers off in the program (an
	// argument of the program, not the environment): the bounds are the old ones.
	t.Run("fallback", func(t *testing.T) {
		r := e.runTimeout(t, 3*time.Minute, bin, nil, "fallback")
		out := sleepOutput(t, r)
		if !strings.Contains(out, "precise 0\n") {
			t.Fatalf("the fallback run did not switch the precise timers off:\n%s", out)
		}
		checkSleepCases(t, out, 3*time.Millisecond, 10*time.Millisecond, false)
		checkHolders(t, out, "1", "0", "0")
	})
}

// sleepOutput checks that the program ended well and returns what it printed.
func sleepOutput(t *testing.T, r winRun) string {
	t.Helper()
	if r.code != 0 || len(r.stderr) != 0 || !strings.HasSuffix(string(r.stdout), "done\n") {
		t.Fatalf("exit code %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
	}
	t.Logf("kind want_us runs median_us p99_us max_us (microseconds later than asked)\n%s", r.stdout)
	return string(r.stdout)
}

// checkHolders checks the lines "holders short|long|end N": how many waits held the system's fine
// ticks while a goroutine slept 30 ms (a short wait) and 400 ms (a long one), and after all waits
// ended.
func checkHolders(t *testing.T, out, short, long, end string) {
	t.Helper()
	for kind, want := range map[string]string{"short": short, "long": long, "end": end} {
		line := "holders " + kind + " " + want + "\n"
		if !strings.Contains(out, line) {
			t.Errorf("no line %q in the output (waits that hold fine ticks while a goroutine sleeps)",
				strings.TrimSpace(line))
		}
	}
}

// numbers returns the integers that follow the words of the line of out that starts with prefix
// ("handles before 87 after 87" gives before=87, after=87).
func numbers(t *testing.T, out, prefix string) map[string]int64 {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, prefix+" ") {
			continue
		}
		f := strings.Fields(line)[1:]
		m := make(map[string]int64)
		for i := 0; i+1 < len(f); i++ {
			if n, err := strconv.ParseInt(f[i+1], 10, 64); err == nil {
				m[f[i]] = n
			}
		}
		return m
	}
	t.Errorf("no line starting with %q in the output", prefix)
	return nil
}

// checkSleepCases checks the lines "sleep|timer|ticker want runs median p99 max" against the
// bounds. With precise, the back-to-back sleeps of up to 5 ms must also end on time (the spin at
// the end of the wait), and a ticker of 1 ms must keep its ticks.
func checkSleepCases(t *testing.T, out string, medianLate, p99Late time.Duration, precise bool) {
	t.Helper()
	seen := 0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Fields(line)
		if len(f) != 6 || (f[0] != "sleep" && f[0] != "timer" && f[0] != "ticker") {
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
			// The back-to-back sleeps of up to 5 ms end on time: the timer is set 0.3 ms early
			// and the rest is a spin (measured 10 to 110 us; a bare high-resolution timer is
			// 400 to 550 us late).
			if precise && f[0] == "sleep" && micros[0] <= 5000 && median > 300*time.Microsecond {
				t.Errorf("median %v later than asked, want at most 300us (the spin at the end)", median)
			}
		})
	}
	want := 11
	if !precise {
		want = 3 // the fallback run measures the sleeps only
	}
	if seen != want {
		t.Errorf("%d result lines, want %d:\n%s", seen, want, out)
	}
}

// checkPreciseCosts checks what the precise waits cost: the idle tick, CPU time and handles.
func checkPreciseCosts(t *testing.T, out string) {
	t.Helper()
	t.Run("idle", func(t *testing.T) {
		// A goroutine sleeps 1.5 s. The process must use no CPU for it (the measure includes the
		// program's own probes), and must not hold the system's fine ticks: a plain timed wait of
		// 2 ms still takes a full tick (15 ms), as before the sleep.
		m := numbers(t, out, "idle")
		if m["probe_during_us"] < 5000 {
			t.Errorf("a plain wait of 2 ms took %d us during a sleep (%d us before): the process "+
				"holds fine timer ticks while it sleeps", m["probe_during_us"], m["probe_before_us"])
		}
		if m["cpu_us"] > 30000 {
			t.Errorf("a sleep of 1.5 s used %d us of CPU, want about 0 (at most 30000)", m["cpu_us"])
		}
	})
	t.Run("cpu", func(t *testing.T) {
		// The end of a precise wait is a spin of at most 0.3 ms: a frame-rate sleep (16 ms) costs
		// about 40 us (0.25%) and a sleep of 1 ms about 20 us. The bound is 10 times that.
		for _, line := range strings.Split(out, "\n") {
			f := strings.Fields(line)
			if len(f) != 5 || f[0] != "cpu" {
				continue
			}
			us, _ := strconv.ParseInt(f[2], 10, 64)
			perSleep, _ := strconv.ParseInt(f[4], 10, 64)
			const limit = 400000
			if perSleep > limit {
				t.Errorf("a sleep of %d us used %d ns of CPU, want at most %d", us, perSleep, limit)
			}
		}
	})
	t.Run("loops", func(t *testing.T) {
		// A program that polls with short sleeps must not use a core for it: a loop of 100 us
		// sleeps used 1.9 CPU seconds in 2 s when a wait of up to 0.3 ms was a spin only, and 24
		// goroutines 6.8 s. Measured now: 27 ms, 37 to 57 ms (1 ms sleeps) and 300 ms (24
		// goroutines, 15% of a core); ADR-132: 16, 10 and 106 ms. The loops run 2 s; the bounds
		// are in CPU milliseconds of those 2 s.
		limits := map[string]int64{"sleep100us": 150, "sleep1ms": 150, "sleeps24": 800}
		for name, limit := range limits {
			var cpu int64 = -1
			for _, line := range strings.Split(out, "\n") {
				f := strings.Fields(line)
				if len(f) == 4 && f[0] == "loop" && f[1] == name {
					cpu, _ = strconv.ParseInt(f[3], 10, 64)
				}
			}
			if cpu < 0 {
				t.Errorf("no line %q in the output", "loop "+name)
			} else if cpu > limit {
				t.Errorf("loop %s used %d ms of CPU in 2 s, want at most %d", name, cpu, limit)
			}
		}
	})
	t.Run("handles", func(t *testing.T) {
		// 10000 sleeps and 1000 timers from 8 goroutines: the process's handle count does not
		// grow (the slots of the waits are kept and reused: at most one for each wait that was
		// going on at one time).
		m := numbers(t, out, "handles")
		if m["after"]-m["before"] > 4 {
			t.Errorf("handles grew from %d to %d", m["before"], m["after"])
		}
		if m["slots"] < 1 || m["slots"] > 64 {
			t.Errorf("%d slots made, want 1 to 64", m["slots"])
		}
	})
	t.Run("burst", func(t *testing.T) {
		// 1000 goroutines sleep at once: a slot for each, and when they are over, the idle ones
		// beyond 64 have given their handles back. (The idle threads of the goroutine pool, up
		// to 256, wait with a timeout of 5 s and keep a slot each for a while: 320 are open.)
		m := numbers(t, out, "burst")
		if m["slots_made"] < 500 {
			t.Errorf("only %d slots were made for 1000 sleepers at once", m["slots_made"])
		}
		if m["slots_made"]-m["slots_open_after"] < 500 {
			t.Errorf("%d of %d slots keep their handles after the burst, want most of them closed",
				m["slots_open_after"], m["slots_made"])
		}
		if m["slots_open_late"] > 64+8 {
			t.Errorf("%d slots keep their handles 6 s after the burst, want at most 64 and a few "+
				"that are in use", m["slots_open_late"])
		}
	})
	t.Run("ticker-rate", func(t *testing.T) {
		// A ticker of 1 ms for one second: a tick is dropped when the timer is a full period late.
		// Before ADR-133 about a fifth of them went (805 of 1000; a third in a worse run); the
		// timer of this PC is a millisecond late for about one wait in ten.
		m := numbers(t, out, "ticks")
		if m["1000"] < 750 {
			t.Errorf("got %d of 1000 ticks, want at least 750", m["1000"])
		}
	})
}
