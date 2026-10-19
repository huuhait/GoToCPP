// Package trace is the language server's tiny span recorder (Phase 11,
// step 11.12): no OpenTelemetry, no dependencies. A request (or another
// piece of work: a diagnostics run, a namespace probe) is a root span; the
// steps inside it (loading a package, type-checking it, reading a summary
// from disk) are spans that Start makes from the context. Two things come
// out of them:
//
//   - the breakdown of a root span (Span.End returns it): the time spent
//     in each kind of span, which the slow-request log prints;
//   - when a Recorder is on (gocpp lsp -trace file), every span as an
//     event of the Chrome trace format, for chrome://tracing, Perfetto or
//     speedscope (Recorder.WriteChrome).
//
// The cost of a span is small (a few hundred nanoseconds), and Start does
// nothing outside a root span, so the steps may call it freely. Everything
// here is safe for concurrent use. A nil *Span is a span that does
// nothing.
package trace

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// A Recorder keeps the finished spans, up to a maximum, for WriteChrome.
type Recorder struct {
	start   time.Time
	max     int
	nextTID atomic.Int64

	mu      sync.Mutex
	events  []event
	names   map[int64]string // the name of each root span's row
	dropped int
}

// An event is a finished span: a "complete" event of the Chrome trace
// format.
type event struct {
	name  string
	start time.Time
	dur   time.Duration
	tid   int64
	args  []any // key, value, key, value, ...
}

// NewRecorder returns a recorder that keeps up to max spans (later ones
// are counted and dropped; 0 means a million).
func NewRecorder(max int) *Recorder {
	if max <= 0 {
		max = 1 << 20
	}
	return &Recorder{start: time.Now(), max: max, names: make(map[int64]string)}
}

// Len returns the number of spans kept, and how many were dropped.
func (r *Recorder) Len() (kept, dropped int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events), r.dropped
}

func (r *Recorder) add(e event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) >= r.max {
		r.dropped++
		return
	}
	r.events = append(r.events, e)
}

// WriteChrome writes the spans as Chrome trace JSON (the "JSON object
// format": {"traceEvents": [...]}), sorted by start time. Each root span
// has its own row (tid), named by the root's name.
func (r *Recorder) WriteChrome(w io.Writer) error {
	r.mu.Lock()
	events := slices.Clone(r.events)
	names := maps.Clone(r.names)
	dropped := r.dropped
	r.mu.Unlock()
	sort.SliceStable(events, func(i, j int) bool { return events[i].start.Before(events[j].start) })
	type chrome struct {
		Name string         `json:"name"`
		Cat  string         `json:"cat,omitempty"`
		Ph   string         `json:"ph"`
		Ts   float64        `json:"ts"` // microseconds since the start of the recorder
		Dur  float64        `json:"dur,omitempty"`
		Pid  int            `json:"pid"`
		Tid  int64          `json:"tid"`
		Args map[string]any `json:"args,omitempty"`
	}
	out := make([]chrome, 0, len(events)+1)
	out = append(out, chrome{Name: "process_name", Ph: "M", Pid: 1, Args: map[string]any{"name": "gocpp lsp"}})
	for _, tid := range slices.Sorted(maps.Keys(names)) {
		out = append(out, chrome{Name: "thread_name", Ph: "M", Pid: 1, Tid: tid, Args: map[string]any{"name": names[tid]}})
	}
	for _, e := range events {
		c := chrome{Name: e.name, Ph: "X", Pid: 1, Tid: e.tid,
			Ts:  float64(e.start.Sub(r.start).Nanoseconds()) / 1e3,
			Dur: max(float64(e.dur.Nanoseconds())/1e3, 0.001)}
		if len(e.args) > 1 {
			c.Args = make(map[string]any, len(e.args)/2)
			for i := 0; i+1 < len(e.args); i += 2 {
				if k, ok := e.args[i].(string); ok {
					c.Args[k] = e.args[i+1]
				}
			}
		}
		out = append(out, c)
	}
	enc := json.NewEncoder(w)
	return enc.Encode(map[string]any{"traceEvents": out, "displayTimeUnit": "ms", "droppedEvents": dropped})
}

