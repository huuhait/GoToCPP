// phase: 09

// WithTimeout and WithDeadline: DeadlineExceeded, its Timeout method, the earlier deadline of a
// parent, a deadline in the past, and cancel before the deadline.
package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, ok := ctx.Deadline()
	fmt.Println("has deadline:", ok)
	<-ctx.Done()
	err := ctx.Err()
	fmt.Println(err, err == context.DeadlineExceeded, errors.Is(err, context.DeadlineExceeded))
	to, isTimeout := err.(interface{ Timeout() bool })
	fmt.Println(isTimeout && to.Timeout())

	// the parent's earlier deadline governs the child
	parent, cancelP := context.WithDeadline(context.Background(), time.Now().Add(10*time.Millisecond))
	defer cancelP()
	child, cancelC := context.WithTimeout(parent, time.Hour)
	defer cancelC()
	pd, _ := parent.Deadline()
	cd, _ := child.Deadline()
	fmt.Println(pd.Equal(cd))
	<-child.Done()
	fmt.Println(child.Err())

	// a deadline in the past is expired at once
	past, cancelPast := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelPast()
	select {
	case <-past.Done():
		fmt.Println("past:", past.Err())
	default:
		fmt.Println("past: not done")
	}

	// cancel wins over a distant deadline
	far, cancelFar := context.WithTimeout(context.Background(), time.Hour)
	cancelFar()
	<-far.Done()
	fmt.Println(far.Err())

	// a timeout of zero or less
	zero, cancelZero := context.WithTimeout(context.Background(), 0)
	defer cancelZero()
	<-zero.Done()
	fmt.Println(zero.Err())

	// the error of a late cancel stays DeadlineExceeded
	late, cancelLate := context.WithTimeout(context.Background(), time.Millisecond)
	<-late.Done()
	cancelLate()
	fmt.Println(late.Err())
}

// Output:
// has deadline: true
// context deadline exceeded true true
// true
// true
// context deadline exceeded
// past: context deadline exceeded
// context canceled
// context deadline exceeded
// context deadline exceeded
