//go:build ignore

package main

// Throwaway simulation of the register of ADR-124 in plain Go (design note 10.7a).
// Depth stands for the stack address: a deeper call has a larger depth, an older frame a smaller.
// The identity of an error is the pointer it holds (a typed nil pointer has none); the register has one.
// A wrapper that the runtime makes of the error in flight (Wrapf, %w, Join) takes the identity over; a
// wrapper of any other error changes nothing. The rule:
//   hop:   continue only for the same identity and a frame older than the last hop's
//          (depth smaller); nothing else is looked at, and no method of the error is called;
//   Trace: keep the longest suffix of hops in which each hop was made in the function that the
//          previous hop returned to (the "return address" of the sim is the name of that function;
//          a hop whose function was called through an interface or a function value returns into a
//          thunk, whose name is unusable: its return address is "" and the link after it is a cut).
// The name of a site is its static name (the Go function that holds the !), so inlining cannot change it.

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const kFirst, kRing = 16, 16

type site struct {
	file string
	line int
	fn   string
	ret  string // the function the function of this hop returns to ("" when unknown)
}

type register struct {
	ident     ident
	total     int
	sites     [kFirst + kRing]site
	lastDepth int
}

var registers sync.Map // goroutine id -> *register

func goid() string {
	buf := make([]byte, 64)
	buf = buf[:runtime.Stack(buf, false)]
	f := strings.Fields(string(buf))
	return f[1]
}

func current() *register {
	v, _ := registers.LoadOrStore(goid(), &register{})
	return v.(*register)
}

func depth() int {
	pcs := make([]uintptr, 1024)
	return runtime.Callers(0, pcs)
}

// ident is the identity of an error object: its pointer and type. (The register keeps the error alive in
// the sim so that an address is never reused; gocpp keeps a weak reference to the owner, which has the same
// effect.) An interface that holds a typed nil pointer or a value of another kind has no identity (in gocpp:
// no data pointer, or a box that is not shared).
type ident struct {
	t    reflect.Type
	p    uintptr
	keep error
}

func mkID(e error) ident {
	if e == nil {
		return ident{}
	}
	v := reflect.ValueOf(e)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return ident{}
	}
	return ident{v.Type(), v.Pointer(), e}
}

func (i ident) is(e error) bool {
	if i.t == nil || e == nil {
		return false
	}
	v := reflect.ValueOf(e)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return false
	}
	return v.Type() == i.t && v.Pointer() == i.p
}

// contains reports whether the tree of err (Unwrap() error and Unwrap() []error, pre-order,
// at most 64 nodes) holds the identity target. Only Trace and Traced call it, never hop.
func contains(err error, target ident) bool {
	budget := 64
	var walk func(e error) bool
	walk = func(e error) bool {
		if e == nil || budget == 0 {
			return false
		}
		budget--
		if target.is(e) {
			return true
		}
		switch u := e.(type) {
		case interface{ Unwrap() error }:
			return walk(u.Unwrap())
		case interface{ Unwrap() []error }:
			for _, c := range u.Unwrap() {
				if walk(c) {
					return true
				}
			}
		}
		return false
	}
	return walk(err)
}

// callerOf returns the name of the function that called the nearest frame named fn.
func callerOf(fn string) string {
	pcs := make([]uintptr, 64)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	seen := false
	for {
		f, more := frames.Next()
		if seen {
			return f.Function
		}
		if f.Function == fn {
			seen = true
		}
		if !more {
			return ""
		}
	}
}

// HopT is Hop for a function that its caller reaches through an interface or a function value: the
// real hop returns into a thunk, so its return address names nothing the link check can use.
func HopT(err error) {
	pc, file, line, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc).Name()
	hopAt(site{filepath.Base(file), line, fn, ""}, err, depth())
}

// Hop is what the error branch of a ! call runs.
func Hop(err error) {
	pc, file, line, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc).Name()
	hopAt(site{filepath.Base(file), line, fn, callerOf(fn)}, err, depth())
}

// HopAs is Hop for the deferred call of a defer F!() that becomes the error of its function fn: the
// hop is made in the deferred closure, but the site is named after fn and the return address is the
// one that fn saved when it started (the lowering keeps it in a local of fn).
func HopAs(err error, fn string) {
	_, file, line, _ := runtime.Caller(1)
	hopAt(site{filepath.Base(file), line, fn, callerOf(fn)}, err, depth())
}

func hopAt(s site, err error, d int) {
	r := current()
	same := r.total != 0 && r.ident.is(err)
	if !(same && d < r.lastDepth) {
		r.total = 0
	}
	n := r.total
	r.total++
	if n < kFirst {
		r.sites[n] = s
	} else {
		r.sites[kFirst+(n-kFirst)%kRing] = s
	}
	r.lastDepth = d
	if !same {
		r.ident = mkID(err)
	}
}

