// phase: 09
// lsan: cycle
// Go 1.25's context/x_test.go (WithCancel, WithDeadline, WithTimeout, WithValue, parent/child cancellation trees, Cause, WithoutCancel, AfterFunc via the Context API, String methods, canceling and deadline races) through a small harness; expected output from real Go.

package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)


func (t *testT) Deadline() (time.Time, bool) { return time.Time{}, false } // no -timeout in the harness

// The following are regular tests in package main.

// otherContext is a Context that's not one of the types defined in context.go.
// This lets us test code paths that differ based on the underlying type of the
// Context.
type otherContext struct {
	context.Context
}

const (
	shortDuration    = 1 * time.Millisecond // a reasonable duration to block in a test
	veryLongDuration = 1000 * time.Hour     // an arbitrary upper bound on the test's running time
)

// quiescent returns an arbitrary duration by which the program should have
// completed any remaining work and reached a steady (idle) state.
func quiescent(t *testT) time.Duration {
	deadline, ok := t.Deadline()
	atomic.AddInt64(&checks, 1)
	if !ok {
		return 5 * time.Second
	}

	const arbitraryCleanupMargin = 1 * time.Second
	return time.Until(deadline) - arbitraryCleanupMargin
}
func TestBackground(t *testT) {
	c := context.Background()
	atomic.AddInt64(&checks, 1)
	if c == nil {
		t.Fatalf("Background returned nil")
	}
	select {
	case x := <-c.Done():
		t.Errorf("<-c.Done() == %v want nothing (it should block)", x)
	default:
	}
	atomic.AddInt64(&checks, 1)
	if got, want := fmt.Sprint(c), "context.Background"; got != want {
		t.Errorf("Background().String() = %q want %q", got, want)
	}
}

func TestTODO(t *testT) {
	c := context.TODO()
	atomic.AddInt64(&checks, 1)
	if c == nil {
		t.Fatalf("TODO returned nil")
	}
	select {
	case x := <-c.Done():
		t.Errorf("<-c.Done() == %v want nothing (it should block)", x)
	default:
	}
	atomic.AddInt64(&checks, 1)
	if got, want := fmt.Sprint(c), "context.TODO"; got != want {
		t.Errorf("TODO().String() = %q want %q", got, want)
	}
}

func TestWithCancel(t *testT) {
	c1, cancel := context.WithCancel(context.Background())

	atomic.AddInt64(&checks, 1)
	if got, want := fmt.Sprint(c1), "context.Background.WithCancel"; got != want {
		t.Errorf("c1.String() = %q want %q", got, want)
	}

	o := otherContext{c1}
	c2, _ := context.WithCancel(o)
	contexts := []context.Context{c1, o, c2}

	for i, c := range contexts {
		atomic.AddInt64(&checks, 1)
		if d := c.Done(); d == nil {
			t.Errorf("c[%d].Done() == %v want non-nil", i, d)
		}
		atomic.AddInt64(&checks, 1)
		if e := c.Err(); e != nil {
			t.Errorf("c[%d].Err() == %v want nil", i, e)
		}

		select {
		case x := <-c.Done():
			t.Errorf("<-c.Done() == %v want nothing (it should block)", x)
		default:
		}
	}

	cancel() // Should propagate synchronously.
	for i, c := range contexts {
		select {
		case <-c.Done():
		default:
			t.Errorf("<-c[%d].Done() blocked, but shouldn't have", i)
		}
		atomic.AddInt64(&checks, 1)
		if e := c.Err(); e != context.Canceled {
			t.Errorf("c[%d].Err() == %v want %v", i, e, context.Canceled)
		}
	}
}

func testDeadline(c context.Context, name string, t *testT) {
	t.Helper()
	d := quiescent(t)
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		t.Fatalf("%s: context not timed out after %v", name, d)
	case <-c.Done():
	}
	atomic.AddInt64(&checks, 1)
	if e := c.Err(); e != context.DeadlineExceeded {
		t.Errorf("%s: c.Err() == %v; want %v", name, e, context.DeadlineExceeded)
	}
}

func TestDeadline(t *testT) {
	t.Parallel()

	c, _ := context.WithDeadline(context.Background(), time.Now().Add(shortDuration))
	atomic.AddInt64(&checks, 1)
	if got, prefix := fmt.Sprint(c), "context.Background.WithDeadline("; !strings.HasPrefix(got, prefix) {
		t.Errorf("c.String() = %q want prefix %q", got, prefix)
	}
	testDeadline(c, "WithDeadline", t)

	c, _ = context.WithDeadline(context.Background(), time.Now().Add(shortDuration))
	o := otherContext{c}
	testDeadline(o, "WithDeadline+otherContext", t)

	c, _ = context.WithDeadline(context.Background(), time.Now().Add(shortDuration))
	o = otherContext{c}
	c, _ = context.WithDeadline(o, time.Now().Add(veryLongDuration))
	testDeadline(c, "WithDeadline+otherContext+WithDeadline", t)

	c, _ = context.WithDeadline(context.Background(), time.Now().Add(-shortDuration))
	testDeadline(c, "WithDeadline+inthepast", t)

	c, _ = context.WithDeadline(context.Background(), time.Now())
	testDeadline(c, "WithDeadline+now", t)
}

func TestTimeout(t *testT) {
	t.Parallel()

	c, _ := context.WithTimeout(context.Background(), shortDuration)
	atomic.AddInt64(&checks, 1)
	if got, prefix := fmt.Sprint(c), "context.Background.WithDeadline("; !strings.HasPrefix(got, prefix) {
		t.Errorf("c.String() = %q want prefix %q", got, prefix)
	}
	testDeadline(c, "WithTimeout", t)

	c, _ = context.WithTimeout(context.Background(), shortDuration)
	o := otherContext{c}
	testDeadline(o, "WithTimeout+otherContext", t)

	c, _ = context.WithTimeout(context.Background(), shortDuration)
	o = otherContext{c}
	c, _ = context.WithTimeout(o, veryLongDuration)
	testDeadline(c, "WithTimeout+otherContext+WithTimeout", t)
}

