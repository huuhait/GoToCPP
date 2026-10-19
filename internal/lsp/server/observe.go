package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/lsp/trace"
	"github.com/huuhait/gocpp/internal/version"
)

// Observability (Phase 11 README §2.5, step 11.12): the slow-request log,
// the span recorder behind gocpp lsp -trace, and the request gocpp/stats
// (also the command gocpp.stats of workspace/executeCommand) with the cache
// hit rates, the snapshots and the memory.
//
// Every request, every notification and every diagnostics run is a root
// span (internal/lsp/trace): the steps inside it (loading, type-checking,
// reading a summary from disk, parsing) are spans made from its context. A
// piece of work that takes the slow time or more is logged with its
// breakdown, so a bug report about "completion took a second" says where
// the second went. Each kind of work (a request method, a notification, the
// diagnostics runs) also counts its latencies for gocpp/stats.

// DefaultSlow is the default time from which a piece of work is logged as
// slow (gocpp lsp -slow).
const DefaultSlow = 500 * time.Millisecond

// Options are the settings of a server that the tests and gocpp lsp
// choose; the zero value has the defaults, except that Slow 0 means
// DefaultSlow (negative: the slow log is off).
type Options struct {
	// Slow is the time from which a request, a notification or a
	// diagnostics run is logged with its breakdown.
	Slow time.Duration
	// Trace, if not nil, records every span (gocpp lsp -trace).
	Trace *trace.Recorder
	// Phase is check.Config.Phase, the phase of the gates, as gocpp check
	// has it (version.Phase, or later with GOCPP_GATE_PHASE).
	Phase int
}

// StatsCommand is the command of workspace/executeCommand that returns the
// stats, as the request GocppStats does.
const StatsCommand = "gocpp.stats"

// GocppStats is the method of the request that returns the Stats.
const GocppStats = "gocpp/stats"

// observer counts and logs the work of a server.
type observer struct {
	s       *Server
	slow    time.Duration // <= 0: no slow log
	rec     *trace.Recorder
	started time.Time

	mu      sync.Mutex
	methods map[string]*methodStats
}

// A methodStats counts the latencies of one kind of work.
type methodStats struct {
	count, slow int64
	total, max  time.Duration
	recent      [recentLatencies]time.Duration // a ring of the last latencies
	next        int
}

const recentLatencies = 256

func newObserver(s *Server, o Options) *observer {
	slow := o.Slow
	if slow == 0 {
		slow = DefaultSlow
	}
	return &observer{s: s, slow: slow, rec: o.Trace, started: time.Now(), methods: make(map[string]*methodStats)}
}

// begin starts the root span of a piece of work called name. The context
// has the recorder, so the steps inside make spans.
func (o *observer) begin(ctx context.Context, name string, kv ...any) (context.Context, *trace.Span) {
	return trace.Begin(trace.WithRecorder(ctx, o.rec), name, kv...)
}

// end ends the root span, counts the work, and logs it when it was slow.
// info is called only then: it says what the work was about (the document,
// the snapshot).
func (o *observer) end(span *trace.Span, info func() string) {
	sum := span.End()
	o.mu.Lock()
	st := o.methods[sum.Name]
	if st == nil {
		st = new(methodStats)
		o.methods[sum.Name] = st
	}
	st.count++
	st.total += sum.Total
	st.max = max(st.max, sum.Total)
	st.recent[st.next%recentLatencies] = sum.Total
	st.next++
	isSlow := o.slow > 0 && sum.Total >= o.slow
	if isSlow {
		st.slow++
	}
	o.mu.Unlock()
	if isSlow {
		o.s.log.Print(slowMessage(sum, o.slow, info()))
	}
}

// slowMessage is the line of the slow log: what took long, how long, what
// it was about, and the time of its phases (the spans of one name add up;
// spans of different names may overlap: "check" holds "typecheck").
func slowMessage(sum trace.Summary, threshold time.Duration, info string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "slow: %s took %v (over %v)", sum.Name, sum.Total.Round(time.Microsecond), threshold)
	if info != "" {
		b.WriteString(" ")
		b.WriteString(info)
	}
	if len(sum.Phases) > 0 {
		b.WriteString("; phases:")
		for i, p := range sum.Phases {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, " %s %v", p.Name, p.Total.Round(time.Microsecond))
			if p.Count > 1 {
				fmt.Fprintf(&b, " (x%d)", p.Count)
			}
		}
	}
	return b.String()
}

// snapshotInfo describes the session for the slow log: the current
// snapshot, how many are alive, and the open documents.
func (s *Server) snapshotInfo() string {
	snap, release := s.session.Snapshot()
	defer release()
	_, _, checks := s.pipeline.Stats()
	return fmt.Sprintf("snapshot=%d live=%d overlays=%d checks=%d", snap.ID(), s.session.Live(), len(snap.Overlays()), checks)
}

