// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syscall

import (
	"internal/syscall/windows/sysdll"
	"sync"
	"unsafe"
)

// DLLError describes reasons for DLL load failures.
type DLLError struct {
	Err     error
	ObjName string
	Msg     string
}

func (e *DLLError) Error() string { return e.Msg }

func (e *DLLError) Unwrap() error { return e.Err }

// The calls into the system (gocpp: ADR-076). Go's runtime implements Syscall..SyscallN, loadlibrary,
// loadsystemlibrary and getprocaddress; here they are natives of the Windows OS layer
// (runtime/src/os/windows/win_syscall.cpp, declared in gocpp/syscall.hpp): syscall15 calls the function
// at address trap with up to 15 integer arguments on the calling thread (a goroutine is an OS thread) and
// returns its RAX, 0 (Go returns XMM0 as r2, so a function that returns a double loses it here) and
// GetLastError() captured right after the call. syscallN is the same for up to 42 arguments (Go's limit).

//gocpp:native gp::syscall::win_syscall15
func syscall15(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15 uintptr) (r1, r2, errno uintptr)

//gocpp:native gp::syscall::win_syscall_n
func syscallN(trap uintptr, args []uintptr) (r1, r2, errno uintptr)

//gocpp:native gp::syscall::win_loadlibrary
func nativeLoadLibrary(filename uintptr) (handle, errno uintptr)

//gocpp:native gp::syscall::win_loadsystemlibrary
func nativeLoadSystemLibrary(filename uintptr) (handle, errno uintptr)

//gocpp:native gp::syscall::win_getprocaddress
func nativeGetProcAddress(handle, procname uintptr) (proc, errno uintptr)

