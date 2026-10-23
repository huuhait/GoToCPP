// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package os

import (
	"errors"
	"internal/itoa"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// The part of Go's os.Process that gocpp has (Phase 09 step 9.6, stdlib/os/PORT.md): a Process
// is a pid (Go's version also keeps a pidfd where the kernel has them), Signal and Kill are
// kill(2), Wait is waitid(2) then wait4(2) (Go's pidWait), and the states are Go's.
// StartProcess is not built (it needs the fork and exec of package syscall: os/exec comes in a
// later step); it fails with ENOSYS.

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

// Special values for Process.Pid.
const (
	pidUnset    = 0
	pidReleased = -1
)

// Process stores the information about a process created by [StartProcess].
type Process struct {
	Pid int

	state atomic.Uint32 // processStatus

	sigMu sync.RWMutex // avoid race between wait and signal
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
// On Unix systems, FindProcess always succeeds and returns a Process
// for the given pid, regardless of whether the process exists. To test whether
// the process actually exists, see whether p.Signal(syscall.Signal(0)) reports
// an error.
func FindProcess(pid int) (*Process, error) {
	return &Process{Pid: pid}, nil
}

// StartProcess is not built yet (it needs fork and exec: os/exec is a later step of Phase
// 09): it returns a [*PathError] with ENOSYS, "function not implemented", which satisfies
// errors.Is(err, errors.ErrUnsupported).
func StartProcess(name string, argv []string, attr *ProcAttr) (*Process, error) {
	return nil, &PathError{Op: "fork/exec", Path: name, Err: syscall.ENOSYS}
}

// Release releases any resources associated with the [Process] p,
// rendering it unusable in the future.
// Release only needs to be called if [Process.Wait] is not.
func (p *Process) Release() error {
	// Unfortunately, for historical reasons, on systems other
	// than Windows, Release sets the Pid field to -1.
	// This causes the race detector to report a problem
	// on concurrent calls to Release, but we can't change it now.
	if runtime.GOOS != "windows" {
		p.Pid = -1
	}
	p.doRelease(statusReleased)
	return nil
}

// doRelease releases a [Process], setting the status to newStatus.
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
	// Go's pidWait (exec_unix.go): there is no pidfd here.
	if processStatus(p.state.Load()) == statusReleased {
		return nil, syscall.EINVAL
	}

	// If we can block until Wait4 will succeed immediately, do so.
	ready, err := p.blockUntilWaitable()
	if err != nil {
		return nil, err
	}
	if ready {
		// Mark the process done now, before the call to Wait4,
		// so that Process.pidSignal will not send a signal.
		p.doRelease(statusDone)
		// Acquire a write lock on sigMu to wait for any
		// active call to the signal method to complete.
		p.sigMu.Lock()
		p.sigMu.Unlock()
	}

	var (
		status syscall.WaitStatus
		rusage syscall.Rusage
	)
	pid1, err := ignoringEINTR2(func() (int, error) {
		return syscall.Wait4(p.Pid, &status, 0, &rusage)
	})
	if err != nil {
		return nil, NewSyscallError("wait", err)
	}
	p.doRelease(statusDone)
	return &ProcessState{
		pid:    pid1,
		status: status,
		rusage: &rusage,
	}, nil
}

// Signal sends a signal to the [Process].
// Sending [Interrupt] on Windows is not implemented.
func (p *Process) Signal(sig Signal) error {
	s, ok := sig.(syscall.Signal)
	if !ok {
		return errors.New("os: unsupported signal type")
	}
	if p.Pid == pidReleased {
		return errors.New("os: process already released")
	}
	if p.Pid == pidUnset {
		return errors.New("os: process not initialized")
	}

	p.sigMu.RLock()
	defer p.sigMu.RUnlock()

	switch processStatus(p.state.Load()) {
	case statusDone:
		return ErrProcessDone
	case statusReleased:
		return errors.New("os: process already released")
	}

	return convertESRCH(syscall.Kill(p.Pid, s))
}

func convertESRCH(err error) error {
	if err == syscall.ESRCH {
		return ErrProcessDone
	}
	return err
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

// UserTime returns the user CPU time of the exited process and its children.
func (p *ProcessState) UserTime() time.Duration {
	return time.Duration(p.rusage.Utime.Nano()) * time.Nanosecond
}

// SystemTime returns the system CPU time of the exited process and its children.
func (p *ProcessState) SystemTime() time.Duration {
	return time.Duration(p.rusage.Stime.Nano()) * time.Nanosecond
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
		res = "exit status " + itoa.Itoa(code) // unix
	case status.Signaled():
		res = "signal: " + status.Signal().String()
	case status.Stopped():
		res = "stop signal: " + status.StopSignal().String()
		if status.StopSignal() == syscall.SIGTRAP && status.TrapCause() != 0 {
			res += " (trap " + itoa.Itoa(status.TrapCause()) + ")"
		}
	case status.Continued():
		res = "continued"
	}
	if status.CoreDump() {
		res += " (core dumped)"
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
	return p.status.ExitStatus()
}
