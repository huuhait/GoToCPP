package bench

import (
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"
)

// The Windows side of the benchmark harness (Phase 14 step 14.5): the language server is a process whose children
// (the C++ compiler of the namespace probe) must end with it, and its memory is the working set.

func setProcAttr(cmd *exec.Cmd) {}

// killGroup ends the process and the processes it started (taskkill /T).
func killGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	cmd.Process.Kill()
}

var (
	psapi                = syscall.NewLazyDLL("psapi.dll")
	getProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
)

// processMemoryCounters is PROCESS_MEMORY_COUNTERS.
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

// procMemory returns the working set of the process pid now and its peak, in bytes.
func procMemory(pid int) (rss, peak int64) {
	const processQueryLimitedInformation = 0x1000
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return 0, 0
	}
	defer syscall.CloseHandle(h)
	var c processMemoryCounters
	c.cb = uint32(unsafe.Sizeof(c))
	if r, _, _ := getProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&c)), uintptr(c.cb)); r == 0 {
		return 0, 0
	}
	return int64(c.workingSetSize), int64(c.peakWorkingSetSize)
}