// A Stats is the answer of gocpp/stats.
type Stats struct {
	Version       string  `json:"version"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
	Snapshots     struct {
		Current uint64 `json:"current"` // the ID of the current snapshot
		Live    int    `json:"live"`    // the snapshots not destroyed yet
		Open    int    `json:"openDocuments"`
	} `json:"snapshots"`
	Pipeline struct {
		Parses        int64                 `json:"parses"`
		MetadataLoads int64                 `json:"metadataLoads"`
		Checks        int64                 `json:"checks"`
		Caches        map[string]StatsCache `json:"caches"`
		Disk          struct {
			Hits   int64 `json:"hits"`
			Misses int64 `json:"misses"`
			Writes int64 `json:"writes"`
		} `json:"disk"`
	} `json:"pipeline"`
	Requests map[string]StatsMethod `json:"requests"`
	Memory   StatsMemory            `json:"memory"`
}

// StatsCache is one cache of the pipeline in Stats.
type StatsCache struct {
	Entries  int     `json:"entries"`
	Bytes    int64   `json:"bytes"`    // the estimated memory of the values
	MaxBytes int64   `json:"maxBytes"` // the cap, 0 for none
	Hits     int64   `json:"hits"`
	Misses   int64   `json:"misses"`
	HitRate  float64 `json:"hitRate"`
}

// StatsMethod is the latencies of one kind of work in Stats: a request
// method, a notification, or "diagnostics" (the runs of the diagnostics
// loop). P50Ms, P95Ms are over the last 256.
type StatsMethod struct {
	Count   int64   `json:"count"`
	Slow    int64   `json:"slow"` // the ones logged
	TotalMs float64 `json:"totalMs"`
	MaxMs   float64 `json:"maxMs"`
	P50Ms   float64 `json:"p50Ms"`
	P95Ms   float64 `json:"p95Ms"`
}

// StatsMemory is the memory of the server process in Stats.
type StatsMemory struct {
	HeapAlloc  uint64 `json:"heapAlloc"`
	HeapInuse  uint64 `json:"heapInuse"`
	Sys        uint64 `json:"sys"` // from the operating system, by the Go runtime
	NumGC      uint32 `json:"numGC"`
	Goroutines int    `json:"goroutines"`
	RSS        uint64 `json:"rss"`     // the resident set now, from the operating system (0 if unknown)
	PeakRSS    uint64 `json:"peakRss"` // its peak (0 if unknown)
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// stats collects the Stats.
func (s *Server) stats() *Stats {
	st := new(Stats)
	st.Version = version.Version
	st.UptimeSeconds = time.Since(s.obs.started).Seconds()
	snap, release := s.session.Snapshot()
	st.Snapshots.Current, st.Snapshots.Open = snap.ID(), len(snap.Overlays())
	release()
	st.Snapshots.Live = s.session.Live()
	m := s.pipeline.Metrics()
	st.Pipeline.Parses, st.Pipeline.MetadataLoads, st.Pipeline.Checks = m.Parses, m.MetadataLoads, m.Checks
	st.Pipeline.Disk.Hits, st.Pipeline.Disk.Misses, st.Pipeline.Disk.Writes = m.DiskHits, m.DiskMisses, m.DiskWrites
	st.Pipeline.Caches = make(map[string]StatsCache, len(m.Caches))
	for name, c := range m.Caches {
		st.Pipeline.Caches[name] = StatsCache{Entries: c.Entries, Bytes: c.Bytes, MaxBytes: c.MaxBytes, Hits: c.Hits, Misses: c.Misses, HitRate: c.HitRate()}
	}
	st.Requests = s.obs.methodStats()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	st.Memory = StatsMemory{HeapAlloc: ms.HeapAlloc, HeapInuse: ms.HeapInuse, Sys: ms.Sys, NumGC: ms.NumGC, Goroutines: runtime.NumGoroutine()}
	st.Memory.RSS, st.Memory.PeakRSS = processMemory()
	return st
}

// methodStats returns the counted work by name.
func (o *observer) methodStats() map[string]StatsMethod {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make(map[string]StatsMethod, len(o.methods))
	for name, st := range o.methods {
		n := min(st.next, recentLatencies)
		recent := slices.Clone(st.recent[:n])
		slices.Sort(recent)
		sm := StatsMethod{Count: st.count, Slow: st.slow, TotalMs: ms(st.total), MaxMs: ms(st.max)}
		if n > 0 {
			sm.P50Ms, sm.P95Ms = ms(recent[n/2]), ms(recent[min(n-1, n*95/100)])
		}
		out[name] = sm
	}
	return out
}

// processMemory returns the resident set of the process and its peak, from
// /proc/self/status (Linux); 0, 0 elsewhere.
func processMemory() (rss, peak uint64) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		var key string
		var kb uint64
		switch {
		case strings.HasPrefix(line, "VmRSS:"):
			key = "VmRSS:"
		case strings.HasPrefix(line, "VmHWM:"):
			key = "VmHWM:"
		default:
			continue
		}
		if _, err := fmt.Sscanf(strings.TrimPrefix(line, key), "%d", &kb); err != nil {
			continue
		}
		if key == "VmRSS:" {
			rss = kb << 10
		} else {
			peak = kb << 10
		}
	}
	return rss, peak
}

// addObservability registers gocpp/stats and the command gocpp.stats.
func (s *Server) addObservability() {
	s.requests[GocppStats] = method{run: func(ctx context.Context, _ env, _ json.RawMessage) (any, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return s.stats(), nil
	}}
	s.requests["workspace/executeCommand"] = method{run: func(ctx context.Context, _ env, params json.RawMessage) (any, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var p protocol.ExecuteCommandParams
		if err := decode(params, &p); err != nil {
			return nil, err
		}
		if p.Command != StatsCommand {
			return nil, protocol.Errorf(protocol.InvalidParams, "unknown command %q", p.Command)
		}
		return s.stats(), nil
	}}
}

// observabilityCapabilities adds the command gocpp.stats to the server's
// capabilities.
func observabilityCapabilities(caps *protocol.ServerCapabilities) {
	caps.ExecuteCommandProvider = &protocol.ExecuteCommandOptions{Commands: []string{StatsCommand}}
}
