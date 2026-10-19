// phase: 09

// WithCancelCause, WithTimeoutCause and WithDeadlineCause: the first cause wins, Err stays Canceled
// or DeadlineExceeded, and Cause of an uncancelled context is nil.
package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var errStop = errors.New("stop requested")
var errOther = errors.New("other reason")
var errSlow = errors.New("took too long")

func main() {
	fmt.Println(context.Cause(context.Background()))

	ctx, cancel := context.WithCancelCause(context.Background())
	fmt.Println(context.Cause(ctx), ctx.Err())
	cancel(errStop)
	cancel(errOther) // too late
	<-ctx.Done()
	fmt.Println(ctx.Err(), context.Cause(ctx), errors.Is(context.Cause(ctx), errStop))

	ctx2, cancel2 := context.WithCancelCause(context.Background())
	cancel2(nil) // a nil cause is Canceled
	fmt.Println(ctx2.Err(), context.Cause(ctx2))

	// the cause of a parent shows in a child
	parent, cancelParent := context.WithCancelCause(context.Background())
	child, cancelChild := context.WithCancel(parent)
	defer cancelChild()
	cancelParent(errOther)
	<-child.Done()
	fmt.Println(child.Err(), context.Cause(child))

	// a timeout with a cause
	tctx, tcancel := context.WithTimeoutCause(context.Background(), time.Millisecond, errSlow)
	defer tcancel()
	<-tctx.Done()
	fmt.Println(tctx.Err(), context.Cause(tctx), errors.Is(tctx.Err(), context.DeadlineExceeded))

	// cancelling before the deadline: the cause is Canceled, not the deadline cause
	dctx, dcancel := context.WithDeadlineCause(context.Background(), time.Now().Add(time.Hour), errSlow)
	dcancel()
	<-dctx.Done()
	fmt.Println(dctx.Err(), context.Cause(dctx))

	// AfterFunc sees the context done
	actx, acancel := context.WithCancelCause(context.Background())
	ran := make(chan error, 1)
	stop := context.AfterFunc(actx, func() { ran <- context.Cause(actx) })
	acancel(errStop)
	fmt.Println(<-ran, stop())
}

// Output:
// <nil>
// <nil> <nil>
// context canceled stop requested true
// context canceled context canceled
// context canceled other reason
// context deadline exceeded took too long true
// context canceled context canceled
// stop requested false
