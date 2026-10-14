package main

import (
	"os/signal"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// dieOf makes gocpp die of the signal sig, after a program that gocpp run
// ran died of it, or after sig stopped a build: so that the shell or the
// test that started gocpp sees the same as for the program itself (bash's
// loops stop at a Ctrl-C only when the command died of SIGINT, and the
// diff tests compare the signal with real Go's). It returns only if gocpp
// is still alive after a moment; main then exits with 128 plus the
// signal's number.
//
// Go's runtime keeps a handler of its own for some signals (SIGSEGV,
// SIGABRT, SIGQUIT, ...: it would print a traceback) and ignores a
// SIGPIPE that kill sent, even after signal.Reset. So the default action
// is set with the rt_sigaction system call itself. gocpp must not dump a
// core of its own for SIGSEGV or SIGABRT (systemd-coredump ignores a zero
// RLIMIT_CORE): PR_SET_DUMPABLE 0 turns that off. The signal goes to this
// thread (tgkill), which has it unblocked, so it is delivered before the
// next instruction.
func dieOf(sig syscall.Signal) {
	signal.Reset(sig)
	runtime.LockOSThread()
	var act [4]uintptr // struct kernel_sigaction: handler SIG_DFL (0), flags, restorer, mask
	syscall.RawSyscall6(syscall.SYS_RT_SIGACTION, uintptr(sig), uintptr(unsafe.Pointer(&act)), 0, 8, 0, 0)
	const prSetDumpable = 4
	syscall.RawSyscall6(syscall.SYS_PRCTL, prSetDumpable, 0, 0, 0, 0, 0)
	mask := uint64(1) << (uint(sig) - 1)
	const sigUnblock = 1
	syscall.RawSyscall6(syscall.SYS_RT_SIGPROCMASK, sigUnblock, uintptr(unsafe.Pointer(&mask)), 0, 8, 0, 0)
	syscall.Tgkill(syscall.Getpid(), syscall.Gettid(), sig)
	time.Sleep(100 * time.Millisecond) // a signal that is ignored or stops gocpp (SIGTSTP)
}