func TestCanceledTimeout(t *testT) {
	c, _ := context.WithTimeout(context.Background(), time.Second)
	o := otherContext{c}
	c, cancel := context.WithTimeout(o, veryLongDuration)
	cancel() // Should propagate synchronously.
	select {
	case <-c.Done():
	default:
		t.Errorf("<-c.Done() blocked, but shouldn't have")
	}
	atomic.AddInt64(&checks, 1)
	if e := c.Err(); e != context.Canceled {
		t.Errorf("c.Err() == %v want %v", e, context.Canceled)
	}
}

type key1 int
type key2 int

func (k key2) String() string { return fmt.Sprintf("%[1]T(%[1]d)", k) }

var k1 = key1(1)
var k2 = key2(1) // same int as k1, different type
var k3 = key2(3) // same type as k2, different int

func TestValues(t *testT) {
	check := func(c context.Context, nm, v1, v2, v3 string) {
		atomic.AddInt64(&checks, 1)
		if v, ok := c.Value(k1).(string); ok == (len(v1) == 0) || v != v1 {
			t.Errorf(`%s.Value(k1).(string) = %q, %t want %q, %t`, nm, v, ok, v1, len(v1) != 0)
		}
		atomic.AddInt64(&checks, 1)
		if v, ok := c.Value(k2).(string); ok == (len(v2) == 0) || v != v2 {
			t.Errorf(`%s.Value(k2).(string) = %q, %t want %q, %t`, nm, v, ok, v2, len(v2) != 0)
		}
		atomic.AddInt64(&checks, 1)
		if v, ok := c.Value(k3).(string); ok == (len(v3) == 0) || v != v3 {
			t.Errorf(`%s.Value(k3).(string) = %q, %t want %q, %t`, nm, v, ok, v3, len(v3) != 0)
		}
	}

	c0 := context.Background()
	check(c0, "c0", "", "", "")

	c1 := context.WithValue(context.Background(), k1, "c1k1")
	check(c1, "c1", "c1k1", "", "")

	atomic.AddInt64(&checks, 1)
	if got, want := fmt.Sprint(c1), `context.Background.WithValue(main.key1, c1k1)`; got != want {
		t.Errorf("c.String() = %q want %q", got, want)
	}

	c2 := context.WithValue(c1, k2, "c2k2")
	check(c2, "c2", "c1k1", "c2k2", "")

	atomic.AddInt64(&checks, 1)
	if got, want := fmt.Sprint(c2), `context.Background.WithValue(main.key1, c1k1).WithValue(main.key2(1), c2k2)`; got != want {
		t.Errorf("c.String() = %q want %q", got, want)
	}

	c3 := context.WithValue(c2, k3, "c3k3")
	check(c3, "c2", "c1k1", "c2k2", "c3k3")

	c4 := context.WithValue(c3, k1, nil)
	check(c4, "c4", "", "c2k2", "c3k3")

	atomic.AddInt64(&checks, 1)
	if got, want := fmt.Sprint(c4), `context.Background.WithValue(main.key1, c1k1).WithValue(main.key2(1), c2k2).WithValue(main.key2(3), c3k3).WithValue(main.key1, <nil>)`; got != want {
		t.Errorf("c.String() = %q want %q", got, want)
	}

	o0 := otherContext{context.Background()}
	check(o0, "o0", "", "", "")

	o1 := otherContext{context.WithValue(context.Background(), k1, "c1k1")}
	check(o1, "o1", "c1k1", "", "")

	o2 := context.WithValue(o1, k2, "o2k2")
	check(o2, "o2", "c1k1", "o2k2", "")

	o3 := otherContext{c4}
	check(o3, "o3", "", "c2k2", "c3k3")

	o4 := context.WithValue(o3, k3, nil)
	check(o4, "o4", "", "c2k2", "")
}

func TestAllocs(t *testT) {
	atomic.AddInt64(&checks, 1)
	if false {
		t.Skip("test allocates more with -asan")
	}
	bg := context.Background()
	for _, test := range []struct {
		desc       string
		f          func()
		limit      float64
		gccgoLimit float64
	}{
		{
			desc:       "Background()",
			f:          func() { context.Background() },
			limit:      0,
			gccgoLimit: 0,
		},
		{
			desc: fmt.Sprintf("WithValue(bg, %v, nil)", k1),
			f: func() {
				c := context.WithValue(bg, k1, nil)
				c.Value(k1)
			},
			limit:      3,
			gccgoLimit: 3,
		},
		{
			desc: "WithTimeout(bg, 1*time.Nanosecond)",
			f: func() {
				c, _ := context.WithTimeout(bg, 1*time.Nanosecond)
				<-c.Done()
			},
			limit:      12,
			gccgoLimit: 15,
		},
		{
			desc: "WithCancel(bg)",
			f: func() {
				c, cancel := context.WithCancel(bg)
				cancel()
				<-c.Done()
			},
			limit:      5,
			gccgoLimit: 8,
		},
		{
			desc: "WithTimeout(bg, 5*time.Millisecond)",
			f: func() {
				c, cancel := context.WithTimeout(bg, 5*time.Millisecond)
				cancel()
				<-c.Done()
			},
			limit:      8,
			gccgoLimit: 25,
		},
	} {
		limit := test.limit
		atomic.AddInt64(&checks, 1)
		if runtime.Compiler == "gccgo" {
			// gccgo does not yet do escape analysis.
			// TODO(iant): Remove this when gccgo does do escape analysis.
			limit = test.gccgoLimit
		}
		numRuns := 100
		atomic.AddInt64(&checks, 1)
		if true {
			numRuns = 10
		}
		atomic.AddInt64(&checks, 1)
		if n := allocsPerRun(numRuns, test.f); n > limit {
			t.Errorf("%s allocs = %f want %d", test.desc, n, int(limit))
		}
	}
}

