// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

// Sleep pauses the current goroutine for at least the duration d.
// A negative or zero duration causes Sleep to return immediately.
//
// The goroutine waits on its own waiter with a deadline (no timer thread is involved), so a
// sleeping goroutine is not "asleep" for the deadlock detector: it will wake up by itself.
func Sleep(d Duration) {
	sleep(int64(d))
}

// sleep is Sleep for nanoseconds (a native takes a Go type that the runtime knows: int64, not Duration).
//
//gocpp:native gp::time::sleep
func sleep(ns int64)

// when is a helper function for setting the 'when' field of a runtimeTimer.
// It returns what the time will be, in nanoseconds, Duration d in the future.
// If d is negative, it is ignored. If the returned value would be less than
// zero because of an overflow, MaxInt64 is returned.
func when(d Duration) int64 {
	if d <= 0 {
		return runtimeNano()
	}
	t := runtimeNano() + int64(d)
	if t < 0 {
		// N.B. runtimeNano() and d are always positive, so addition
		// (including overflow) will never result in t == 0.
		t = 1<<63 - 1 // math.MaxInt64
	}
	return t
}

// The runtime's timers (runtime/src/timer.cpp, gocpp/time.hpp). A timer is a record attached to a
// channel object: the channel of a Timer or a Ticker, which is also what the user receives from, or,
// for AfterFunc, a channel that nobody uses and that only carries the record. One thread, started
// when the first timer is armed, keeps the armed timers in a heap and sleeps until the earliest.
// The heap holds the channel of a Timer or Ticker weakly: when nobody holds the Timer, the Ticker or
// its channel any more, the record goes away with the channel, even if the timer is still armed.

// startChan arms the timer of the channel c: at the monotonic time when (nanoseconds, the clock of
// runtimeNano), and then every period nanoseconds if period > 0. It calls f(c, due) from the timer
// thread (or from the goroutine that looks at the channel when the timer is due); due is the time
// (in the clock of runtimeNano) that the call is for.
//
//gocpp:native gp::time::start_chan
func startChan(c chan Time, when, period int64, f func(c chan Time, due int64))

// startFunc arms the timer of an AfterFunc: it starts a goroutine that runs f at the time when. c is
// a channel that carries the record. The heap holds it strongly until the timer has fired or is
// stopped, so the timer runs although nobody holds the Timer.
//
//gocpp:native gp::time::start_func
func startFunc(c chan Time, when int64, f func())

// stopTimer stops the timer of c, and reports whether it was active (whether it had not fired yet, or
// whether a value that it sent was still in the channel, which stopTimer removes).
//
//gocpp:native gp::time::stop
func stopTimer(c chan Time) bool

// resetTimer changes the timer of c to fire at when, then every period, and reports whether it was active, as
// stopTimer does. A value that an earlier setting sent is removed from the channel.
//
//gocpp:native gp::time::reset
func resetTimer(c chan Time, when, period int64) bool

// trySend sends v on c if that does not block, and reports whether it did.
//
//gocpp:native gp::time::try_send
func trySend(c chan Time, v Time) bool

// The Timer type represents a single event.
// When the Timer expires, the current time will be sent on C,
// unless the Timer was created by [AfterFunc].
// A Timer must be created with [NewTimer] or AfterFunc.
type Timer struct {
	C         <-chan Time
	r         chan Time // the channel that carries the timer's record (C for a channel timer)
	initTimer bool
}

// Stop prevents the [Timer] from firing.
// It returns true if the call stops the timer, false if the timer has already
// expired or been stopped.
//
// For a func-based timer created with [AfterFunc](d, f),
// if t.Stop returns false, then the timer has already expired
// and the function f has been started in its own goroutine;
// Stop does not wait for f to complete before returning.
// If the caller needs to know whether f is completed,
// it must coordinate with f explicitly.
//
// For a chan-based timer created with NewTimer(d), as of Go 1.23,
// any receive from t.C after Stop has returned is guaranteed to block
// rather than receive a stale time value from before the Stop;
// if the program has not received from t.C already and the timer is
// running, Stop is guaranteed to return true.
// Before Go 1.23, the only safe way to use Stop was insert an extra
// <-t.C if Stop returned false to drain a potential stale value.
// See the [NewTimer] documentation for more details.
func (t *Timer) Stop() bool {
	if !t.initTimer {
		panic("time: Stop called on uninitialized Timer")
	}
	return stopTimer(t.r)
}

