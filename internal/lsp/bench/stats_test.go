package bench

import (
	"math"
	"testing"
	"time"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestPercentile(t *testing.T) {
	ten := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	hundred := make([]float64, 100)
	for i := range hundred {
		hundred[i] = float64(i + 1)
	}
	for _, c := range []struct {
		xs   []float64
		p    float64
		want float64
	}{
		{nil, 50, 0},
		{[]float64{7}, 50, 7},
		{[]float64{7}, 95, 7},
		{[]float64{7}, 0, 7},
		{ten, 50, 5}, // nearest rank: ceil(0.5*10) = 5th
		{ten, 95, 10},
		{ten, 100, 10},
		{ten, 10, 1},
		{ten, 11, 2},
		{hundred, 50, 50},
		{hundred, 95, 95},
		{hundred, 99, 99},
		{hundred, 100, 100},
		{[]float64{1, 2}, 50, 1},
		{[]float64{1, 2}, 51, 2},
	} {
		if got := Percentile(c.xs, c.p); !near(got, c.want) {
			t.Errorf("Percentile(%v, %v) = %v, want %v", c.xs, c.p, got, c.want)
		}
	}
}

func TestMedian(t *testing.T) {
	for _, c := range []struct {
		xs   []float64
		want float64
	}{
		{nil, 0}, {[]float64{4}, 4}, {[]float64{3, 1, 2}, 2}, {[]float64{4, 1, 3, 2}, 2.5}, {[]float64{5, 5, 100, 5, 5}, 5},
	} {
		in := append([]float64(nil), c.xs...)
		if got := Median(c.xs); !near(got, c.want) {
			t.Errorf("Median(%v) = %v, want %v", in, got, c.want)
		}
		for i := range in {
			if in[i] != c.xs[i] {
				t.Errorf("Median changed its argument")
			}
		}
	}
}

func TestSummarize(t *testing.T) {
	s := Summarize([]float64{9, 1, 5, 3, 7})
	if s.N != 5 || s.First != 9 || s.Min != 1 || s.Max != 9 || s.P50 != 5 || s.P95 != 9 || !near(s.Mean, 5) {
		t.Errorf("%+v", s)
	}
	if (Summarize(nil) != Summary{}) {
		t.Errorf("no samples: %+v", Summarize(nil))
	}
}

func TestAggregates(t *testing.T) {
	runs := []Summary{
		{N: 10, First: 50, P50: 5, P95: 9, Max: 10},
		{N: 10, First: 60, P50: 6, P95: 10, Max: 11},
		{N: 10, First: 400, P50: 40, P95: 90, Max: 500}, // a run on a busy machine
		{}, // a run with no samples is left out
		{N: 10, First: 55, P50: 5.5, P95: 9.5, Max: 10.5},
		{N: 10, First: 52, P50: 5.2, P95: 9.2, Max: 10.2},
	}
	a := Aggregates(runs)
	if a.Runs != 5 || a.N != 10 || !near(a.P50, 5.5) || !near(a.P95, 9.5) || !near(a.Max, 10.5) || !near(a.First, 55) || a.Worst != 500 {
		t.Errorf("%+v", a)
	}
	if (Aggregates(nil) != Aggregate{}) {
		t.Errorf("no runs: %+v", Aggregates(nil))
	}
}

func TestMillis(t *testing.T) {
	if got := Millis(1500 * time.Microsecond); !near(got, 1.5) {
		t.Errorf("%v", got)
	}
}