func TestSimultaneousCancels(t *testT) {
	root, cancel := context.WithCancel(context.Background())
	m := map[context.Context]context.CancelFunc{root: cancel}
	q := []context.Context{root}
	// Create a tree of contexts.
	for len(q) != 0 && len(m) < 100 {
		parent := q[0]
		q = q[1:]
		for i := 0; i < 4; i++ {
			ctx, cancel := context.WithCancel(parent)
			m[ctx] = cancel
			q = append(q, ctx)
		}
	}
	// Start all the cancels in a random order.
	var wg sync.WaitGroup
	wg.Add(len(m))
	for _, cancel := range m {
		go func(cancel context.CancelFunc) {
			cancel()
			wg.Done()
		}(cancel)
	}

	d := quiescent(t)
	stuck := make(chan struct{})
	timer := time.AfterFunc(d, func() { close(stuck) })
	defer timer.Stop()

	// Wait on all the contexts in a random order.
	for ctx := range m {
		select {
		case <-ctx.Done():
		case <-stuck:
			buf := make([]byte, 10<<10)
			n := runtime.Stack(buf, true)
			t.Fatalf("timed out after %v waiting for <-ctx.Done(); stacks:\n%s", d, buf[:n])
		}
	}
	// Wait for all the cancel functions to return.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-stuck:
		buf := make([]byte, 10<<10)
		n := runtime.Stack(buf, true)
		t.Fatalf("timed out after %v waiting for cancel functions; stacks:\n%s", d, buf[:n])
	}
}

func TestInterlockedCancels(t *testT) {
	parent, cancelParent := context.WithCancel(context.Background())
	child, cancelChild := context.WithCancel(parent)
	go func() {
		<-parent.Done()
		cancelChild()
	}()
	cancelParent()
	d := quiescent(t)
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-child.Done():
	case <-timer.C:
		buf := make([]byte, 10<<10)
		n := runtime.Stack(buf, true)
		t.Fatalf("timed out after %v waiting for child.Done(); stacks:\n%s", d, buf[:n])
	}
}

func TestLayersCancel(t *testT) {
	for seed := int64(1); seed <= 20; seed++ {
		testLayers(t, seed, false)
	}
}

func TestLayersTimeout(t *testT) {
	for seed := int64(1); seed <= 20; seed++ {
		testLayers(t, seed, true)
	}
}

func testLayers(t *testT, seed int64, testTimeout bool) {
	t.Parallel()

	r := &rngT{uint64(seed)}
	prefix := fmt.Sprintf("seed=%d", seed)
	errorf := func(format string, a ...any) {
		t.Errorf(prefix+format, a...)
	}
	const (
		minLayers = 30
	)
	type value int
	var (
		vals      []*value
		cancels   []context.CancelFunc
		numTimers int
		ctx       = context.Background()
	)
	for i := 0; i < minLayers || numTimers == 0 || len(cancels) == 0 || len(vals) == 0; i++ {
		switch r.Intn(3) {
		case 0:
			v := new(value)
			ctx = context.WithValue(ctx, v, v)
			vals = append(vals, v)
		case 1:
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancels = append(cancels, cancel)
		case 2:
			var cancel context.CancelFunc
			d := veryLongDuration
			atomic.AddInt64(&checks, 1)
			if testTimeout {
				d = shortDuration
			}
			ctx, cancel = context.WithTimeout(ctx, d)
			cancels = append(cancels, cancel)
			numTimers++
		}
	}
	checkValues := func(when string) {
		for _, key := range vals {
			atomic.AddInt64(&checks, 1)
			if val := ctx.Value(key).(*value); key != val {
				errorf("%s: ctx.Value(%p) = %p want %p", when, key, val, key)
			}
		}
	}
	atomic.AddInt64(&checks, 1)
	if !testTimeout {
		select {
		case <-ctx.Done():
			errorf("ctx should not be canceled yet")
		default:
		}
	}
	atomic.AddInt64(&checks, 1)
	if s, prefix := fmt.Sprint(ctx), "context.Background."; !strings.HasPrefix(s, prefix) {
		t.Errorf("ctx.String() = %q want prefix %q", s, prefix)
	}
	t.Log(ctx)
	checkValues("before cancel")
	atomic.AddInt64(&checks, 1)
	if testTimeout {
		d := quiescent(t)
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
			errorf("ctx should have timed out after %v", d)
		}
		checkValues("after timeout")
	} else {
		cancel := cancels[r.Intn(len(cancels))]
		cancel()
		select {
		case <-ctx.Done():
		default:
			errorf("ctx should be canceled")
		}
		checkValues("after cancel")
	}
}

func TestWithCancelCanceledParent(t *testT) {
	parent, pcancel := context.WithCancelCause(context.Background())
	cause := fmt.Errorf("Because!")
	pcancel(cause)

	c, _ := context.WithCancel(parent)
	select {
	case <-c.Done():
	default:
		t.Errorf("child not done immediately upon construction")
	}
	atomic.AddInt64(&checks, 1)
	if got, want := c.Err(), context.Canceled; got != want {
		t.Errorf("child not canceled; got = %v, want = %v", got, want)
	}
	atomic.AddInt64(&checks, 1)
	if got, want := context.Cause(c), cause; got != want {
		t.Errorf("child has wrong cause; got = %v, want = %v", got, want)
	}
}

