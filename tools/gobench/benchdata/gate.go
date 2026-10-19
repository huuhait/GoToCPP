package benchdata

import (
	"fmt"
	"math"
)

// Verdict is the outcome of comparing one measurement with its baseline.
type Verdict int

const (
	// OK: within the threshold (or too small to tell from noise in the good direction).
	OK Verdict = iota
	// Improved: faster than the baseline by more than the threshold.
	Improved
	// Regression: slower than the baseline by more than the threshold, beyond noise.
	Regression
	// Inconclusive: the figures cannot tell (too few runs, or the median is over the
	// threshold but the fastest run is not: a busy machine).
	Inconclusive
)

func (v Verdict) String() string {
	return [...]string{"ok", "improved", "REGRESSION", "inconclusive"}[v]
}

// GateConfig is the parameters of a comparison.
type GateConfig struct {
	Threshold float64 // allowed slowdown as a fraction, 0.05 for 5 %
	Drift     float64 // the machine's speed now over the baseline's (1: no correction)
	MinRuns   int     // fewer runs than this in either set is inconclusive (default 3)
}

// Decision is the verdict for one measurement with the numbers behind it.
type Decision struct {
	Verdict  Verdict
	Ratio    float64 // median now (drift-corrected) over median of the baseline
	MinRatio float64 // fastest run now (drift-corrected) over fastest run of the baseline
	Noise    float64 // the larger relative MAD of the two sets
	Reason   string
}

// Decide compares the measurements cur with the baseline base.
//
// A slowdown is a regression only when three things hold: the median is over
// the threshold, the FASTEST run is over the threshold too (a few slow runs
// from a busy machine move the median, not the minimum, so the whole
// distribution really moved), and the difference of the medians is more than
// twice the larger MAD. When the median is over but the minimum is not, the
// answer is inconclusive, never a failure. Both sets are first corrected by
// the machine drift (cfg.Drift), which tools/gobench measures with the
// implementations that gocpp does not change (Go and the C++ twins).
func Decide(base, cur Stat, cfg GateConfig) Decision {
	minRuns := cfg.MinRuns
	if minRuns == 0 {
		minRuns = 3
	}
	d := cfg.Drift
	if d <= 0 {
		d = 1
	}
	if base.N < minRuns || cur.N < minRuns || base.Median <= 0 || cur.Median <= 0 {
		return Decision{Verdict: Inconclusive, Reason: fmt.Sprintf("too few runs (%d baseline, %d now)", base.N, cur.N)}
	}
	curMed, curMin := cur.Median/d, cur.Min/d
	dec := Decision{
		Ratio:    curMed / base.Median,
		MinRatio: curMin / base.Min,
		Noise:    math.Max(cur.RelNoise(), base.RelNoise()),
	}
	t := cfg.Threshold
	switch {
	case dec.Ratio > 1+t:
		if dec.MinRatio <= 1+t {
			dec.Verdict = Inconclusive
			dec.Reason = fmt.Sprintf("median %+.1f%% but the fastest run only %+.1f%%: noise", (dec.Ratio-1)*100, (dec.MinRatio-1)*100)
		} else if curMed-base.Median <= 2*math.Max(cur.MAD/d, base.MAD) {
			dec.Verdict = Inconclusive
			dec.Reason = fmt.Sprintf("%+.1f%% is within twice the MAD", (dec.Ratio-1)*100)
		} else {
			dec.Verdict = Regression
			dec.Reason = fmt.Sprintf("median %+.1f%%, fastest run %+.1f%%", (dec.Ratio-1)*100, (dec.MinRatio-1)*100)
		}
	case dec.Ratio < 1-t && dec.MinRatio < 1-t:
		dec.Verdict = Improved
		dec.Reason = fmt.Sprintf("median %+.1f%%", (dec.Ratio-1)*100)
	default:
		dec.Verdict = OK
	}
	return dec
}

// Confirm combines the decisions of repeated measurements of the same
// benchmark (the first one and the re-runs): a regression must reproduce in
// every one of them. One run that is ok or improved clears it; a mix of
// regressions and inconclusive runs stays inconclusive.
func Confirm(ds []Decision) Verdict {
	if len(ds) == 0 {
		return Inconclusive
	}
	reg := 0
	for _, d := range ds {
		switch d.Verdict {
		case OK, Improved:
			return OK
		case Regression:
			reg++
		}
	}
	if reg == len(ds) {
		return Regression
	}
	return Inconclusive
}

// LoadOK reports whether the 1-minute load average is low enough to measure:
// at most maxLoad (0: no check).
func LoadOK(load1, maxLoad float64) bool { return maxLoad <= 0 || load1 <= maxLoad }

// DefaultMaxLoad returns the 1-minute load average above which the gate and
// the runner call the machine busy: an eighth of the cores, and at least 4
// (the language server's gate uses 16 on this 128-core machine).
func DefaultMaxLoad(cores int) float64 { return math.Max(4, float64(cores)/8) }

// Drift returns the speed of this machine relative to the baseline's: the
// geometric mean over the benchmarks of (reference time now / reference time
// in the baseline), where the reference is a measurement that gocpp does not
// influence (the Go program, and the C++ twins). It returns 1 and 0 when no
// reference pair exists, and the number of pairs it used.
func Drift(base, cur *Report) (float64, int) {
	var ratios []float64
	for _, r := range cur.Results {
		if IsGocpp(r.Impl) || r.Status != StatusOK {
			continue
		}
		b := base.Find(r.Program, r.Impl)
		if b == nil || b.Status != StatusOK {
			continue
		}
		bs, cs := b.WallStat(), r.WallStat()
		if bs.N == 0 || cs.N == 0 {
			continue
		}
		ratios = append(ratios, Ratio(cs.Median, bs.Median))
	}
	g, n := GeoMean(ratios)
	if n == 0 {
		return 1, 0
	}
	return g, n
}

// Row is one benchmark's comparison.
type Row struct {
	Program  string
	Impl     string
	Decision Decision
}

// Comparison is the result of comparing a report with a baseline.
type Comparison struct {
	Drift      float64
	DriftPairs int
	Rows       []Row
	Missing    []string // in the baseline, not measured now (or not ok now)
}

// Compare compares the gocpp implementations of cur with base, benchmark by
// benchmark. impls limits the implementations (nil: every gocpp one).
func Compare(base, cur *Report, cfg GateConfig, useDrift bool, impls []string) Comparison {
	var c Comparison
	c.Drift = 1
	if useDrift {
		c.Drift, c.DriftPairs = Drift(base, cur)
	}
	cfg.Drift = c.Drift
	want := func(impl string) bool {
		if impls == nil {
			return IsGocpp(impl)
		}
		for _, x := range impls {
			if x == impl {
				return true
			}
		}
		return false
	}
	for _, b := range base.Results {
		if !want(b.Impl) || b.Status != StatusOK {
			continue
		}
		r := cur.Find(b.Program, b.Impl)
		if r == nil || r.Status != StatusOK {
			c.Missing = append(c.Missing, b.Program+"/"+b.Impl)
			continue
		}
		c.Rows = append(c.Rows, Row{b.Program, b.Impl, Decide(b.WallStat(), r.WallStat(), cfg)})
	}
	return c
}

// Count returns how many rows have verdict v.
func (c Comparison) Count(v Verdict) int {
	n := 0
	for _, r := range c.Rows {
		if r.Decision.Verdict == v {
			n++
		}
	}
	return n
}
