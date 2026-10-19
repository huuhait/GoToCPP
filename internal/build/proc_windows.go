//go:build windows

package build

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

// Windows has no process groups; its equivalent is the job object (Phase 14 step 14.2). InGroup asks
// for one, Start puts the process into it right after the start (joinGroup), and canceling the
// command, KillGroup, and the end of gocpp itself (the job is created with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, so the system kills what is in it when the last handle
// closes, also when gocpp is killed or crashes) end every process of the compiler's tree: g++ runs
// cc1plus, as and ld as children, and they must not go on when gocpp is gone.
//
// There is one small window: a process that starts a child in the first moments after CreateProcess
// and before joinGroup runs has that child outside of the job (Windows gives the process to the job
// after it exists; making it suspended and resuming it would need the thread handle, which os/exec
// does not hand out). A compiler driver spends its first milliseconds loading, so in practice
// its children are inside. A job nests (Windows 8 and later), so a gocpp that is itself in a job,
// of a CI service or a terminal, still works; if the assignment is refused, canceling kills the
// compiler's own process only, as before.

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW      = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObj  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObj = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject    = kernel32.NewProc("TerminateJobObject")
	jobs                      sync.Map // *exec.Cmd -> *job: the commands that asked for a group
)

const (
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x2000
	processTerminate                  = 0x0001
	processSetQuota                   = 0x0100
)

// extendedLimit is JOBOBJECT_EXTENDED_LIMIT_INFORMATION of 64-bit Windows (144 bytes): the basic
// limits (64), the I/O counters (48) and four sizes (32); only LimitFlags is set.
type extendedLimit struct {
	perProcessUserTimeLimit int64
	perJobUserTimeLimit     int64
	limitFlags              uint32
	_                       uint32
	minimumWorkingSetSize   uintptr
	maximumWorkingSetSize   uintptr
	activeProcessLimit      uint32
	_                       uint32
	affinity                uintptr
	priorityClass           uint32
	schedulingClass         uint32
	ioInfo                  [6]uint64
	processMemoryLimit      uintptr
	jobMemoryLimit          uintptr
	peakProcessMemoryUsed   uintptr
	peakJobMemoryUsed       uintptr
}

type job struct {
	mu sync.Mutex
	h  syscall.Handle // 0 when there is none (not made yet, or closed)
}

// terminate ends every process of the job and closes it.
func (j *job) terminate() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.h != 0 {
		procTerminateJobObject.Call(uintptr(j.h), 1)
		syscall.CloseHandle(j.h)
		j.h = 0
	}
}

// InGroup makes the process tree of cmd killable as one: cmd gets a job object when it starts
// (Start), and canceling it (a time limit, a canceled build or probe, too much output) kills the
// whole tree.
func InGroup(cmd *exec.Cmd) {
	j := &job{}
	jobs.Store(cmd, j)
	cmd.Cancel = func() error {
		j.terminate()
		if cmd.Process != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}

// joinGroup puts the started cmd into its job object, when it asked for one (InGroup).
func joinGroup(cmd *exec.Cmd) {
	v, ok := jobs.Load(cmd)
	if !ok || cmd.Process == nil {
		return
	}
	j := v.(*job)
	h, _, _ := procCreateJobObjectW.Call(0, 0)
	if h == 0 {
		return
	}
	info := extendedLimit{limitFlags: jobObjectLimitKillOnJobClose}
	if r, _, _ := procSetInformationJobObj.Call(h, jobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)); r == 0 {
		syscall.CloseHandle(syscall.Handle(h))
		return
	}
	ph, err := syscall.OpenProcess(processTerminate|processSetQuota, false, uint32(cmd.Process.Pid))
	if err != nil {
		syscall.CloseHandle(syscall.Handle(h))
		return
	}
	defer syscall.CloseHandle(ph)
	if r, _, _ := procAssignProcessToJobObj.Call(h, uintptr(ph)); r == 0 {
		syscall.CloseHandle(syscall.Handle(h)) // refused (an old system's job): the compiler alone is killed
		return
	}
	j.mu.Lock()
	j.h = syscall.Handle(h)
	j.mu.Unlock()
}

// KillGroup ends what is left of the job of cmd (started with InGroup) after cmd exited, so that
// nothing writes into a folder while it is removed, and releases the job.
func KillGroup(cmd *exec.Cmd) {
	if v, ok := jobs.LoadAndDelete(cmd); ok {
		v.(*job).terminate()
	}
}
