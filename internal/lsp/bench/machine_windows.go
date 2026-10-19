package bench

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

// describePlatform fills in the machine on Windows: the processor from the environment (PROCESSOR_IDENTIFIER), the
// memory from GlobalMemoryStatusEx, the version from "cmd /c ver".
func describePlatform(m *Machine) {
	if m.CPU == "" {
		m.CPU = os.Getenv("PROCESSOR_IDENTIFIER")
	}
	if m.MemGiB == 0 {
		var st struct {
			length               uint32
			memoryLoad           uint32
			totalPhys, availPhys uint64
			totalPage, availPage uint64
			totalVirt, availVirt uint64
			availExtendedVirtual uint64
		}
		st.length = uint32(unsafe.Sizeof(st))
		proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
		if r, _, _ := proc.Call(uintptr(unsafe.Pointer(&st))); r != 0 {
			m.MemGiB = float64(st.totalPhys) / (1 << 30)
		}
	}
	if m.Kernel == "" {
		if out, err := exec.Command("cmd", "/c", "ver").Output(); err == nil {
			m.Kernel = strings.TrimSpace(string(out))
		}
	}
}
