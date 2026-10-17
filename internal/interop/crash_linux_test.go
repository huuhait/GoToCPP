package interop

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// crashBody is the body of a fake compiler (a shell script) that dies of SIGSEGV without a core
// dump, by running this test binary as the compiler (see TestMain).
func crashBody() string {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	return "GOCPP_TEST_CRASH=1 exec '" + exe + "'"
}

// crashIfAsked makes the process die of SIGSEGV when it runs as the fake compiler of
// crashBody: with the default action of the signal (Go's runtime handles SIGSEGV itself),
// and not dumpable, so that there is no core dump (cmd/gocpp does the same, die_linux.go).
func crashIfAsked() {
	if os.Getenv("GOCPP_TEST_CRASH") != "1" {
		return
	}
	signal.Reset(syscall.SIGSEGV)
	runtime.LockOSThread()
	var act [4]uintptr // struct kernel_sigaction: handler SIG_DFL (0), flags, restorer, mask
	syscall.RawSyscall6(syscall.SYS_RT_SIGACTION, uintptr(syscall.SIGSEGV), uintptr(unsafe.Pointer(&act)), 0, 8, 0, 0)
	const prSetDumpable = 4
	syscall.RawSyscall6(syscall.SYS_PRCTL, prSetDumpable, 0, 0, 0, 0, 0)
	mask := uint64(1) << (uint(syscall.SIGSEGV) - 1)
	const sigUnblock = 1
	syscall.RawSyscall6(syscall.SYS_RT_SIGPROCMASK, sigUnblock, uintptr(unsafe.Pointer(&mask)), 0, 8, 0, 0)
	syscall.Tgkill(syscall.Getpid(), syscall.Gettid(), syscall.SIGSEGV)
	time.Sleep(time.Second)
	os.Exit(3)
}