func TestWithCancelSimultaneouslyCanceledParent(t *testT) {
	// Cancel the parent goroutine concurrently with creating a child.
	for i := 0; i < 100; i++ {
		parent, pcancel := context.WithCancelCause(context.Background())
		cause := fmt.Errorf("Because!")
		go pcancel(cause)

		c, _ := context.WithCancel(parent)
		<-c.Done()
		atomic.AddInt64(&checks, 1)
		if got, want := c.Err(), context.Canceled; got != want {
			t.Errorf("child not canceled; got = %v, want = %v", got, want)
		}
		atomic.AddInt64(&checks, 1)
		if got, want := context.Cause(c), cause; got != want {
			t.Errorf("child has wrong cause; got = %v, want = %v", got, want)
		}
	}
}

func TestWithValueChecksKey(t *testT) {
	panicVal := recoveredValue(func() { _ = context.WithValue(context.Background(), []byte("foo"), "bar") })
	atomic.AddInt64(&checks, 1)
	if panicVal == nil {
		t.Error("expected panic")
	}
	panicVal = recoveredValue(func() { _ = context.WithValue(context.Background(), nil, "bar") })
	atomic.AddInt64(&checks, 1)
	if got, want := fmt.Sprint(panicVal), "nil key"; got != want {
		t.Errorf("panic = %q; want %q", got, want)
	}
}

func TestInvalidDerivedFail(t *testT) {
	panicVal := recoveredValue(func() { _, _ = context.WithCancel(nil) })
	atomic.AddInt64(&checks, 1)
	if panicVal == nil {
		t.Error("expected panic")
	}
	panicVal = recoveredValue(func() { _, _ = context.WithDeadline(nil, time.Now().Add(shortDuration)) })
	atomic.AddInt64(&checks, 1)
	if panicVal == nil {
		t.Error("expected panic")
	}
	panicVal = recoveredValue(func() { _ = context.WithValue(nil, "foo", "bar") })
	atomic.AddInt64(&checks, 1)
	if panicVal == nil {
		t.Error("expected panic")
	}
}

func recoveredValue(fn func()) (v any) {
	defer func() { v = recover() }()
	fn()
	return
}

