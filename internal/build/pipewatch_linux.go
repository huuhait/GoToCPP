package build

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The looks at a running compiler (WatchPipes): the first one after pipePollFirst, then at
// intervals that double up to pipePollMax. A process that waits in the open of a named pipe at two
// looks in a row is stopped, so a hang ends within about two intervals; most compiles end before
// the second look, and a look reads a few small files of /proc per process.
var (
	pipePollFirst = 100 * time.Millisecond
	pipePollMax   = time.Second
)

// pipeWaitFuncs are the kernel functions in which an open of a named pipe waits for the other
// end (fs/pipe.c: fifo_open calls wait_for_partner, which the compiler of the kernel may inline),
// as /proc/<pid>/task/<tid>/wchan names them.
var pipeWaitFuncs = []string{"wait_for_partner", "fifo_open"}

// childrenFiles reports whether this kernel has the files /proc/<pid>/task/<tid>/children
// (CONFIG_PROC_CHILDREN), decided once, from gocpp's own thread: a children file that is missing
// later is a process that just exited, not another kernel. A test replaces it, to take the way of
// a kernel without the files.
var childrenFiles = sync.OnceValue(func() bool {
	_, err := os.Stat("/proc/self/task/" + strconv.Itoa(syscall.Gettid()) + "/children")
	return err == nil
})

// WatchPipes watches the processes of cmd, a compiler that was started (in a process group of its
// own: InGroup), until stop is called: a process of it (the driver, cc1plus, a compiler wrapper)
// whose thread waits in the open of a named pipe is stopped with the whole group (stopCompiler: the
// driver removes its temporary files), and stop then returns a *PipeError (nil otherwise). Call stop
// once, after cmd.Wait. WatchPipes does not change cmd (after Start, os/exec's own goroutines read
// it): the caller gives it a WaitDelay before Start, so that a process that it started and that
// holds its output does not make Wait wait after the stop (every compiler run of gocpp has one).
//
// On Linux, the wait is the thread's wchan in /proc (readable for the processes of the same user),
// and the pipe's name comes from the open's arguments (/proc/<pid>/task/<tid>/syscall and the
// process's memory: readable for gocpp's own child processes, unless the system forbids it, as
// kernel.yama.ptrace_scope 2 does; then the error has no path). Without a wchan (a kernel without
// symbols), nothing is found and the run goes on as before.
func WatchPipes(cmd *exec.Cmd) (stop func() error) {
	if cmd == nil || cmd.Process == nil {
		return func() error { return nil }
	}
	pid := cmd.Process.Pid
	group := cmd.SysProcAttr != nil && cmd.SysProcAttr.Setpgid
	done := make(chan struct{})
	var result error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		every := pipePollFirst
		t := time.NewTimer(every)
		defer t.Stop()
		var last map[procTask]bool
		for {
			select {
			case <-done:
				return
			case <-t.C:
			}
			now := pipeWaiters(pid)
			for task := range now {
				if !last[task] {
					continue
				}
				result = &PipeError{Path: pipePath(task)}
				stopCompiler(pid, task.pid, group)
				return
			}
			last = now
			every = min(every*2, pipePollMax)
			t.Reset(every)
		}
	}()
	return func() error {
		close(done)
		wg.Wait()
		return result
	}
}

// pipeStopGrace is how long stopCompiler waits for the compiler to end after SIGTERM, before
// SIGKILL.
var pipeStopGrace = 2 * time.Second

// stopCompiler stops the compiler pid (and its group, when group is set) and the process waiter
// that waits for the named pipe, which may have left the group (setsid) and holds the output that
// Wait reads. First with SIGTERM, on which a compiler driver removes its temporary files (gcc's
// cc*.s in TMPDIR, clang's outputs), then, for what still runs after pipeStopGrace, with SIGKILL:
// the group (as KillGroup), and pid and waiter only while they have not ended, so that no number
// of a process that has ended (and may be another process's now) gets the signal.
func stopCompiler(pid, waiter int, group bool) {
	if group {
		syscall.Kill(-pid, syscall.SIGTERM)
	}
	syscall.Kill(pid, syscall.SIGTERM) // both were seen running a moment ago
	syscall.Kill(waiter, syscall.SIGTERM)
	deadline := time.Now().Add(pipeStopGrace)
	for time.Now().Before(deadline) && !(ended(pid) && ended(waiter)) {
		time.Sleep(10 * time.Millisecond)
	}
	if group {
		syscall.Kill(-pid, syscall.SIGKILL) // whatever the group still has
	}
	for _, p := range []int{pid, waiter} {
		if !ended(p) {
			syscall.Kill(p, syscall.SIGKILL)
		}
	}
}

// ended reports whether the process pid has exited: it is not in /proc, or it is a zombie that
// its parent has not waited for yet.
func ended(pid int) bool {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	i := bytes.LastIndexByte(stat, ')')
	return i >= 0 && strings.HasPrefix(string(stat[i+1:]), " Z")
}

// procTask is a thread (tid) of the process pid.
type procTask struct{ pid, tid int }

