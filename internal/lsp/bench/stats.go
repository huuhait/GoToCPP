package bench

import (
	"math"
	"slices"
	"time"
)

// Percentile returns the p-th percentile (0 to 100) of sorted, by the
// nearest-rank rule: the smallest value that at least p percent of the
// values are less than or equal to. With fewer than 20 values the 95th
// percentile is the maximum. It is 0 for no values.
func Percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := int(math.Ceil(p / 100 * float64(n)))
	return sorted[min(max(rank, 1), n)-1]
}

// Median returns the middle value of xs (the mean of the two middle ones
// for an even number); it does not change xs. It is 0 for no values.
func Median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	if n := len(s); n%2 == 0 {
		return (s[n/2-1] + s[n/2]) / 2
	}
	return s[len(s)/2]
}

// Millis returns d in milliseconds.
func Millis(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

// A Summary describes the samples of one run of a scenario, in
// milliseconds.
type Summary struct {
	N             int
	First         float64 // the first sample (the cold one: nothing was asked before)
	P50, P95, Max float64
	Min, Mean     float64
}

// Summarize describes the samples (milliseconds, in the order they were
// taken). The zero Summary is for no samples.
func Summarize(samples []float64) Summary {
	if len(samples) == 0 {
		return Summary{}
	}
	s := slices.Clone(samples)
	slices.Sort(s)
	sum := 0.0
	for _, x := range s {
		sum += x
	}
	return Summary{N: len(s), First: samples[0], P50: Percentile(s, 50), P95: Percentile(s, 95),
		Max: s[len(s)-1], Min: s[0], Mean: sum / float64(len(s))}
}

// An Aggregate is a scenario over several runs: the median over the runs
// of each statistic of a run (the noise of a shared machine moves single
// runs a lot; the median of five does not move much).
type Aggregate struct {
	Runs          int     // the runs with samples
	N             int     // the samples per run (of the first run)
	First         float64 // median of the first samples
	P50, P95, Max float64 // medians of the runs' p50, p95 and max
	Worst         float64 // the largest sample of any run
}

// Aggregates combines the summaries of the runs.
func Aggregates(runs []Summary) Aggregate {
	var a Aggregate
	var first, p50, p95, mx []float64
	for _, r := range runs {
		if r.N == 0 {
			continue
		}
		a.Runs++
		if a.N == 0 {
			a.N = r.N
		}
		first = append(first, r.First)
		p50 = append(p50, r.P50)
		p95 = append(p95, r.P95)
		mx = append(mx, r.Max)
		a.Worst = max(a.Worst, r.Max)
	}
	a.First, a.P50, a.P95, a.Max = Median(first), Median(p50), Median(p95), Median(mx)
	return a
}
