package rtfiles_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The cases of test/thread_test.cpp that end the process (Phase 08, step 8.15): stack overflows
// on the main thread and on the stacks of threads that gp::os::thread_start starts, the hook of
// the OS layer, and the faults that must stay crashes. Each is run with the case name as the
// only argument. The helpers here (caseRun, overflowRx, parseHex) are used by the goroutine
// tests too (goroutine_test.go).

// hex is a number the way the stack overflow lines print it.
const hex = `0x[0-9a-f]+`

// overflowRx is Go's report of a stack overflow, up to its last line, with the stack of the
// thread in the second one: the stack pointer, and the bounds [lo, hi); then Go's blank line and
// "goroutine N [running]:" (without a trace) for a thread that has a goroutine record.
var overflowRx = regexp.MustCompile(`^(?:main runs\n|thread runs\n|goroutine runs\n)*` +
	`runtime: goroutine stack exceeds 1000000000-byte limit\n` +
	`runtime: sp=(` + hex + `) stack=\[(` + hex + `), (` + hex + `)\]\n` +
	`fatal error: stack overflow\n(?:\ngoroutine (\d+) \[running\]:\n)?$`)

// caseRun runs case name of the test program bin and returns what it printed (stdout and stderr
// in one stream), the exit code (-1 when a signal killed it) and the error that says which
// signal.
func caseRun(t *testing.T, bin, name string) (out string, code int, sig string) {
	t.Helper()
	prog, args := bin, []string{name}
	if lowLimitCases[name] && runtime.GOOS == "linux" {
		// The runtime raises the main thread's soft RLIMIT_STACK to 1 GiB (at most the hard
		// limit): a hard limit of 64 MiB keeps an endless recursion on the main thread from touching
		// 1 GiB. mainLimitCase checks the 1 GiB limit without a recursion.
		prog, args = "sh", []string{"-c", `ulimit -S -s 8192 && ulimit -H -s 65536 && exec "$0" "$1"`, bin, name}
	}
	b, _, code, err := command(filepath.Dir(bin), nil, true, prog, args...)
	if err != nil {
		if code != -1 {
			t.Fatalf("%s: %v", name, err)
		}
		return string(b), -1, err.Error()
	}
	return string(b), code, ""
}

// lowLimitCases are the cases that recurse without end on the main thread: caseRun runs them under
// a hard stack limit of 64 MiB.
var lowLimitCases = map[string]bool{"overflow-main": true, "overflow-hook-main": true}

