package agent

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// A group is a job object. Closing its last handle ends every process in it (KILL_ON_JOB_CLOSE), so
// even a killed agent takes the programs it started with it, and kill ends the tree of one program.
type group struct{ h syscall.Handle }

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObject    = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJob  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJob = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject = kernel32.NewProc("TerminateJobObject")
)

const (
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x2000
	processSetQuota                   = 0x0100
	processTerminate                  = 0x0001
)

// jobExtendedLimit is JOBOBJECT_EXTENDED_LIMIT_INFORMATION.
type jobExtendedLimit struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
	ReadOperationCount      uint64
	WriteOperationCount     uint64
	OtherOperationCount     uint64
	ReadTransferCount       uint64
	WriteTransferCount      uint64
	OtherTransferCount      uint64
	ProcessMemoryLimit      uintptr
	JobMemoryLimit          uintptr
	PeakProcessMemoryUsed   uintptr
	PeakJobMemoryUsed       uintptr
}

func newGroup(cmd *exec.Cmd) (*group, error) {
	h, _, err := procCreateJobObject.Call(0, 0)
	if h == 0 {
		return nil, fmt.Errorf("CreateJobObject: %v", err)
	}
	var info jobExtendedLimit
	info.LimitFlags = jobObjectLimitKillOnJobClose
	r, _, err := procSetInformationJob.Call(h, jobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if r == 0 {
		syscall.CloseHandle(syscall.Handle(h))
		return nil, fmt.Errorf("SetInformationJobObject: %v", err)
	}
	return &group{h: syscall.Handle(h)}, nil
}

// attach puts the program, just started, into the job. A process that the program starts in the few
// microseconds before this call is not in it; the harness's own programs do not do that.
func (g *group) attach(p *os.Process) {
	ph, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(p.Pid))
	if err != nil {
		return
	}
	defer syscall.CloseHandle(ph)
	procAssignProcessToJob.Call(uintptr(g.h), uintptr(ph))
}

func (g *group) kill() {
	if g.h != 0 {
		procTerminateJobObject.Call(uintptr(g.h), 1)
	}
}

func (g *group) close() {
	if g.h != 0 {
		syscall.CloseHandle(g.h)
		g.h = 0
	}
}