// Deprecated: Use [SyscallN] instead.
func Syscall(trap, nargs, a1, a2, a3 uintptr) (r1, r2 uintptr, err Errno) {
	r1, r2, e := syscall15(trap, nargs, a1, a2, a3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	return r1, r2, Errno(e)
}

// Deprecated: Use [SyscallN] instead.
func Syscall6(trap, nargs, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err Errno) {
	r1, r2, e := syscall15(trap, nargs, a1, a2, a3, a4, a5, a6, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	return r1, r2, Errno(e)
}

// Deprecated: Use [SyscallN] instead.
func Syscall9(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9 uintptr) (r1, r2 uintptr, err Errno) {
	r1, r2, e := syscall15(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9, 0, 0, 0, 0, 0, 0)
	return r1, r2, Errno(e)
}

// Deprecated: Use [SyscallN] instead.
func Syscall12(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12 uintptr) (r1, r2 uintptr, err Errno) {
	r1, r2, e := syscall15(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, 0, 0, 0)
	return r1, r2, Errno(e)
}

// Deprecated: Use [SyscallN] instead.
func Syscall15(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15 uintptr) (r1, r2 uintptr, err Errno) {
	r1, r2, e := syscall15(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15)
	return r1, r2, Errno(e)
}

// Deprecated: Use [SyscallN] instead.
func Syscall18(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15, a16, a17, a18 uintptr) (r1, r2 uintptr, err Errno) {
	r1, r2, e := syscallN(trap, []uintptr{a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15, a16, a17, a18}[:nargs])
	return r1, r2, Errno(e)
}

// SyscallN calls the function at trap with the arguments args (at most 42, Go's limit; up to 15 without
// allocating).
func SyscallN(trap uintptr, args ...uintptr) (r1, r2 uintptr, err Errno) {
	if len(args) > 15 {
		if len(args) > 42 {
			panic("syscall: SyscallN: too many arguments (at most 42)")
		}
		r1, r2, e := syscallN(trap, args)
		return r1, r2, Errno(e)
	}
	var a [15]uintptr
	copy(a[:], args)
	r1, r2, e := syscall15(trap, uintptr(len(args)), a[0], a[1], a[2], a[3], a[4], a[5], a[6], a[7], a[8], a[9], a[10], a[11], a[12], a[13], a[14])
	return r1, r2, Errno(e)
}

func loadlibrary(filename *uint16) (handle uintptr, err Errno) {
	h, e := nativeLoadLibrary(uintptr(unsafe.Pointer(filename)))
	return h, Errno(e)
}

func loadsystemlibrary(filename *uint16) (handle uintptr, err Errno) {
	h, e := nativeLoadSystemLibrary(uintptr(unsafe.Pointer(filename)))
	return h, Errno(e)
}

func getprocaddress(handle uintptr, procname *uint8) (proc uintptr, err Errno) {
	p, e := nativeGetProcAddress(handle, uintptr(unsafe.Pointer(procname)))
	return p, Errno(e)
}

// A DLL implements access to a single DLL.
type DLL struct {
	Name   string
	Handle Handle
}

// LoadDLL loads the named DLL file into memory.
//
// If name is not an absolute path and is not a known system DLL used by
// Go, Windows will search for the named DLL in many locations, causing
// potential DLL preloading attacks.
//
// Use [LazyDLL] in golang.org/x/sys/windows for a secure way to
// load system DLLs.
func LoadDLL(name string) (*DLL, error) {
	namep, err := UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	var h uintptr
	var e Errno
	if sysdll.IsSystemDLL[name] {
		h, e = loadsystemlibrary(namep)
	} else {
		h, e = loadlibrary(namep)
	}
	if e != 0 {
		return nil, &DLLError{
			Err:     e,
			ObjName: name,
			Msg:     "Failed to load " + name + ": " + e.Error(),
		}
	}
	d := &DLL{
		Name:   name,
		Handle: Handle(h),
	}
	return d, nil
}

// MustLoadDLL is like [LoadDLL] but panics if load operation fails.
func MustLoadDLL(name string) *DLL {
	d, e := LoadDLL(name)
	if e != nil {
		panic(e)
	}
	return d
}

// FindProc searches [DLL] d for procedure named name and returns [*Proc]
// if found. It returns an error if search fails.
func (d *DLL) FindProc(name string) (proc *Proc, err error) {
	namep, err := BytePtrFromString(name)
	if err != nil {
		return nil, err
	}
	a, e := getprocaddress(uintptr(d.Handle), namep)
	if e != 0 {
		return nil, &DLLError{
			Err:     e,
			ObjName: name,
			Msg:     "Failed to find " + name + " procedure in " + d.Name + ": " + e.Error(),
		}
	}
	p := &Proc{
		Dll:  d,
		Name: name,
		addr: a,
	}
	return p, nil
}

// MustFindProc is like [DLL.FindProc] but panics if search fails.
func (d *DLL) MustFindProc(name string) *Proc {
	p, e := d.FindProc(name)
	if e != nil {
		panic(e)
	}
	return p
}

// Release unloads [DLL] d from memory.
func (d *DLL) Release() (err error) {
	return FreeLibrary(d.Handle)
}

// A Proc implements access to a procedure inside a [DLL].
type Proc struct {
	Dll  *DLL
	Name string
	addr uintptr
}

// Addr returns the address of the procedure represented by p.
// The return value can be passed to Syscall to run the procedure.
func (p *Proc) Addr() uintptr {
	return p.addr
}

// Call executes procedure p with arguments a.
//
// The returned error is always non-nil, constructed from the result of GetLastError.
// Callers must inspect the primary return value to decide whether an error occurred
// (according to the semantics of the specific function being called) before consulting
// the error. The error always has type [Errno].
//
// On amd64, Call can pass and return floating-point values. To pass
// an argument x with C type "float", use
// uintptr(math.Float32bits(x)). To pass an argument with C type
// "double", use uintptr(math.Float64bits(x)). Floating-point return
// values are returned in r2. The return value for C type "float" is
// [math.Float32frombits](uint32(r2)). For C type "double", it is
// [math.Float64frombits](uint64(r2)).
func (p *Proc) Call(a ...uintptr) (uintptr, uintptr, error) {
	return SyscallN(p.Addr(), a...)
}

// A LazyDLL implements access to a single [DLL].
// It will delay the load of the DLL until the first
// call to its [LazyDLL.Handle] method or to one of its
// [LazyProc]'s Addr method.
//
// LazyDLL is subject to the same DLL preloading attacks as documented
// on [LoadDLL].
//
// Use LazyDLL in golang.org/x/sys/windows for a secure way to
// load system DLLs.
type LazyDLL struct {
	mu   sync.Mutex
	dll  *DLL // non nil once DLL is loaded
	Name string
}

// Load loads DLL file d.Name into memory. It returns an error if fails.
// Load will not try to load DLL, if it is already loaded into memory.
func (d *LazyDLL) Load() error {
	// gocpp: always under the mutex (a pointer that is read without a lock needs an atomic pointer,
	// and the lock is cheap beside the call).
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dll == nil {
		dll, e := LoadDLL(d.Name)
		if e != nil {
			return e
		}
		d.dll = dll
	}
	return nil
}

// mustLoad is like Load but panics if search fails.
func (d *LazyDLL) mustLoad() {
	e := d.Load()
	if e != nil {
		panic(e)
	}
}

// Handle returns d's module handle.
func (d *LazyDLL) Handle() uintptr {
	d.mustLoad()
	return uintptr(d.dll.Handle)
}

// NewProc returns a [LazyProc] for accessing the named procedure in the [DLL] d.
func (d *LazyDLL) NewProc(name string) *LazyProc {
	return &LazyProc{l: d, Name: name}
}

// NewLazyDLL creates new [LazyDLL] associated with [DLL] file.
func NewLazyDLL(name string) *LazyDLL {
	return &LazyDLL{Name: name}
}

// A LazyProc implements access to a procedure inside a [LazyDLL].
// It delays the lookup until the [LazyProc.Addr], [LazyProc.Call], or [LazyProc.Find] method is called.
type LazyProc struct {
	mu   sync.Mutex
	Name string
	l    *LazyDLL
	proc *Proc
}

// Find searches [DLL] for procedure named p.Name. It returns
// an error if search fails. Find will not search procedure,
// if it is already found and loaded into memory.
func (p *LazyProc) Find() error {
	// gocpp: always under the mutex.
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.proc == nil {
		e := p.l.Load()
		if e != nil {
			return e
		}
		proc, e := p.l.dll.FindProc(p.Name)
		if e != nil {
			return e
		}
		p.proc = proc
	}
	return nil
}

// mustFind is like Find but panics if search fails.
func (p *LazyProc) mustFind() {
	e := p.Find()
	if e != nil {
		panic(e)
	}
}

// Addr returns the address of the procedure represented by p.
// The return value can be passed to Syscall to run the procedure.
func (p *LazyProc) Addr() uintptr {
	p.mustFind()
	return p.proc.Addr()
}

// Call executes procedure p with arguments a. See the documentation of
// Proc.Call for more information.
func (p *LazyProc) Call(a ...uintptr) (r1, r2 uintptr, lastErr error) {
	p.mustFind()
	return p.proc.Call(a...)
}