// rekey is what a wrapper that the runtime made of the error in flight does: it takes over the
// identity. It compares pointers only.
func rekey(old, wrapper error) {
	r := current()
	if r.total != 0 && r.ident.is(old) {
		r.ident = mkID(wrapper)
	}
}

type Frame struct {
	Function string
	File     string
	Line     int
}

func (f Frame) String() string {
	if f.File == "" {
		return "... " + strconv.Itoa(f.Line) + " hops omitted"
	}
	return f.File + ":" + strconv.Itoa(f.Line) + " " + f.Function
}

type tracedError struct {
	err    error
	frames []Frame
}

func (t *tracedError) Error() string { return t.err.Error() }
func (t *tracedError) Unwrap() error { return t.err }

// inflight returns the hops of the register if err holds one of its identities: the longest suffix
// whose links hold (the function a hop returned to is the function of the next hop). A link that
// cannot be checked is a cut; the junction of the ring's gap is the one exception (the gap frame says
// that hops are missing there).
func inflight(err error) []Frame {
	r := current()
	if r.total == 0 || !contains(err, r.ident) {
		return nil
	}
	n := r.total
	var hops []site
	for i := 0; i < n && i < kFirst; i++ {
		hops = append(hops, r.sites[i])
	}
	gapAt, gap := -1, 0
	if n > kFirst+kRing {
		gapAt, gap = len(hops), n-kFirst-kRing
	}
	if n > kFirst {
		start := kFirst
		if n > kFirst+kRing {
			start = n - kRing
		}
		for i := start; i < n; i++ {
			hops = append(hops, r.sites[kFirst+(i-kFirst)%kRing])
		}
	}
	first := 0
	for i := 1; i < len(hops); i++ {
		if i == gapAt {
			continue
		}
		if hops[i-1].ret == "" || hops[i-1].ret != hops[i].fn {
			first = i
		}
	}
	var out []Frame
	for i := first; i < len(hops); i++ {
		if i == gapAt && gapAt > first {
			out = append(out, Frame{Line: gap})
		}
		out = append(out, Frame{hops[i].fn, hops[i].file, hops[i].line})
	}
	return out
}

// Trace is errors.Trace: the snapshot of the first Traced wrapper in the tree of err, then the
// hops of the register.
func Trace(err error) []Frame {
	if err == nil {
		return nil
	}
	var snap []Frame
	for e := err; e != nil; e = errors.Unwrap(e) {
		if t, ok := e.(*tracedError); ok {
			snap = t.frames
			break
		}
	}
	return append(append([]Frame{}, snap...), inflight(err)...)
}

// Traced is errors.Traced: the wrapper takes the trace (the register is emptied when the trace
// was the error's), so a second Traced or a read in the same goroutine finds each hop once.
func Traced(err error) error {
	if err == nil {
		return nil
	}
	w := &tracedError{err, Trace(err)}
	r := current()
	if r.total != 0 && contains(err, r.ident) {
		r.total = 0
		r.ident = ident{}
	}
	return w
}

// Verb is what %+v adds to an error (a top-level operand only).
func Verb(err error) string {
	out := err.Error()
	fr := Trace(err)
	if len(fr) > 0 {
		out += "\nerror return trace:"
		for _, f := range fr {
			out += "\n\t" + f.String()
		}
	}
	return out
}

func Show(label string, err error) {
	fmt.Println("--", label)
	fmt.Println(Verb(err))
}

// Wrapf is errors.Wrapf: the wrapper takes over the identity of the error it wraps.
func Wrapf(errp *error, format string, args ...any) {
	if *errp == nil {
		return
	}
	old := *errp
	*errp = fmt.Errorf(format+": %w", append(args, old)...)
	rekey(old, *errp)
}

// Errorf is fmt.Errorf of gocpp's native engine: a %w operand that is the error in flight hands
// the identity to the new error.
func Errorf(format string, args ...any) error {
	e := fmt.Errorf(format, args...)
	for _, wrapped := range wrappedOperands(e) {
		if current().ident.is(wrapped) {
			rekey(wrapped, e)
			break
		}
	}
	return e
}

// wrappedOperands returns the %w operands of an error made by fmt.Errorf, and only those.
func wrappedOperands(e error) []error {
	switch u := e.(type) {
	case interface{ Unwrap() error }:
		return []error{u.Unwrap()}
	case interface{ Unwrap() []error }:
		return u.Unwrap()
	}
	return nil
}

// Join is errors.Join of gocpp: same for the operands.
func Join(errs ...error) error {
	j := errors.Join(errs...)
	for _, e := range errs {
		if r := current(); r.ident.is(e) {
			rekey(e, j)
			break
		}
	}
	return j
}