// NewTimer creates a new Timer that will send
// the current time on its channel after at least duration d.
//
// Before Go 1.23, the garbage collector did not recover
// timers that had not yet expired or been stopped, so code often
// immediately deferred t.Stop after calling NewTimer, to make
// the timer recoverable when it was no longer needed.
// As of Go 1.23, the garbage collector can recover unreferenced
// timers, even if they haven't expired or been stopped.
// The Stop method is no longer necessary to help the garbage collector.
// (Code may of course still want to call Stop to stop the timer for other reasons.)
//
// Before Go 1.23, the channel associated with a Timer was
// asynchronous (buffered, capacity 1), which meant that
// stale time values could be received even after [Timer.Stop]
// or [Timer.Reset] returned.
// As of Go 1.23, the channel is synchronous (unbuffered, capacity 0),
// eliminating the possibility of those stale values.
//
// The GODEBUG setting asynctimerchan=1 restores both pre-Go 1.23
// behaviors: when set, unexpired timers won't be garbage collected, and
// channels will have buffered capacity. This setting may be removed
// in Go 1.27 or later.
func NewTimer(d Duration) *Timer {
	c := make(chan Time, 1)
	startChan(c, when(d), 0, sendTime)
	return &Timer{C: c, r: c, initTimer: true}
}

// Reset changes the timer to expire after duration d.
// It returns true if the timer had been active, false if the timer had
// expired or been stopped.
//
// For a func-based timer created with [AfterFunc](d, f), Reset either reschedules
// when f will run, in which case Reset returns true, or schedules f
// to run again, in which case it returns false.
// When Reset returns false, Reset neither waits for the prior f to
// complete before returning nor does it guarantee that the subsequent
// goroutine running f does not run concurrently with the prior
// one. If the caller needs to know whether the prior execution of
// f is completed, it must coordinate with f explicitly.
//
// For a chan-based timer created with NewTimer, as of Go 1.23,
// any receive from t.C after Reset has returned is guaranteed not
// to receive a time value corresponding to the previous timer settings;
// if the program has not received from t.C already and the timer is
// running, Reset is guaranteed to return true.
// Before Go 1.23, the only safe way to use Reset was to call [Timer.Stop]
// and explicitly drain the timer first.
// See the [NewTimer] documentation for more details.
func (t *Timer) Reset(d Duration) bool {
	if !t.initTimer {
		panic("time: Reset called on uninitialized Timer")
	}
	w := when(d)
	return resetTimer(t.r, w, 0)
}

// sendTime does a non-blocking send of the current time on c.
func sendTime(c chan Time, due int64) {
	// due is the (monotonic) time at which the channel send was supposed to happen. The current
	// time can be arbitrarily far into the future, because the thread can be delayed. Subtract the
	// difference, measured on one clock reading, to go back to the time that the timer was for.
	n := Now()
	var late int64
	if n.wall&hasMonotonic != 0 {
		late = n.ext - (due - startNano)
	} else {
		late = runtimeNano() - due
	}
	trySend(c, n.Add(Duration(-late)))
}

// After waits for the duration to elapse and then sends the current time
// on the returned channel.
// It is equivalent to [NewTimer](d).C.
//
// Before Go 1.23, this documentation warned that the underlying
// [Timer] would not be recovered by the garbage collector until the
// timer fired, and that if efficiency was a concern, code should use
// NewTimer instead and call [Timer.Stop] if the timer is no longer needed.
// As of Go 1.23, the garbage collector can recover unreferenced,
// unstopped timers. There is no reason to prefer NewTimer when After will do.
func After(d Duration) <-chan Time {
	return NewTimer(d).C
}

// AfterFunc waits for the duration to elapse and then calls f
// in its own goroutine. It returns a [Timer] that can
// be used to cancel the call using its Stop method.
// The returned Timer's C field is not used and will be nil.
func AfterFunc(d Duration, f func()) *Timer {
	r := make(chan Time)
	startFunc(r, when(d), f)
	return &Timer{r: r, initTimer: true}
}
