// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"errors"
	"internal/itoa"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// The part of Go's os.Process that gocpp has on Windows (Phase 14 step 14.3, stdlib/os/PORT.md): Go's
// exec.go and exec_windows.go with a process handle that the Process owns (FindProcess opens it),
// Kill is TerminateProcess, Wait is WaitForSingleObject and GetExitCodeProcess, and the states
// are Go's. StartProcess is not built (os/exec comes later); it fails with ENOSYS as on Linux.

func init() {
	// Go's exec_windows.go splits GetCommandLine here; the runtime did that at the start of the program with
	// the same rules (Go's, runtime/src/os/windows/cmdline.cpp, ADR-053), and with the executable's file as
	// the only argument for an empty command line.
	Args = runtime_args()
}

// ErrProcessDone indicates a [Process] has finished.
var ErrProcessDone = errors.New("os: process already finished")

type processStatus uint32

const (
	// statusOK means that the Process is ready to use.
	statusOK processStatus = iota

	// statusDone means that the Process has completed.
	statusDone

	// statusReleased means that the Process has been released.
	statusReleased
)

// Process stores the information about a process created by [StartProcess].
type Process struct {
	Pid int

	state atomic.Uint32 // processStatus

	// mu protects handle: the handle is closed by Release and by Wait, and a Signal or Wait
	// that runs meanwhile must not use a closed handle.
	mu     sync.RWMutex
	handle uintptr
}

// ProcAttr holds the attributes that will be applied to a new process
// started by StartProcess.
type ProcAttr struct {
	// If Dir is non-empty, the child changes into the directory before
	// creating the process.
	Dir string
	// If Env is non-nil, it gives the environment variables for the
	// new process in the form returned by Environ.
	// If it is nil, the result of Environ will be used.
	Env []string
	// Files specifies the open files inherited by the new process. The
	// first three entries correspond to standard input, standard output, and
	// standard error. An implementation may support additional entries,
	// depending on the underlying operating system. A nil entry corresponds
	// to that file being closed when the process starts.
	// On Unix systems, StartProcess will change these File values
	// to blocking mode, which means that SetDeadline will stop working
	// and calling Close will not interrupt a Read or Write.
	Files []*File

	// Operating system-specific process creation attributes.
	// Note that setting this field means that your program
	// may not execute properly or even compile on some
	// operating systems.
	Sys *syscall.SysProcAttr
}

// A Signal represents an operating system signal.
// The usual underlying implementation is operating system-dependent:
// on Unix it is syscall.Signal.
type Signal interface {
	String() string
	Signal() // to distinguish from other Stringers
}

// The only signal values guaranteed to be present in the os package on all
// systems are os.Interrupt (send the process an interrupt) and os.Kill (force
// the process to exit). On Windows, sending os.Interrupt to a process with
// os.Process.Signal is not implemented; it will return an error instead of
// sending a signal.
var (
	Interrupt Signal = syscall.SIGINT
	Kill      Signal = syscall.SIGKILL
)

// Getpid returns the process id of the caller.
func Getpid() int { return syscall.Getpid() }

// Getppid returns the process id of the caller's parent.
func Getppid() int { return syscall.Getppid() }

// FindProcess looks for a running process by its pid.
//
// The [Process] it returns can be used to obtain information
// about the underlying operating system process.
//
// On Windows, FindProcess opens the process (OpenProcess) and fails when it does not exist
// or cannot be opened.
func FindProcess(pid int) (*Process, error) {
	const da = syscall.STANDARD_RIGHTS_READ |
		syscall.PROCESS_QUERY_INFORMATION | syscall.SYNCHRONIZE
	h, e := syscall.OpenProcess(da, false, uint32(pid))
	if e != nil {
		return nil, NewSyscallError("OpenProcess", e)
	}
	return &Process{Pid: pid, handle: uintptr(h)}, nil
}

// StartProcess is not built yet (os/exec is a later step): it returns a [*PathError] with
// ENOSYS, "function not implemented", which satisfies errors.Is(err, errors.ErrUnsupported).
func StartProcess(name string, argv []string, attr *ProcAttr) (*Process, error) {
	return nil, &PathError{Op: "fork/exec", Path: name, Err: syscall.ENOSYS}
}

// Release releases any resources associated with the [Process] p,
// rendering it unusable in the future.
// Release only needs to be called if [Process.Wait] is not.
func (p *Process) Release() error {
	oldStatus := p.doRelease(statusReleased)

	// For backward compatibility, on Windows only,
	// we return EINVAL on a second call to Release.
	if oldStatus == statusReleased {
		return syscall.EINVAL
	}
	return nil
}

// doRelease releases a [Process], setting the status to newStatus and closing the handle.
// If the previous status is not statusOK, this does nothing.
// It returns the previous status.
func (p *Process) doRelease(newStatus processStatus) processStatus {
	for {
		state := p.state.Load()
		oldStatus := processStatus(state)
		if oldStatus != statusOK {
			return oldStatus
		}
		if !p.state.CompareAndSwap(state, uint32(newStatus)) {
			continue
		}
		p.mu.Lock()
		h := p.handle
		p.handle = 0
		p.mu.Unlock()
		if h != 0 {
			syscall.CloseHandle(syscall.Handle(h))
		}
		return statusOK
	}
}

// Kill causes the [Process] to exit immediately. Kill does not wait until
// the Process has actually exited. This only kills the Process itself,
// not any other processes it may have started.
func (p *Process) Kill() error {
	return p.Signal(Kill)
}

