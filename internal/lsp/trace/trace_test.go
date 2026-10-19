package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestBreakdown(t *testing.T) {
	ctx, root := Begin(context.Background(), "request")
	cctx, check := Start(ctx, "check")
	_, load := Start(cctx, "load")
	time.Sleep(2 * time.Millisecond)
	load.End()
	_, tc := Start(cctx, "typecheck")
	time.Sleep(5 * time.Millisecond)
	tc.End()
	check.End()
	for range 3 { // the spans of one name add up
		_, p := Start(ctx, "parse")
		time.Sleep(time.Millisecond)
		p.End()
	}
	sum := root.End()
	if sum.Name != "request" || sum.Total < 7*time.Millisecond {
		t.Fatalf("summary %+v", sum)
	}
	got := map[string]Phase{}
	for _, p := range sum.Phases {
		got[p.Name] = p
	}
	if len(got) != 4 || got["parse"].Count != 3 || got["load"].Count != 1 {
		t.Errorf("phases %+v", sum.Phases)
	}
	if got["check"].Total < got["typecheck"].Total+got["load"].Total {
		t.Errorf("check %v holds typecheck %v and load %v", got["check"].Total, got["typecheck"].Total, got["load"].Total)
	}
	if got["parse"].Total < 3*time.Millisecond {
		t.Errorf("three parse spans of 1 ms add up to %v", got["parse"].Total)
	}
	// Longest first.
	for i := 1; i < len(sum.Phases); i++ {
		if sum.Phases[i].Total > sum.Phases[i-1].Total {
			t.Errorf("phases not sorted: %+v", sum.Phases)
		}
	}
}

func TestNoRoot(t *testing.T) {
	ctx, s := Start(context.Background(), "x")
	if s != nil {
		t.Fatal("a span outside a root span")
	}
	s.Set("a", 1)
	if sum := s.End(); sum.Name != "" || ctx == nil {
		t.Errorf("summary %+v", sum)
	}
}

func TestEndTwice(t *testing.T) {
	ctx, root := Begin(context.Background(), "r")
	_, s := Start(ctx, "s")
	s.End()
	s.End()
	sum := root.End()
	if len(sum.Phases) != 1 || sum.Phases[0].Count != 1 {
		t.Errorf("phases %+v", sum.Phases)
	}
	if again := root.End(); again.Name != "" {
		t.Errorf("a second End gave %+v", again)
	}
}

func TestChrome(t *testing.T) {
	rec := NewRecorder(0)
	ctx := WithRecorder(context.Background(), rec)
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cctx, root := Begin(ctx, "request", "n", i)
			_, s := Start(cctx, "check", "dir", "/x")
			s.End()
			root.End()
		}()
	}
	wg.Wait()
	var buf bytes.Buffer
	if err := rec.WriteChrome(&buf); err != nil {
		t.Fatal(err)
	}
	var out struct {
		TraceEvents []struct {
			Name string         `json:"name"`
			Ph   string         `json:"ph"`
			Ts   float64        `json:"ts"`
			Dur  float64        `json:"dur"`
			Tid  int64          `json:"tid"`
			Args map[string]any `json:"args"`
		} `json:"traceEvents"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("%v\n%s", err, buf.String())
	}
	count := map[string]int{}
	tids := map[int64]bool{}
	for _, e := range out.TraceEvents {
		count[e.Ph+":"+e.Name]++
		if e.Ph == "X" {
			tids[e.Tid] = true
			if e.Dur <= 0 || e.Ts < 0 {
				t.Errorf("event %+v", e)
			}
		}
	}
	if count["X:request"] != 3 || count["X:check"] != 3 || count["M:thread_name"] != 3 || len(tids) != 3 {
		t.Errorf("events %v, rows %v", count, tids)
	}
	kept, dropped := rec.Len()
	if kept != 6 || dropped != 0 {
		t.Errorf("kept %d, dropped %d", kept, dropped)
	}
}

func TestRecorderMax(t *testing.T) {
	rec := NewRecorder(2)
	ctx := WithRecorder(context.Background(), rec)
	for range 5 {
		_, root := Begin(ctx, "r")
		root.End()
	}
	kept, dropped := rec.Len()
	if kept != 2 || dropped != 3 {
		t.Errorf("kept %d, dropped %d", kept, dropped)
	}
}
