package cputime

import (
	"runtime"
	"testing"
	"time"
)

// A sleep uses no processor time; a busy loop does. Both are told apart by the amount of work,
// not by a bound that a loaded machine could break: the sleep is long, the loop's time is only
// required to be more than nothing.
func TestOfCountsWorkNotWaiting(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("no clock of a thread's processor time: Of returns the wall time")
	}
	slept, precise := Of(func() { time.Sleep(200 * time.Millisecond) })
	if !precise {
		t.Fatal("Of is not precise on Linux")
	}
	if slept >= 200*time.Millisecond {
		t.Errorf("a sleep of 200ms used %v of processor time", slept)
	}
	sum := 0
	busy, _ := Of(func() {
		for i := range 50_000_000 {
			sum += i % 7
		}
	})
	if busy <= 0 || sum == 0 {
		t.Errorf("a busy loop used %v of processor time", busy)
	}
}
