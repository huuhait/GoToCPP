// Package benchdata holds what tools/gobench and tools/benchhist share: the
// result types (the JSON schema of bench/baseline.json, of a run's results
// and of the history lines), the statistics over repeated runs, and the
// regression-gate decision. It has no I/O except reading and writing the
// JSON files, so everything in it is tested with plain numbers.
package benchdata

import (
	"math"
	"sort"
)

// Median returns the median of xs (the mean of the two middle values for an
// even count); 0 for no values. xs is not changed.
func Median(xs []int64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int64(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	n := len(s)
	if n%2 == 1 {
		return float64(s[n/2])
	}
	return (float64(s[n/2-1]) + float64(s[n/2])) / 2
}

// MAD returns the median absolute deviation of xs from their median: the
// robust measure of noise (one slow run, from a busy machine, does not move
// it, unlike the standard deviation). It is not scaled to a standard
// deviation. 0 for fewer than two values.
func MAD(xs []int64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := Median(xs)
	d := make([]float64, len(xs))
	for i, x := range xs {
		d[i] = math.Abs(float64(x) - m)
	}
	sort.Float64s(d)
	n := len(d)
	if n%2 == 1 {
		return d[n/2]
	}
	return (d[n/2-1] + d[n/2]) / 2
}

// Min returns the smallest of xs; 0 for no values.
func Min(xs []int64) int64 {
	if len(xs) == 0 {
		return 0
	}
	m := xs[0]
	for _, x := range xs[1:] {
		m = min(m, x)
	}
	return m
}

// Stat summarizes the repeated measurements of one quantity.
type Stat struct {
	N      int     // number of values
	Median float64 // the median
	MAD    float64 // the median absolute deviation
	Min    float64 // the smallest value
}

// Summarize computes the Stat of xs.
func Summarize(xs []int64) Stat {
	return Stat{N: len(xs), Median: Median(xs), MAD: MAD(xs), Min: float64(Min(xs))}
}

// RelNoise returns MAD / median: the noise of a measurement as a fraction of
// its value (0 when there is no median).
func (s Stat) RelNoise() float64 {
	if s.Median <= 0 {
		return 0
	}
	return s.MAD / s.Median
}

// GeoMean returns the geometric mean of the positive values of xs, and how
// many values it used (values that are not positive, or not finite, are
// left out). With no usable value it returns 0, 0.
func GeoMean(xs []float64) (float64, int) {
	sum, n := 0.0, 0
	for _, x := range xs {
		if x > 0 && !math.IsInf(x, 0) && !math.IsNaN(x) {
			sum += math.Log(x)
			n++
		}
	}
	if n == 0 {
		return 0, 0
	}
	return math.Exp(sum / float64(n)), n
}

// Ratio returns a / b, or 0 when either is not positive: "no ratio".
func Ratio(a, b float64) float64 {
	if a <= 0 || b <= 0 {
		return 0
	}
	return a / b
}
