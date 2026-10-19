package build

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// The executor (Phase 06 step 6.7). The compiles of a build (the generated units, the package's
// own C and C++ files, the sources of the libraries of the modules, the runtime) are actions that
// runActions runs, at most Builder.Jobs at a time (gocpp build -p N, GOMAXPROCS by default, like
// go build). The rules:
//
//   - Actions are started in the order of the list, so with -p 1 they run one after the other in
//     that order, and the list puts the user's own C and C++ files first, then the libraries,
//     then the generated units, by package.
//   - A failure does not stop the others: every action that can run does, so one build reports
//     all the errors that it can (go build does the same for packages that do not depend on each
//     other). Only after maxFailedActions failures nothing new is started, and the error says how
//     many actions were left out; a canceled context (^C) stops the starting too.
//   - What an action printed when it succeeded (the warnings of the compiler) is written in the
//     order of the list, not in the order of the end, so the output of a build does not depend on
//     which compile was faster; an action's text comes out as soon as every action before it has
//     finished.
//   - The errors come back in the order of the list too (Errors), whatever the order of the end.
//
// The link is one more action that needs all the others; it runs only when none of them failed.

// DefaultJobs returns what -p is without a value: GOMAXPROCS, like go build.
func DefaultJobs() int { return max(1, runtime.GOMAXPROCS(0)) }

// maxFailedActions is how many actions of a build may fail before no new one is started. A
// variable, for the tests.
var maxFailedActions = 20

// Stats counts the actions that a Builder ran, for gocpp build -x and for the tests: they count
// operations, not time. The fields are atomic: actions run at the same time.
type Stats struct {
	Compiles        atomic.Int64 // compiler runs for a unit, a C or C++ file or a library source (a miss of the object cache)
	Hits            atomic.Int64 // compiles that the object cache answered
	Links           atomic.Int64 // runs of the linker
	RuntimeCompiles atomic.Int64 // compiler runs for the runtime's files
}

// Reset sets the counts to zero (a build that is made again after its work folder was removed).
func (s *Stats) Reset() {
	s.Compiles.Store(0)
	s.Hits.Store(0)
	s.Links.Store(0)
	s.RuntimeCompiles.Store(0)
}

func (s *Stats) compiled() {
	if s != nil {
		s.Compiles.Add(1)
	}
}

func (s *Stats) hit() {
	if s != nil {
		s.Hits.Add(1)
	}
}

func (s *Stats) linked() {
	if s != nil {
		s.Links.Add(1)
	}
}

func (s *Stats) runtimeCompiled() {
	if s != nil {
		s.RuntimeCompiles.Add(1)
	}
}

// String returns the counts as the words of the "# actions:" line of -x: compile=2 hit=3 link=1.
func (s *Stats) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("compile=%d hit=%d link=%d runtime=%d", s.Compiles.Load(), s.Hits.Load(), s.Links.Load(), s.RuntimeCompiles.Load())
}

// An action is one job of the executor: name says what it is, for messages, and run does it. flush,
// when it is not nil, writes what the action printed; the executor calls it in the order of the
// list, once, whether the action succeeded or failed.
type action struct {
	name string
	run  func() (flush func(), err error)
}

// Errors is the error of a build in which more than one action failed: the errors in the order of
// the actions. A build with one failure returns that error itself.
type Errors struct {
	Errs []error

	// NotStarted is the number of actions that were left out because of the failures (see
	// maxFailedActions).
	NotStarted int
}

func (e *Errors) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d errors", len(e.Errs))
	for _, err := range e.Errs {
		b.WriteString("; ")
		b.WriteString(err.Error())
	}
	if e.NotStarted > 0 {
		fmt.Fprintf(&b, "; %d more actions were not started", e.NotStarted)
	}
	return b.String()
}

func (e *Errors) Unwrap() []error { return e.Errs }

// Flatten returns the errors that err holds: those of an *Errors, or err itself, or nil for nil.
func Flatten(err error) []error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Errors); ok {
		return e.Errs
	}
	return []error{err}
}

// jobs returns the number of actions that run at the same time.
func (b *Builder) jobs() int {
	if b.Jobs > 0 {
		return b.Jobs
	}
	return DefaultJobs()
}

// runActions runs acts with at most b.jobs of them at the same time (see the comment at the top)
// and returns their errors in the order of the list, nil for an action that succeeded or was not
// started, and the number of actions that were not started.
func (b *Builder) runActions(ctx context.Context, acts []action) (errs []error, notStarted int) {
	errs = make([]error, len(acts))
	var (
		mu      sync.Mutex
		next    int
		failed  int
		done    = make([]bool, len(acts))
		flushes = make([]func(), len(acts))
		flushed int
	)
	worker := func() {
		for {
			mu.Lock()
			if next >= len(acts) || ctx.Err() != nil || failed >= maxFailedActions {
				mu.Unlock()
				return
			}
			i := next
			next++
			mu.Unlock()
			flush, err := acts[i].run()
			mu.Lock()
			errs[i], done[i], flushes[i] = err, true, flush
			if err != nil {
				failed++
			}
			for flushed < len(acts) && done[flushed] {
				if f := flushes[flushed]; f != nil {
					f()
				}
				flushes[flushed] = nil
				flushed++
			}
			mu.Unlock()
		}
	}
	var wg sync.WaitGroup
	for range min(b.jobs(), len(acts)) {
		wg.Go(worker)
	}
	wg.Wait()
	// What is left: the actions after one that was not started.
	for i := flushed; i < len(acts); i++ {
		if !done[i] {
			notStarted++
		} else if f := flushes[i]; f != nil {
			f()
		}
	}
	return errs, notStarted
}

// joinErrors returns the errors of errs that are not nil, in order: nil for none, the error itself
// for one, an *Errors for more.
func joinErrors(notStarted int, errs ...error) error {
	var list []error
	for _, err := range errs {
		if err != nil {
			list = append(list, err)
		}
	}
	switch {
	case len(list) == 0:
		return nil
	case len(list) == 1 && notStarted == 0:
		return list[0]
	}
	return &Errors{Errs: list, NotStarted: notStarted}
}