// pipeWaiters returns the threads of the process pid and of its descendants that wait in the open
// of a named pipe. The descendants are those of /proc/<pid>/task/<tid>/children, also those that
// left the process group (setsid); a kernel without that file (CONFIG_PROC_CHILDREN) gets them
// by their parent, from one look at every process that the watchers share (childrenByScan).
func pipeWaiters(pid int) map[procTask]bool {
	found := map[procTask]bool{}
	seen := map[int]bool{}
	todo := []int{pid}
	for len(todo) > 0 {
		p := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[p] {
			continue
		}
		seen[p] = true
		dir := "/proc/" + strconv.Itoa(p) + "/task"
		tasks, err := os.ReadDir(dir)
		if err != nil {
			continue // it exited
		}
		files := childrenFiles()
		for _, task := range tasks {
			tid, err := strconv.Atoi(task.Name())
			if err != nil {
				continue
			}
			if w, err := os.ReadFile(dir + "/" + task.Name() + "/wchan"); err == nil && isPipeWait(string(w)) {
				found[procTask{p, tid}] = true
			}
			if !files {
				continue
			}
			kids, _ := os.ReadFile(dir + "/" + task.Name() + "/children") // none: the thread exited
			for _, f := range strings.Fields(string(kids)) {
				if k, err := strconv.Atoi(f); err == nil {
					todo = append(todo, k)
				}
			}
		}
		if !files {
			todo = append(todo, childrenByScan(p)...)
		}
	}
	return found
}

// isPipeWait reports whether the wchan w is one of pipeWaitFuncs.
func isPipeWait(w string) bool {
	w = strings.TrimSpace(w)
	for _, f := range pipeWaitFuncs {
		if w == f {
			return true
		}
	}
	return false
}

// procScan is the last look at every process of the system (/proc/*/stat): the children of each
// process, for a kernel without the children files. All the watchers of the moment share it: it is
// made again only when it is older than procScanLife, so the cost (about 9 ms for 2,500 processes)
// does not grow with the number of compiles that run at once. A process that started after the
// look is found at a later one; a wait is stopped only at the second look anyway.
var procScan struct {
	sync.Mutex
	at       time.Time
	children map[int][]int
}

// procScanLife is how long procScan is used again.
var procScanLife = 500 * time.Millisecond

// childrenByScan returns the child processes of pid by procScan.
func childrenByScan(pid int) []int {
	procScan.Lock()
	defer procScan.Unlock()
	if procScan.children == nil || time.Since(procScan.at) > procScanLife {
		procScan.children, procScan.at = scanParents(), time.Now()
	}
	return procScan.children[pid]
}

// scanParents returns the children of each process, from the parent in /proc/<pid>/stat (the
// fourth field).
func scanParents() map[int][]int {
	children := map[int][]int{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return children
	}
	for _, e := range entries {
		p, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		// pid (comm) state ppid ...: comm may hold spaces and parentheses, so the fields are
		// counted after its last ')'.
		i := bytes.LastIndexByte(stat, ')')
		if i < 0 {
			continue
		}
		fields := strings.Fields(string(stat[i+1:]))
		if len(fields) < 2 {
			continue
		}
		if ppid, err := strconv.Atoi(fields[1]); err == nil {
			children[ppid] = append(children[ppid], p)
		}
	}
	return children
}

// openCalls are the numbers of the system calls that open a file by name, for gocpp's own
// architecture (a compiler of another architecture is not recognized: its error has no path),
// and the index of the argument with the name. openat and openat2 have the folder's descriptor in
// their first argument.
var openCalls = map[string]map[int]int{
	"amd64":   {2: 0, 257: 1, 437: 1},
	"386":     {5: 0, 295: 1, 437: 1},
	"arm64":   {56: 1, 437: 1},
	"riscv64": {56: 1, 437: 1},
	"loong64": {56: 1, 437: 1},
}

// atFDCWD is AT_FDCWD as /proc/<pid>/syscall shows it (an unsigned word in hex).
const atFDCWD = -100

// pipePath returns the named pipe that the thread task waits to open, or "" when it can't be
// told: the system call and its arguments, the name read from the process's memory, made
// absolute with the process's folder (or the folder of openat's descriptor), and checked to be a
// named pipe.
func pipePath(task procTask) string {
	base := "/proc/" + strconv.Itoa(task.pid)
	sc, err := os.ReadFile(base + "/task/" + strconv.Itoa(task.tid) + "/syscall")
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(sc))
	if len(fields) < 4 {
		return ""
	}
	nr, err := strconv.Atoi(fields[0])
	if err != nil {
		return ""
	}
	arg, ok := openCalls[runtime.GOARCH][nr]
	if !ok {
		return ""
	}
	addr, err := strconv.ParseUint(strings.TrimPrefix(fields[1+arg], "0x"), 16, 64)
	if err != nil {
		return ""
	}
	name := readCString(base+"/mem", addr)
	if name == "" {
		return ""
	}
	if !filepath.IsAbs(name) {
		dir := base + "/cwd"
		if arg == 1 {
			fd, err := strconv.ParseUint(strings.TrimPrefix(fields[1], "0x"), 16, 64)
			if err != nil {
				return ""
			}
			if int32(fd) != atFDCWD {
				dir = base + "/fd/" + strconv.Itoa(int(int32(fd)))
			}
		}
		d, err := os.Readlink(dir)
		if err != nil {
			return ""
		}
		name = filepath.Join(d, name)
	}
	name = filepath.Clean(name)
	if fi, err := os.Stat(name); err != nil || fi.Mode()&os.ModeNamedPipe == 0 {
		return ""
	}
	return name
}

// readCString reads the NUL-terminated string at addr in the memory file mem (at most a path's
// length), a page at a time, so that a read never crosses into a page that is not mapped.
func readCString(mem string, addr uint64) string {
	f, err := os.Open(mem)
	if err != nil {
		return ""
	}
	defer f.Close()
	const page = 4096
	const max = 4096 // PATH_MAX
	var name []byte
	for len(name) < max {
		n := page - int(addr%page)
		buf := make([]byte, n)
		got, _ := f.ReadAt(buf, int64(addr))
		if got == 0 {
			return ""
		}
		if i := bytes.IndexByte(buf[:got], 0); i >= 0 {
			return string(append(name, buf[:i]...))
		}
		name = append(name, buf[:got]...)
		addr += uint64(got)
	}
	return ""
}