type ctxKey int

const (
	recorderKey ctxKey = iota
	spanKey
)

// WithRecorder returns a context whose root spans are recorded by r (nil:
// not recorded).
func WithRecorder(ctx context.Context, r *Recorder) context.Context {
	return context.WithValue(ctx, recorderKey, r)
}

// A root is the part of a root span that its children share.
type root struct {
	tid  int64
	rec  *Recorder
	name string

	mu     sync.Mutex
	phases map[string]*phase
	order  []string // the names of phases in the order they first ended
}

type phase struct {
	total time.Duration
	n     int
}

// A Span is a piece of work that has a start and an end.
type Span struct {
	root  *root
	name  string
	start time.Time
	args  []any
	top   bool // the root span itself
	ended atomic.Bool
}

// Begin starts a root span and returns a context in which Start makes its
// children. kv are key and value pairs for the Chrome trace (for example
// "uri", "file:///a.gp"); they are not copied.
func Begin(ctx context.Context, name string, kv ...any) (context.Context, *Span) {
	rec, _ := ctx.Value(recorderKey).(*Recorder)
	rt := &root{name: name, rec: rec, phases: make(map[string]*phase)}
	if rec != nil {
		rt.tid = rec.nextTID.Add(1)
		rec.mu.Lock()
		rec.names[rt.tid] = name
		rec.mu.Unlock()
	}
	s := &Span{root: rt, name: name, start: time.Now(), args: kv, top: true}
	return context.WithValue(ctx, spanKey, s), s
}

// Start starts a span inside the root span of ctx, and returns a context
// for its own children. Outside a root span it does nothing and returns a
// nil span.
func Start(ctx context.Context, name string, kv ...any) (context.Context, *Span) {
	parent, _ := ctx.Value(spanKey).(*Span)
	if parent == nil {
		return ctx, nil
	}
	s := &Span{root: parent.root, name: name, start: time.Now(), args: kv}
	return context.WithValue(ctx, spanKey, s), s
}

// Set adds key and value pairs to the span (for the Chrome trace).
func (s *Span) Set(kv ...any) {
	if s == nil {
		return
	}
	s.args = append(s.args, kv...)
}

// A Phase is the time spent in the spans of one name inside a root span.
type Phase struct {
	Name  string
	Total time.Duration
	Count int
}

// A Summary is what a root span did.
type Summary struct {
	Name   string
	Total  time.Duration
	Phases []Phase // by total time, longest first
}

// End ends the span. For a root span it returns what its children did (the
// phases: the spans of one name add up, and spans of different names may
// overlap, for example a "check" with its "typecheck"); for another span the
// summary is empty. A second End does nothing.
func (s *Span) End() Summary {
	if s == nil || !s.ended.CompareAndSwap(false, true) {
		return Summary{}
	}
	d := time.Since(s.start)
	rt := s.root
	if rt.rec != nil {
		rt.rec.add(event{name: s.name, start: s.start, dur: d, tid: rt.tid, args: s.args})
	}
	if !s.top {
		rt.mu.Lock()
		p := rt.phases[s.name]
		if p == nil {
			p = new(phase)
			rt.phases[s.name] = p
			rt.order = append(rt.order, s.name)
		}
		p.total += d
		p.n++
		rt.mu.Unlock()
		return Summary{}
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	sum := Summary{Name: s.name, Total: d}
	for _, name := range rt.order {
		p := rt.phases[name]
		sum.Phases = append(sum.Phases, Phase{Name: name, Total: p.total, Count: p.n})
	}
	sort.SliceStable(sum.Phases, func(i, j int) bool { return sum.Phases[i].Total > sum.Phases[j].Total })
	return sum
}