// parseHex returns the number of a hex match.
func parseHex(t *testing.T, s string) uint64 {
	t.Helper()
	v, err := strconv.ParseUint(strings.TrimPrefix(s, "0x"), 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// overflowCase runs the case name, a stack overflow, and checks Go's report and exit code 2: the
// stack in the report must be at least minStack bytes with sp near its end. In the asan mode
// the runtime's handler is not installed: the sanitizer's report instead.
func overflowCase(t *testing.T, bin, name string, minStack uint64, asan bool) {
	t.Run(name, func(t *testing.T) {
		out, code, sig := caseRun(t, bin, name)
		if asan {
			if code == 0 || code == 2 && strings.Contains(out, "fatal error: stack overflow") ||
				!strings.Contains(out, "Sanitizer") || !strings.Contains(out, "stack-overflow") && !strings.Contains(out, "SEGV") {
				t.Errorf("%s: exit code %d %s, output:\n%.2000s\nwant the sanitizer's report of the overflow and a nonzero exit code", name, code, sig, out)
			}
			return
		}
		m := overflowRx.FindStringSubmatch(out)
		if code != 2 || sig != "" || m == nil {
			t.Fatalf("%s: exit code %d %s, output:\n%s\nwant Go's stack overflow report and exit code 2", name, code, sig, out)
		}
		sp, lo, hi := parseHex(t, m[1]), parseHex(t, m[2]), parseHex(t, m[3])
		// The goroutine's number: the main goroutine's is 1; the goroutines of the tests
		// (names with "-go") have a higher one; a plain thread of thread_start has none.
		switch {
		case name == "overflow-main" && m[4] != "1":
			t.Errorf("%s: the report names goroutine %q, want 1:\n%s", name, m[4], out)
		case strings.Contains(name, "-go") && (m[4] == "" || m[4] == "1"):
			t.Errorf("%s: the report names goroutine %q, want a goroutine of 2 or more:\n%s", name, m[4], out)
		case strings.Contains(name, "-thread") && m[4] != "":
			t.Errorf("%s: the report names goroutine %q, want none (the thread has no record):\n%s", name, m[4], out)
		}
		// The stack pointer is where the stack ended: just below the lowest address (in the
		// guard), or a frame above it.
		if !(lo < hi && hi-lo >= minStack && sp < hi && sp+(1<<20) >= lo) {
			t.Errorf("%s: sp=%#x stack=[%#x, %#x]: want a stack of at least %d bytes with sp near its end", name, sp, lo, hi, minStack)
		}
		if strings.Count(out, "fatal error") != 1 {
			t.Errorf("%s: more than one report:\n%s", name, out)
		}
	})
}

// hookCase runs a case whose hook says what it was given and exits with 7. A sanitizer has no
// hook: its report instead.
func hookCase(t *testing.T, bin, name string, asan bool) {
	t.Run(name, func(t *testing.T) {
		out, code, sig := caseRun(t, bin, name)
		if asan {
			if code == 0 || code == 7 || strings.Contains(out, "hook:") {
				t.Errorf("%s: exit code %d %s, output:\n%.2000s\nwant the sanitizer's report and no call of the hook", name, code, sig, out)
			}
			return
		}
		if code != 7 || sig != "" || out != "hook: sp near the stack, bounds as stack_bounds\n" {
			t.Errorf("%s: exit code %d %s, output:\n%s", name, code, sig, out)
		}
	})
}

// crashCase runs a case that must end with the crash of the C++ runtime (SIGSEGV), without Go's
// message: a fault that is not a stack overflow, and the overflow of a thread that the runtime
// didn't start. In the asan mode: the sanitizer's report.
func crashCase(t *testing.T, bin, name string, asan bool) {
	t.Run(name, func(t *testing.T) {
		out, code, sig := caseRun(t, bin, name)
		// A case that needs a layout of the threads' stacks (guard-write-thread) exits with 3 and
		// "layout not found" when the kernel gave none: another try, and a skip (not a pass) when
		// none of the tries finds one.
		for try := 1; code == 3 && strings.Contains(out, "layout not found"); try++ {
			if try == 5 {
				t.Skipf("%s: the kernel's layout of thread stacks had no pair of neighbors in %d runs", name, try)
			}
			out, code, sig = caseRun(t, bin, name)
		}
		if strings.Contains(out, "fatal error") || strings.Contains(out, "stack overflow") && !asan {
			t.Errorf("%s: output:\n%s\nwant no Go message", name, out)
		}
		if asan {
			if code == 0 || !strings.Contains(out, "Sanitizer") {
				t.Errorf("%s: exit code %d %s, output:\n%.2000s\nwant the sanitizer's report", name, code, sig, out)
			}
			return
		}
		if !strings.Contains(sig, "segmentation fault") {
			t.Errorf("%s: exit code %d %q, output:\n%s\nwant SIGSEGV", name, code, sig, out)
		}
	})
}

// mainLimitCase runs the case overflow-main-far (the main thread's overflow at the lowest address
// of its stack, with two pages touched) under a shell's ulimit -s commands at exec, and checks Go's
// report and the size of the stack: want(size) says whether the size fits the limit. In the asan
// mode: the sanitizer's report instead.
func mainLimitCase(t *testing.T, bin, name string, ulimits []string, want func(size uint64) bool, asan bool) {
	t.Run("main-stack-"+name, func(t *testing.T) {
		script := strings.Join(append(slices.Clone(ulimits), `exec "$0" overflow-main-far`), " && ")
		args := []string{"-c", script, bin}
		if strings.HasPrefix(name, "no-aslr") {
			setarch, err := exec.LookPath("setarch")
			if err != nil {
				t.Skip("setarch is not installed")
			}
			args = append([]string{"-R", "sh"}, args...)
			b, _, code, err := command(filepath.Dir(bin), nil, true, setarch, args...)
			checkMainLimit(t, name, string(b), code, err, want, asan)
			return
		}
		b, _, code, err := command(filepath.Dir(bin), nil, true, "sh", args...)
		checkMainLimit(t, name, string(b), code, err, want, asan)
	})
}

func checkMainLimit(t *testing.T, name, out string, code int, err error, want func(size uint64) bool, asan bool) {
	t.Helper()
	if err != nil && code != -1 {
		t.Fatalf("%s: %v", name, err)
	}
	first, rest, _ := strings.Cut(out, "\n")
	size, perr := strconv.ParseUint(strings.TrimPrefix(first, "stack "), 10, 64)
	if !strings.HasPrefix(first, "stack ") || perr != nil {
		t.Fatalf("%s: exit code %d, output:\n%s\nwant the line \"stack N\" first", name, code, out)
	}
	if !want(size) {
		t.Errorf("%s: the main thread's stack is %d bytes (%d MiB), not what the limit gives", name, size, size>>20)
	}
	if asan {
		if code == 0 || !strings.Contains(rest, "Sanitizer") {
			t.Errorf("%s: exit code %d, output:\n%.2000s\nwant the sanitizer's report", name, code, out)
		}
		return
	}
	m := overflowRx.FindStringSubmatch(rest)
	if code != 2 || m == nil {
		t.Fatalf("%s: exit code %d, output:\n%s\nwant Go's stack overflow report and exit code 2 (no core dump)", name, code, out)
	}
	if lo, hi := parseHex(t, m[2]), parseHex(t, m[3]); hi-lo != size || m[4] != "1" {
		t.Errorf("%s: the report says stack=[%#x, %#x] and goroutine %q, want %d bytes and goroutine 1:\n%s", name, lo, hi, m[4], size, out)
	}
}

// runMainLimits runs mainLimitCase for the limits that exec may give the main thread (Linux): the
// usual 8 MiB with an unlimited hard limit (or whatever hard limit the test has), a tiny soft
// limit, unlimited, a hard limit of 256 MiB and of 8 MiB, a soft limit of 2 GiB (kept: the runtime
// only raises), and 8 MiB without ASLR (setarch -R),
// where the mapping below the stack, not the limit, ends it at about 127 MiB. The runtime sets the
// soft limit to 1 GiB (Go's limit), at most the hard limit (thread.cpp).
func runMainLimits(t *testing.T, bin string, asan bool) {
	if runtime.GOOS != "linux" {
		return
	}
	const gib = uint64(1) << 30
	hard := hardStackLimit(t)
	aslr := true
	if b, err := os.ReadFile("/proc/sys/kernel/randomize_va_space"); err == nil && strings.TrimSpace(string(b)) == "0" {
		aslr = false
	}
	limit := min(gib, hard)
	usual := func(size uint64) bool {
		if aslr {
			return size == limit
		}
		return size <= limit && size >= min(limit, 100<<20)
	}
	gapLimited := func(size uint64) bool { return size >= min(limit, 100<<20) && size <= min(limit, 200<<20) }
	mainLimitCase(t, bin, "8M", []string{"ulimit -S -s 8192"}, usual, asan)
	mainLimitCase(t, bin, "256K", []string{"ulimit -S -s 256"}, usual, asan)
	if hard == ^uint64(0) {
		// The kernel's legacy layout (mmap from the bottom): nothing is near the stack.
		mainLimitCase(t, bin, "unlimited", []string{"ulimit -s unlimited"}, func(size uint64) bool { return size == gib }, asan)
	}
	if hard >= 256<<20 {
		mainLimitCase(t, bin, "hard-256M", []string{"ulimit -H -s 262144", "ulimit -S -s 8192"}, func(size uint64) bool {
			return size == 256<<20 || !aslr && gapLimited(size)
		}, asan)
	}
	mainLimitCase(t, bin, "hard-8M", []string{"ulimit -S -s 8192", "ulimit -H -s 8192"}, func(size uint64) bool { return size == 8<<20 }, asan)
	if hard >= 2<<30 {
		// A soft limit above 1 GiB stays: the runtime only raises it.
		mainLimitCase(t, bin, "2G", []string{"ulimit -S -s 2097152"}, func(size uint64) bool {
			return size == 2<<30 || !aslr && size >= 100<<20 && size < 2<<30
		}, asan)
	}
	mainLimitCase(t, bin, "no-aslr-8M", []string{"ulimit -S -s 8192"}, gapLimited, asan)
	noProcCase(t, bin, asan)
}

// noProcCase runs the endless recursion on the main thread without /proc (a tmpfs over it in a
// mount namespace of its own) and without ASLR: the runtime can't read the mapping below the stack,
// so its bounds say 1 GiB where the stack really ends at about 127 MiB; the overflow handler's rule
// for the main thread (a fault just below the stack pointer, thread.cpp main_overflow) must still
// give Go's report. It needs unprivileged user namespaces (unshare -rm) and setarch; without them
// (a CI container, as a rule) it is skipped.
func noProcCase(t *testing.T, bin string, asan bool) {
	t.Run("main-stack-no-proc-no-aslr", func(t *testing.T) {
		for _, tool := range []string{"unshare", "setarch"} {
			if _, err := exec.LookPath(tool); err != nil {
				t.Skipf("%s is not installed", tool)
			}
		}
		b, _, code, err := command(filepath.Dir(bin), nil, true, "unshare", "-rm", "sh", "-c",
			`mount -t tmpfs none /proc || exit 97; ulimit -S -s 8192 && exec setarch -R "$0" overflow-main`, bin)
		out := string(b)
		if code == 97 || strings.HasPrefix(out, "unshare:") {
			t.Skipf("no mount namespace for the test: %s", strings.TrimSpace(out))
		}
		if err != nil && code != -1 {
			t.Fatal(err)
		}
		if asan {
			if code == 0 || !strings.Contains(out, "Sanitizer") {
				t.Errorf("exit code %d, output:\n%.2000s\nwant the sanitizer's report", code, out)
			}
			return
		}
		if m := overflowRx.FindStringSubmatch(out); code != 2 || m == nil || m[4] != "1" {
			t.Errorf("exit code %d, output:\n%s\nwant Go's stack overflow report of goroutine 1 and exit code 2 (no core dump)", code, out)
		}
	})
}

// hardStackLimit returns the hard RLIMIT_STACK of the test (all ones: unlimited).
func hardStackLimit(t *testing.T) uint64 {
	t.Helper()
	b, _, _, err := command("", nil, true, "sh", "-c", "ulimit -H -s")
	if err != nil {
		t.Fatal(err)
	}
	v := strings.TrimSpace(string(b))
	if v == "unlimited" {
		return ^uint64(0)
	}
	kib, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		t.Fatalf("ulimit -H -s: %q", v)
	}
	return kib << 10
}

// runThread runs the cases of test/thread_test.cpp that end the process, for a build in the mode
// named mode ("O0", "O2", "asan").
func runThread(t *testing.T, bin, mode string) {
	asan := mode == "asan"
	runMainLimits(t, bin, asan)
	overflowCase(t, bin, "overflow-main", 64<<20, asan) // under a hard limit of 64 MiB (caseRun)
	overflowCase(t, bin, "overflow-thread", 256<<10, asan)
	overflowCase(t, bin, "overflow-thread-default", 8<<20, asan)
	overflowCase(t, bin, "overflow-thread-big-frames", 1<<20, asan)
	overflowCase(t, bin, "overflow-thread-many", 256<<10, asan)
	hookCase(t, bin, "overflow-hook-thread", asan)
	hookCase(t, bin, "overflow-hook-main", asan)
	crashCase(t, bin, "segv-main", asan)
	// Faults just below the main thread's stack pointer on a page that is mapped (no overflow: the
	// stack could have grown there): running code from the stack, a write to a read-only page.
	if runtime.GOOS == "linux" {
		crashCase(t, bin, "exec-stack-main", asan)
		crashCase(t, bin, "write-ro-below-sp-main", asan)
	}
	crashCase(t, bin, "segv-thread", asan)
	crashCase(t, bin, "overflow-foreign", asan)
	// A write in the guard from a normal frame, a wild write with the stack pointer near the end
	// of the stack, and a signal sent with raise: none of them is an overflow.
	crashCase(t, bin, "guard-write-thread", asan)
	crashCase(t, bin, "wild-near-end-thread", asan)
	crashCase(t, bin, "raise-main", asan)
	crashCase(t, bin, "raise-thread", asan)
	// A frame of 4 MB on a 1 MiB stack with neighbors: the frame is probed, so the guard
	// catches it (the build has -fstack-clash-protection, baseFlags).
	overflowCase(t, bin, "overflow-thread-huge-frame", 1<<20, asan)
}

// tsanBuild builds the test program src (a file of test/) with the flags and the runtime, and
// returns the binary; the build fails the test if the compiler prints anything.
func tsanBuild(t *testing.T, c cxx, rt, src string, flags []string) string {
	t.Helper()
	r := c.build(rt, sourcePath(t, src), t.TempDir(), flags, 3)
	if r.own != nil || r.shared != nil {
		t.Fatalf("%s did not build:\n%v\n%v", src, r.own, r.shared)
	}
	return r.bin
}

// TestThreadTSan builds test/thread_test.cpp with ThreadSanitizer, with each compiler, and runs
// its unit tests: no TSan warning (a data race in the thread layer). Phase 08's step 8.12 turns
// TSan into a mode of the whole concurrency suite; this one is for the first thread code.
func TestThreadTSan(t *testing.T) {
	t.Parallel()
	ccs := compilers(t)
	rt := extractRuntime(t)
	for _, c := range ccs {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			flags := []string{"-O1", "-fsanitize=thread", "-fno-omit-frame-pointer"}
			bin := tsanBuild(t, c, rt, "test/thread_test.cpp", flags)
			runUnit(t, bin)
		})
	}
}
