// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syscall

type WaitStatus uint32

// Wait status is 7 bits at bottom, either 0 (exited),
// 0x7F (stopped), or a signal number that caused an exit.
// The 0x80 bit is whether there was a core dump.
// An extra number (exit code, signal causing a stop)
// is in the high bits. At least that's the idea.
// There are various irregularities. For example, the
// "continued" status is 0xFFFF, distinguishing itself
// from stopped via the core dump bit.

const (
	mask    = 0x7F
	core    = 0x80
	exited  = 0x00
	stopped = 0x7F
	shift   = 8
)

func (w WaitStatus) Exited() bool { return w&mask == exited }

func (w WaitStatus) Signaled() bool { return w&mask != stopped && w&mask != exited }

func (w WaitStatus) Stopped() bool { return w&0xFF == stopped }

func (w WaitStatus) Continued() bool { return w == 0xFFFF }

func (w WaitStatus) CoreDump() bool { return w.Signaled() && w&core != 0 }

func (w WaitStatus) ExitStatus() int {
	if !w.Exited() {
		return -1
	}
	return int(w>>shift) & 0xFF
}

func (w WaitStatus) Signal() Signal {
	if !w.Signaled() {
		return -1
	}
	return Signal(w & mask)
}

func (w WaitStatus) StopSignal() Signal {
	if !w.Stopped() {
		return -1
	}
	return Signal(w>>shift) & 0xFF
}

func (w WaitStatus) TrapCause() int {
	if w.StopSignal() != SIGTRAP {
		return -1
	}
	return int(w>>shift) >> 8
}

// Wait4 waits for the process pid (wait4(2)); see the fields of WaitStatus and Rusage.
func Wait4(pid int, wstatus *WaitStatus, options int, rusage *Rusage) (wpid int, err error) {
	var out [19]int64
	wpid, e := sysWait4(pid, options, out[:])
	if e != 0 {
		return -1, errnoErr(Errno(e))
	}
	if wstatus != nil {
		*wstatus = WaitStatus(out[0])
	}
	if rusage != nil {
		rusage.Utime = Timeval{Sec: out[1], Usec: out[2]}
		rusage.Stime = Timeval{Sec: out[3], Usec: out[4]}
		rusage.Maxrss = out[5]
		rusage.Ixrss = out[6]
		rusage.Idrss = out[7]
		rusage.Isrss = out[8]
		rusage.Minflt = out[9]
		rusage.Majflt = out[10]
		rusage.Nswap = out[11]
		rusage.Inblock = out[12]
		rusage.Oublock = out[13]
		rusage.Msgsnd = out[14]
		rusage.Msgrcv = out[15]
		rusage.Nsignals = out[16]
		rusage.Nvcsw = out[17]
		rusage.Nivcsw = out[18]
	}
	return wpid, nil
}

// sysWait4 is wait4(2): out gets the status, then the 18 numbers of struct rusage in the order
// of Rusage's fields (the Timevals as two numbers each).
//
//gocpp:native gp::syscall::sys_wait4
func sysWait4(pid int, options int, out []int64) (wpid int, errno int)