func TestDeadlineExceededSupportsTimeout(t *testT) {
	i, ok := context.DeadlineExceeded.(interface {
		Timeout() bool
	})
	atomic.AddInt64(&checks, 1)
	if !ok {
		t.Fatal("DeadlineExceeded does not support Timeout interface")
	}
	atomic.AddInt64(&checks, 1)
	if !i.Timeout() {
		t.Fatal("wrong value for timeout")
	}
}
func TestCause(t *testT) {
	var (
		forever       = 1e6 * time.Second
		parentCause   = fmt.Errorf("parentCause")
		childCause    = fmt.Errorf("childCause")
		tooSlow       = fmt.Errorf("tooSlow")
		finishedEarly = fmt.Errorf("finishedEarly")
	)
	for _, test := range []struct {
		name  string
		ctx   func() context.Context
		err   error
		cause error
	}{
		{
			name:  "Background",
			ctx:   context.Background,
			err:   nil,
			cause: nil,
		},
		{
			name:  "TODO",
			ctx:   context.TODO,
			err:   nil,
			cause: nil,
		},
		{
			name: "WithCancel",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			err:   context.Canceled,
			cause: context.Canceled,
		},
		{
			name: "WithCancelCause",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancelCause(context.Background())
				cancel(parentCause)
				return ctx
			},
			err:   context.Canceled,
			cause: parentCause,
		},
		{
			name: "WithCancelCause nil",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancelCause(context.Background())
				cancel(nil)
				return ctx
			},
			err:   context.Canceled,
			cause: context.Canceled,
		},
		{
			name: "WithCancelCause: parent cause before child",
			ctx: func() context.Context {
				ctx, cancelParent := context.WithCancelCause(context.Background())
				ctx, cancelChild := context.WithCancelCause(ctx)
				cancelParent(parentCause)
				cancelChild(childCause)
				return ctx
			},
			err:   context.Canceled,
			cause: parentCause,
		},
		{
			name: "WithCancelCause: parent cause after child",
			ctx: func() context.Context {
				ctx, cancelParent := context.WithCancelCause(context.Background())
				ctx, cancelChild := context.WithCancelCause(ctx)
				cancelChild(childCause)
				cancelParent(parentCause)
				return ctx
			},
			err:   context.Canceled,
			cause: childCause,
		},
		{
			name: "WithCancelCause: parent cause before nil",
			ctx: func() context.Context {
				ctx, cancelParent := context.WithCancelCause(context.Background())
				ctx, cancelChild := context.WithCancel(ctx)
				cancelParent(parentCause)
				cancelChild()
				return ctx
			},
			err:   context.Canceled,
			cause: parentCause,
		},
		{
			name: "WithCancelCause: parent cause after nil",
			ctx: func() context.Context {
				ctx, cancelParent := context.WithCancelCause(context.Background())
				ctx, cancelChild := context.WithCancel(ctx)
				cancelChild()
				cancelParent(parentCause)
				return ctx
			},
			err:   context.Canceled,
			cause: context.Canceled,
		},
		{
			name: "WithCancelCause: child cause after nil",
			ctx: func() context.Context {
				ctx, cancelParent := context.WithCancel(context.Background())
				ctx, cancelChild := context.WithCancelCause(ctx)
				cancelParent()
				cancelChild(childCause)
				return ctx
			},
			err:   context.Canceled,
			cause: context.Canceled,
		},
		{
			name: "WithCancelCause: child cause before nil",
			ctx: func() context.Context {
				ctx, cancelParent := context.WithCancel(context.Background())
				ctx, cancelChild := context.WithCancelCause(ctx)
				cancelChild(childCause)
				cancelParent()
				return ctx
			},
			err:   context.Canceled,
			cause: childCause,
		},
		{
			name: "WithTimeout",
			ctx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 0)
				cancel()
				return ctx
			},
			err:   context.DeadlineExceeded,
			cause: context.DeadlineExceeded,
		},
		{
			name: "WithTimeout canceled",
			ctx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), forever)
				cancel()
				return ctx
			},
			err:   context.Canceled,
			cause: context.Canceled,
		},
		{
			name: "WithTimeoutCause",
			ctx: func() context.Context {
				ctx, cancel := context.WithTimeoutCause(context.Background(), 0, tooSlow)
				cancel()
				return ctx
			},
			err:   context.DeadlineExceeded,
			cause: tooSlow,
		},
		{
			name: "WithTimeoutCause canceled",
			ctx: func() context.Context {
				ctx, cancel := context.WithTimeoutCause(context.Background(), forever, tooSlow)
				cancel()
				return ctx
			},
			err:   context.Canceled,
			cause: context.Canceled,
		},
		{
			name: "WithTimeoutCause stacked",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancelCause(context.Background())
				ctx, _ = context.WithTimeoutCause(ctx, 0, tooSlow)
				cancel(finishedEarly)
				return ctx
			},
			err:   context.DeadlineExceeded,
			cause: tooSlow,
		},
		{
			name: "WithTimeoutCause stacked canceled",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancelCause(context.Background())
				ctx, _ = context.WithTimeoutCause(ctx, forever, tooSlow)
				cancel(finishedEarly)
				return ctx
			},
			err:   context.Canceled,
			cause: finishedEarly,
		},
		{
			name: "WithoutCancel",
			ctx: func() context.Context {
				return context.WithoutCancel(context.Background())
			},
			err:   nil,
			cause: nil,
		},
		{
			name: "WithoutCancel canceled",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancelCause(context.Background())
				ctx = context.WithoutCancel(ctx)
				cancel(finishedEarly)
				return ctx
			},
			err:   nil,
			cause: nil,
		},
		{
			name: "WithoutCancel timeout",
			ctx: func() context.Context {
				ctx, cancel := context.WithTimeoutCause(context.Background(), 0, tooSlow)
				ctx = context.WithoutCancel(ctx)
				cancel()
				return ctx
			},
			err:   nil,
			cause: nil,
		},
		{
			name: "parent of custom context not canceled",
			ctx: func() context.Context {
				ctx, _ := context.WithCancelCause(context.Background())
				ctx, cancel2 := newCustomContext(ctx)
				cancel2()
				return ctx
			},
			err:   context.Canceled,
			cause: context.Canceled,
		},
		{
			name: "parent of custom context is canceled before",
			ctx: func() context.Context {
				ctx, cancel1 := context.WithCancelCause(context.Background())
				ctx, cancel2 := newCustomContext(ctx)
				cancel1(parentCause)
				cancel2()
				return ctx
			},
			err:   context.Canceled,
			cause: parentCause,
		},
		{
			name: "parent of custom context is canceled after",
			ctx: func() context.Context {
				ctx, cancel1 := context.WithCancelCause(context.Background())
				ctx, cancel2 := newCustomContext(ctx)
				cancel2()
				cancel1(parentCause)
				return ctx
			},
			err: context.Canceled,
			// This isn't really right: the child context was canceled before
			// the parent context, and shouldn't inherit the parent's cause.
			// However, since the child is a custom context, Cause has no way
			// to tell which was canceled first and returns the parent's cause.
			cause: parentCause,
		},
	} {
		test := test
		t.Run(test.name, func(t *testT) {
			t.Parallel()
			ctx := test.ctx()
			atomic.AddInt64(&checks, 1)
			if got, want := ctx.Err(), test.err; want != got {
				t.Errorf("ctx.Err() = %v want %v", got, want)
			}
			atomic.AddInt64(&checks, 1)
			if got, want := context.Cause(ctx), test.cause; want != got {
				t.Errorf("Cause(ctx) = %v want %v", got, want)
			}
		})
	}
}

func TestCauseRace(t *testT) {
	defer func() { atomic.StoreInt64(&checks, -1) }() // the number of rounds depends on the scheduling
	cause := errors.New("TestCauseRace")
	ctx, cancel := context.WithCancelCause(context.Background())
	go func() {
		cancel(cause)
	}()
	for {
		// Poll Cause, rather than waiting for Done, to test that
		// access to the underlying cause is synchronized properly.
		atomic.AddInt64(&checks, 1)
		if err := context.Cause(ctx); err != nil {
			atomic.AddInt64(&checks, 1)
			if err != cause {
				t.Errorf("Cause returned %v, want %v", err, cause)
			}
			break
		}
		runtime.Gosched()
	}
}

func TestWithoutCancel(t *testT) {
	key, value := "key", "value"
	ctx := context.WithValue(context.Background(), key, value)
	ctx = context.WithoutCancel(ctx)
	atomic.AddInt64(&checks, 1)
	if d, ok := ctx.Deadline(); !d.IsZero() || ok != false {
		t.Errorf("ctx.Deadline() = %v, %v want zero, false", d, ok)
	}
	atomic.AddInt64(&checks, 1)
	if done := ctx.Done(); done != nil {
		t.Errorf("ctx.Deadline() = %v want nil", done)
	}
	atomic.AddInt64(&checks, 1)
	if err := ctx.Err(); err != nil {
		t.Errorf("ctx.Err() = %v want nil", err)
	}
	atomic.AddInt64(&checks, 1)
	if v := ctx.Value(key); v != value {
		t.Errorf("ctx.Value(%q) = %q want %q", key, v, value)
	}
}

type customDoneContext struct {
	context.Context
	donec chan struct{}
}

func (c *customDoneContext) Done() <-chan struct{} {
	return c.donec
}