// Wait waits for the [Process] to exit, and then returns a
// ProcessState describing its status and an error, if any.
// Wait releases any resources associated with the Process.
// On most operating systems, the Process must be a child
// of the current process or an error will be returned.
func (p *Process) Wait() (*ProcessState, error) {
	switch processStatus(p.state.Load()) {
	case statusDone:
		return nil, ErrProcessDone
	case statusReleased:
		return nil, syscall.EINVAL
	}
	p.mu.RLock()
	handle := p.handle
	if handle == 0 {
		p.mu.RUnlock()
		return nil, syscall.EINVAL
	}
	s, e := syscall.WaitForSingleObject(syscall.Handle(handle), syscall.INFINITE)
	switch s {
	case syscall.WAIT_OBJECT_0:
		break
	case syscall.WAIT_FAILED:
		p.mu.RUnlock()
		return nil, NewSyscallError("WaitForSingleObject", e)
	default:
		p.mu.RUnlock()
		return nil, errors.New("os: unexpected result from WaitForSingleObject")
	}
	var ec uint32
	e = syscall.GetExitCodeProcess(syscall.Handle(handle), &ec)
	if e != nil {
		p.mu.RUnlock()
		return nil, NewSyscallError("GetExitCodeProcess", e)
	}
	var u syscall.Rusage
	e = syscall.GetProcessTimes(syscall.Handle(handle), &u.CreationTime, &u.ExitTime, &u.KernelTime, &u.UserTime)
	p.mu.RUnlock()
	if e != nil {
		return nil, NewSyscallError("GetProcessTimes", e)
	}

	// For compatibility we use statusReleased here rather
	// than statusDone.
	p.doRelease(statusReleased)

	return &ProcessState{p.Pid, syscall.WaitStatus{ExitCode: ec}, &u}, nil
}

// Signal sends a signal to the [Process].
// Sending [Interrupt] on Windows is not implemented.
func (p *Process) Signal(sig Signal) error {
	switch processStatus(p.state.Load()) {
	case statusDone:
		return ErrProcessDone
	case statusReleased:
		return syscall.EINVAL
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	handle := p.handle
	if handle == 0 {
		return syscall.EINVAL
	}

	if sig == Kill {
		var terminationHandle syscall.Handle
		e := syscall.DuplicateHandle(^syscall.Handle(0), syscall.Handle(handle), ^syscall.Handle(0), &terminationHandle, syscall.PROCESS_TERMINATE, false, 0)
		if e != nil {
			return NewSyscallError("DuplicateHandle", e)
		}
		defer syscall.CloseHandle(terminationHandle)
		e = syscall.TerminateProcess(syscall.Handle(terminationHandle), 1)
		return NewSyscallError("TerminateProcess", e)
	}
	// TODO(rsc): Handle Interrupt too?
	return syscall.Errno(syscall.EWINDOWS)
}

// ProcessState stores information about a process, as reported by Wait.
type ProcessState struct {
	pid    int                // The process's id.
	status syscall.WaitStatus // System-dependent status info.
	rusage *syscall.Rusage
}

// Pid returns the process id of the exited process.
func (p *ProcessState) Pid() int {
	return p.pid
}

func ftToDuration(ft *syscall.Filetime) time.Duration {
	n := int64(ft.HighDateTime)<<32 + int64(ft.LowDateTime) // in 100-nanosecond intervals
	return time.Duration(n*100) * time.Nanosecond
}

// UserTime returns the user CPU time of the exited process and its children.
func (p *ProcessState) UserTime() time.Duration {
	return ftToDuration(&p.rusage.UserTime)
}

// SystemTime returns the system CPU time of the exited process and its children.
func (p *ProcessState) SystemTime() time.Duration {
	return ftToDuration(&p.rusage.KernelTime)
}

// Exited reports whether the program has exited.
// On Unix systems this reports true if the program exited due to calling exit,
// but false if the program terminated due to a signal.
func (p *ProcessState) Exited() bool {
	return p.status.Exited()
}

// Success reports whether the program exited successfully,
// such as with exit status 0 on Unix.
func (p *ProcessState) Success() bool {
	return p.status.ExitStatus() == 0
}

// Sys returns system-dependent exit information about
// the process. Convert it to the appropriate underlying
// type, such as [syscall.WaitStatus] on Unix, to access its contents.
func (p *ProcessState) Sys() any {
	return p.status
}

// SysUsage returns system-dependent resource usage information about
// the exited process. Convert it to the appropriate underlying
// type, such as [*syscall.Rusage] on Unix, to access its contents.
// (On Unix, *syscall.Rusage matches struct rusage as defined in the
// getrusage(2) manual page.)
func (p *ProcessState) SysUsage() any {
	return p.rusage
}

func (p *ProcessState) String() string {
	if p == nil {
		return "<nil>"
	}
	status := p.Sys().(syscall.WaitStatus)
	res := ""
	switch {
	case status.Exited():
		code := status.ExitStatus()
		if uint(code) >= 1<<16 { // windows uses large hex numbers
			res = "exit status " + itoa.Uitox(uint(code))
		} else {
			res = "exit status " + itoa.Itoa(code)
		}
	}
	return res
}

// ExitCode returns the exit code of the exited process, or -1
// if the process hasn't exited or was terminated by a signal.
func (p *ProcessState) ExitCode() int {
	// return -1 if the process hasn't started.
	if p == nil {
		return -1
	}
	return int(p.status.ExitCode)
}
