package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

// sample is one measured run of one program.
type sample struct {
	WallNS      int64 `json:"wall_ns"`
	CPUNS       int64 `json:"cpu_ns"`
	RSSKB       int64 `json:"rss_kb"`        // the peak resident set used: rusage's, or the sampled VmHWM when rusage only shows the runner's floor
	RusageRSSKB int64 `json:"rusage_rss_kb"` // wait4's ru_maxrss as it is (it also includes the runner's own high-water mark at the exec)
	Exit        int   `json:"exit"`
}

// outputLimit is how much of a program's combined output is kept. The programs
// print a few lines; the limit only stops a broken one from filling memory.
const outputLimit = 64 << 10

// runTimed runs argv with env (nil: the runner's) and measures it: wall time,
// user+system CPU time and the peak resident set from the kernel's accounting
// of the child (wait4's rusage), plus a sampled VmHWM for runs whose rusage
// figure is only the runner's own floor. The combined stdout and stderr (at
// most outputLimit bytes) are returned. A non-zero exit status or a timeout is
// an error (the sample and the output are still filled).
func runTimed(argv []string, env []string, timeout time.Duration) (sample, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = env
	out := &limitedBuffer{max: outputLimit}
	cmd.Stdout, cmd.Stderr = out, out
	setupChild(cmd)
	runnerHWM := readVmHWM(os.Getpid())
	start := time.Now()
	err := cmd.Start()
	var peak atomic.Int64
	done := make(chan struct{})
	sampled := make(chan struct{})
	if err == nil {
		go func() {
			defer close(sampled)
			samplePeakRSS(cmd.Process.Pid, done, &peak)
		}()
		err = cmd.Wait()
	} else {
		close(sampled)
	}
	wall := time.Since(start)
	close(done)
	<-sampled
	s := sample{WallNS: int64(wall)}
	if cmd.ProcessState != nil {
		cpu, rusageRSS := usage(cmd.ProcessState)
		s.CPUNS, s.RusageRSSKB = int64(cpu), rusageRSS
		s.RSSKB = chooseRSS(rusageRSS, runnerHWM, peak.Load(), wall)
		s.Exit = cmd.ProcessState.ExitCode()
	}
	text := strings.TrimSpace(out.String())
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return s, text, fmt.Errorf("timeout after %v", timeout)
		}
		return s, text, fmt.Errorf("exit status %d: %s", s.Exit, firstLine(text))
	}
	return s, text, nil
}

// limitedBuffer keeps the first max bytes written to it and counts the rest
// away.
type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (w *limitedBuffer) Write(p []byte) (int, error) {
	if room := w.max - w.buf.Len(); room > 0 {
		w.buf.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

func (w *limitedBuffer) String() string { return w.buf.String() }

// minRSSWall is the shortest run whose sampled resident set is believed: a
// shorter run ends before the 2 ms sampler sees its real peak.
const minRSSWall = 20 * time.Millisecond

// chooseRSS picks the peak resident set (KiB) of a finished child:
//   - rusageRSS is wait4's ru_maxrss, which is the larger of the child's own
//     peak and the runner's own high-water mark at the exec (the child shares
//     the runner's address space until then). When it is above runnerHWM, it is
//     the child's peak, exactly.
//   - otherwise the child's peak is below that floor and rusage says nothing:
//     the sampled VmHWM is used, but only for a run of at least minRSSWall.
//
// Without /proc (runnerHWM and sampled 0) rusage is all there is.
func chooseRSS(rusageRSS, runnerHWM, sampled int64, wall time.Duration) int64 {
	switch {
	case rusageRSS > runnerHWM:
		return rusageRSS
	case sampled > 0 && wall >= minRSSWall:
		return sampled
	}
	return 0
}

// samplePeakRSS stores into peak the largest VmHWM (KiB) it reads from
// /proc/<pid>/status until done is closed. VmHWM belongs to the child's own
// image and only grows, so the last reading before it exits is its peak.
func samplePeakRSS(pid int, done <-chan struct{}, peak *atomic.Int64) {
	for {
		if v := readVmHWM(pid); v > peak.Load() {
			peak.Store(v)
		}
		select {
		case <-done:
			return
		case <-time.After(2 * time.Millisecond):
		}
	}
}

// firstLine returns the first line of s, cut to 200 bytes.
func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