func TestCustomContextPropagation(t *testT) {
	cause := errors.New("TestCustomContextPropagation")
	donec := make(chan struct{})
	ctx1, cancel1 := context.WithCancelCause(context.Background())
	ctx2 := &customDoneContext{
		Context: ctx1,
		donec:   donec,
	}
	ctx3, cancel3 := context.WithCancel(ctx2)
	defer cancel3()

	cancel1(cause)
	close(donec)

	<-ctx3.Done()
	atomic.AddInt64(&checks, 1)
	if got, want := ctx3.Err(), context.Canceled; got != want {
		t.Errorf("child not canceled; got = %v, want = %v", got, want)
	}
	atomic.AddInt64(&checks, 1)
	if got, want := context.Cause(ctx3), cause; got != want {
		t.Errorf("child has wrong cause; got = %v, want = %v", got, want)
	}
}

// customCauseContext is a custom Context used to test context.Cause.
type customCauseContext struct {
	mu   sync.Mutex
	done chan struct{}
	err  error

	cancelChild context.CancelFunc
}

func (ccc *customCauseContext) Deadline() (deadline time.Time, ok bool) {
	return
}

func (ccc *customCauseContext) Done() <-chan struct{} {
	ccc.mu.Lock()
	defer ccc.mu.Unlock()
	return ccc.done
}

func (ccc *customCauseContext) Err() error {
	ccc.mu.Lock()
	defer ccc.mu.Unlock()
	return ccc.err
}

func (ccc *customCauseContext) Value(key any) any {
	return nil
}

func (ccc *customCauseContext) cancel() {
	ccc.mu.Lock()
	ccc.err = context.Canceled
	close(ccc.done)
	cancelChild := ccc.cancelChild
	ccc.mu.Unlock()

	atomic.AddInt64(&checks, 1)
	if cancelChild != nil {
		cancelChild()
	}
}

func (ccc *customCauseContext) setCancelChild(cancelChild context.CancelFunc) {
	ccc.cancelChild = cancelChild
}

func TestCustomContextCause(t *testT) {
	// Test if we cancel a custom context, Err and Cause return Canceled.
	ccc := &customCauseContext{
		done: make(chan struct{}),
	}
	ccc.cancel()
	atomic.AddInt64(&checks, 1)
	if got := ccc.Err(); got != context.Canceled {
		t.Errorf("ccc.Err() = %v, want %v", got, context.Canceled)
	}
	atomic.AddInt64(&checks, 1)
	if got := context.Cause(ccc); got != context.Canceled {
		t.Errorf("Cause(ccc) = %v, want %v", got, context.Canceled)
	}

	// Test that if we pass a custom context to WithCancelCause,
	// and then cancel that child context with a cause,
	// that the cause of the child canceled context is correct
	// but that the parent custom context is not canceled.
	ccc = &customCauseContext{
		done: make(chan struct{}),
	}
	ctx, causeFunc := context.WithCancelCause(ccc)
	cause := errors.New("TestCustomContextCause")
	causeFunc(cause)
	atomic.AddInt64(&checks, 1)
	if got := ctx.Err(); got != context.Canceled {
		t.Errorf("after CancelCauseFunc ctx.Err() = %v, want %v", got, context.Canceled)
	}
	atomic.AddInt64(&checks, 1)
	if got := context.Cause(ctx); got != cause {
		t.Errorf("after CancelCauseFunc Cause(ctx) = %v, want %v", got, cause)
	}
	atomic.AddInt64(&checks, 1)
	if got := ccc.Err(); got != nil {
		t.Errorf("after CancelCauseFunc ccc.Err() = %v, want %v", got, nil)
	}
	atomic.AddInt64(&checks, 1)
	if got := context.Cause(ccc); got != nil {
		t.Errorf("after CancelCauseFunc Cause(ccc) = %v, want %v", got, nil)
	}

	// Test that if we now cancel the parent custom context,
	// the cause of the child canceled context is still correct,
	// and the parent custom context is canceled without a cause.
	ccc.cancel()
	atomic.AddInt64(&checks, 1)
	if got := ctx.Err(); got != context.Canceled {
		t.Errorf("after CancelCauseFunc ctx.Err() = %v, want %v", got, context.Canceled)
	}
	atomic.AddInt64(&checks, 1)
	if got := context.Cause(ctx); got != cause {
		t.Errorf("after CancelCauseFunc Cause(ctx) = %v, want %v", got, cause)
	}
	atomic.AddInt64(&checks, 1)
	if got := ccc.Err(); got != context.Canceled {
		t.Errorf("after CancelCauseFunc ccc.Err() = %v, want %v", got, context.Canceled)
	}
	atomic.AddInt64(&checks, 1)
	if got := context.Cause(ccc); got != context.Canceled {
		t.Errorf("after CancelCauseFunc Cause(ccc) = %v, want %v", got, context.Canceled)
	}

	// Test that if we associate a custom context with a child,
	// then canceling the custom context cancels the child.
	ccc = &customCauseContext{
		done: make(chan struct{}),
	}
	ctx, cancelFunc := context.WithCancel(ccc)
	ccc.setCancelChild(cancelFunc)
	ccc.cancel()
	atomic.AddInt64(&checks, 1)
	if got := ctx.Err(); got != context.Canceled {
		t.Errorf("after CancelCauseFunc ctx.Err() = %v, want %v", got, context.Canceled)
	}
	atomic.AddInt64(&checks, 1)
	if got := context.Cause(ctx); got != context.Canceled {
		t.Errorf("after CancelCauseFunc Cause(ctx) = %v, want %v", got, context.Canceled)
	}
	atomic.AddInt64(&checks, 1)
	if got := ccc.Err(); got != context.Canceled {
		t.Errorf("after CancelCauseFunc ccc.Err() = %v, want %v", got, context.Canceled)
	}
	atomic.AddInt64(&checks, 1)
	if got := context.Cause(ccc); got != context.Canceled {
		t.Errorf("after CancelCauseFunc Cause(ccc) = %v, want %v", got, context.Canceled)
	}
}

func TestAfterFuncCalledAfterCancel(t *testT) {
	ctx, cancel := context.WithCancel(context.Background())
	donec := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		close(donec)
	})
	select {
	case <-donec:
		t.Fatalf("AfterFunc called before context is done")
	case <-time.After(shortDuration):
	}
	cancel()
	select {
	case <-donec:
	case <-time.After(veryLongDuration):
		t.Fatalf("AfterFunc not called after context is canceled")
	}
	atomic.AddInt64(&checks, 1)
	if stop() {
		t.Fatalf("stop() = true, want false")
	}
}

func TestAfterFuncCalledAfterTimeout(t *testT) {
	ctx, cancel := context.WithTimeout(context.Background(), shortDuration)
	defer cancel()
	donec := make(chan struct{})
	context.AfterFunc(ctx, func() {
		close(donec)
	})
	select {
	case <-donec:
	case <-time.After(veryLongDuration):
		t.Fatalf("AfterFunc not called after context is canceled")
	}
}

func TestAfterFuncCalledImmediately(t *testT) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	donec := make(chan struct{})
	context.AfterFunc(ctx, func() {
		close(donec)
	})
	select {
	case <-donec:
	case <-time.After(veryLongDuration):
		t.Fatalf("AfterFunc not called for already-canceled context")
	}
}

func TestAfterFuncNotCalledAfterStop(t *testT) {
	ctx, cancel := context.WithCancel(context.Background())
	donec := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		close(donec)
	})
	atomic.AddInt64(&checks, 1)
	if !stop() {
		t.Fatalf("stop() = false, want true")
	}
	cancel()
	select {
	case <-donec:
		t.Fatalf("AfterFunc called for already-canceled context")
	case <-time.After(shortDuration):
	}
	atomic.AddInt64(&checks, 1)
	if stop() {
		t.Fatalf("stop() = true, want false")
	}
}

// This test verifies that canceling a context does not block waiting for AfterFuncs to finish.
func TestAfterFuncCalledAsynchronously(t *testT) {
	ctx, cancel := context.WithCancel(context.Background())
	donec := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		// The channel send blocks until donec is read from.
		donec <- struct{}{}
	})
	defer stop()
	cancel()
	// After cancel returns, read from donec and unblock the AfterFunc.
	select {
	case <-donec:
	case <-time.After(veryLongDuration):
		t.Fatalf("AfterFunc not called after context is canceled")
	}
}

// customContext is a custom Context implementation.
type customContext struct {
	parent context.Context

	doneOnce sync.Once
	donec    chan struct{}
	err      error
}

func newCustomContext(parent context.Context) (context.Context, context.CancelFunc) {
	c := &customContext{
		parent: parent,
		donec:  make(chan struct{}),
	}
	context.AfterFunc(parent, func() {
		c.doneOnce.Do(func() {
			c.err = parent.Err()
			close(c.donec)
		})
	})
	return c, func() {
		c.doneOnce.Do(func() {
			c.err = context.Canceled
			close(c.donec)
		})
	}
}

func (c *customContext) Deadline() (time.Time, bool) {
	return c.parent.Deadline()
}

func (c *customContext) Done() <-chan struct{} {
	return c.donec
}

func (c *customContext) Err() error {
	select {
	case <-c.donec:
		return c.err
	default:
		return nil
	}
}

func (c *customContext) Value(key any) any {
	return c.parent.Value(key)
}

// Issue #75533.
func TestContextErrDoneRace(t *testT) {
	defer func() { atomic.StoreInt64(&checks, -1) }() // the number of rounds depends on the scheduling
	// 4 iterations reliably reproduced #75533.
	for range 10 {
		ctx, cancel := context.WithCancel(context.Background())
		donec := ctx.Done()
		go cancel()
		for ctx.Err() == nil {
			atomic.AddInt64(&checks, 1)
			if runtime.GOARCH == "wasm" {
				runtime.Gosched() // need to explicitly yield
			}
		}
		select {
		case <-donec:
		default:
			t.Fatalf("ctx.Err is non-nil, but ctx.Done is not closed")
		}
	}
}

// ---- a small harness instead of package testing (step 9.9); Go's tests are below, unchanged except as listed above ----

type testT struct {
	name   string
	failed bool
}

type fatalExit struct{}
type skipExit struct{}

var subtests int

// checks counts the if statements that the tests of the file run: a Test that returns early shows in the expected output.
// It is atomic: some tests run their checks in several goroutines.
var checks int64

func (t *testT) Helper()                          {}
func (t *testT) Parallel()                        {}
func (t *testT) Name() string                     { return t.name }
func (t *testT) Log(args ...any)                  {}
func (t *testT) Logf(format string, args ...any)  {}
func (t *testT) Skip(args ...any)                 { panic(skipExit{}) }
func (t *testT) Skipf(f string, args ...any)      { panic(skipExit{}) }
func (t *testT) report(msg string)                { t.failed = true; fmt.Println("FAIL", t.name+":", msg) }
func (t *testT) Errorf(format string, args ...any) { t.report(fmt.Sprintf(format, args...)) }
func (t *testT) Error(args ...any)                { t.report(fmt.Sprint(args...)) }
func (t *testT) Fatalf(format string, args ...any) {
	t.Errorf(format, args...)
	panic(fatalExit{})
}
func (t *testT) Fatal(args ...any) {
	t.Error(args...)
	panic(fatalExit{})
}

func (t *testT) Run(name string, f func(t *testT)) bool {
	sub := &testT{name: t.name + "/" + name}
	subtests++
	runTest(sub, f)
	if sub.failed {
		t.failed = true
	}
	return !sub.failed
}

func runTest(t *testT, f func(t *testT)) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case fatalExit, skipExit:
			default:
				t.failed = true
				fmt.Println("PANIC", t.name+":", r)
			}
		}
	}()
	f(t)
}

type testCase struct {
	name string
	f    func(t *testT)
}

func runTests(tests []testCase) {
	failed := 0
	for _, tc := range tests {
		t := &testT{name: tc.name}
		subtests = 0
		atomic.StoreInt64(&checks, 0)
		runTest(t, tc.f)
		if t.failed {
			failed++
			fmt.Println("--- FAIL:", tc.name)
		} else {
			fmt.Println("ok", tc.name, "subtests:", subtests, "checks:", atomic.LoadInt64(&checks))
		}
	}
	fmt.Println("tests:", len(tests), "failed:", failed)
	if failed > 0 {
		panic("tests failed")
	}
}

// allocsPerRun runs f runs times and returns 0: gocpp does not count heap allocations (the tests of allocation counts are
// dropped or their comparison is).
func allocsPerRun(runs int, f func()) float64 {
	for i := 0; i < runs; i++ {
		f()
	}
	return 0
}

// A deterministic generator (splitmix64) in place of math/rand, so that Go and gocpp run the same inputs.
type rngT struct{ s uint64 }

var rng = &rngT{0x9E3779B97F4A7C15}

func (r *rngT) Uint64() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
func (r *rngT) Uint32() uint32 { return uint32(r.Uint64() >> 32) }
func (r *rngT) Intn(n int) int { return int(r.Uint64()>>1) % n }
func (r *rngT) Float64() float64 { return float64(r.Uint64()>>11) / (1 << 53) }

func main() {
	tests_ := []testCase{
		{"TestBackground", TestBackground},
		{"TestTODO", TestTODO},
		{"TestWithCancel", TestWithCancel},
		{"TestDeadline", TestDeadline},
		{"TestTimeout", TestTimeout},
		{"TestCanceledTimeout", TestCanceledTimeout},
		{"TestValues", TestValues},
		{"TestAllocs", TestAllocs},
		{"TestSimultaneousCancels", TestSimultaneousCancels},
		{"TestInterlockedCancels", TestInterlockedCancels},
		{"TestLayersCancel", TestLayersCancel},
		{"TestLayersTimeout", TestLayersTimeout},
		{"TestWithCancelCanceledParent", TestWithCancelCanceledParent},
		{"TestWithCancelSimultaneouslyCanceledParent", TestWithCancelSimultaneouslyCanceledParent},
		{"TestWithValueChecksKey", TestWithValueChecksKey},
		{"TestInvalidDerivedFail", TestInvalidDerivedFail},
		{"TestDeadlineExceededSupportsTimeout", TestDeadlineExceededSupportsTimeout},
		{"TestCause", TestCause},
		{"TestCauseRace", TestCauseRace},
		{"TestWithoutCancel", TestWithoutCancel},
		{"TestCustomContextPropagation", TestCustomContextPropagation},
		{"TestCustomContextCause", TestCustomContextCause},
		{"TestAfterFuncCalledAfterCancel", TestAfterFuncCalledAfterCancel},
		{"TestAfterFuncCalledAfterTimeout", TestAfterFuncCalledAfterTimeout},
		{"TestAfterFuncCalledImmediately", TestAfterFuncCalledImmediately},
		{"TestAfterFuncNotCalledAfterStop", TestAfterFuncNotCalledAfterStop},
		{"TestAfterFuncCalledAsynchronously", TestAfterFuncCalledAsynchronously},
		{"TestContextErrDoneRace", TestContextErrDoneRace},
	}
	runTests(tests_)
}

// Output:
// ok TestBackground subtests: 0 checks: 2
// ok TestTODO subtests: 0 checks: 2
// ok TestWithCancel subtests: 0 checks: 10
// ok TestDeadline subtests: 0 checks: 11
// ok TestTimeout subtests: 0 checks: 7
// ok TestCanceledTimeout subtests: 0 checks: 1
// ok TestValues subtests: 0 checks: 33
// ok TestAllocs subtests: 0 checks: 16
// ok TestSimultaneousCancels subtests: 0 checks: 1
// ok TestInterlockedCancels subtests: 0 checks: 1
// ok TestLayersCancel subtests: 0 checks: 651
// ok TestLayersTimeout subtests: 0 checks: 671
// ok TestWithCancelCanceledParent subtests: 0 checks: 2
// ok TestWithCancelSimultaneouslyCanceledParent subtests: 0 checks: 200
// ok TestWithValueChecksKey subtests: 0 checks: 2
// ok TestInvalidDerivedFail subtests: 0 checks: 3
// ok TestDeadlineExceededSupportsTimeout subtests: 0 checks: 2
// ok TestCause subtests: 23 checks: 46
// ok TestCauseRace subtests: 0 checks: -1
// ok TestWithoutCancel subtests: 0 checks: 4
// ok TestCustomContextPropagation subtests: 0 checks: 2
// ok TestCustomContextCause subtests: 0 checks: 17
// ok TestAfterFuncCalledAfterCancel subtests: 0 checks: 1
// ok TestAfterFuncCalledAfterTimeout subtests: 0 checks: 0
// ok TestAfterFuncCalledImmediately subtests: 0 checks: 0
// ok TestAfterFuncNotCalledAfterStop subtests: 0 checks: 2
// ok TestAfterFuncCalledAsynchronously subtests: 0 checks: 0
// ok TestContextErrDoneRace subtests: 0 checks: -1
// tests: 28 failed: 0
