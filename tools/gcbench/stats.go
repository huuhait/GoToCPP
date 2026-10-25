package main

import (
	"math"
	"sort"
)

// stat summarizes repeated measurements of one quantity. Median and MAD are
// in the unit of the values (nanoseconds for wall and CPU time, KiB for the
// resident set); MAD is the median absolute deviation, not scaled.
type stat struct {
	N      int     `json:"n"`
	Median float64 `json:"median"`
	MAD    float64 `json:"mad"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

// summarize computes the stat of xs. An empty xs gives a zero stat.
func summarize(xs []int64) stat {
	if len(xs) == 0 {
		return stat{}
	}
	s := append([]int64(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	st := stat{N: len(s), Median: medianSorted(s), Min: float64(s[0]), Max: float64(s[len(s)-1])}
	d := make([]float64, len(s))
	for i, x := range s {
		d[i] = math.Abs(float64(x) - st.Median)
	}
	sort.Float64s(d)
	st.MAD = medianFloat(d)
	return st
}

// medianSorted returns the median of an ascending slice.
func medianSorted(s []int64) float64 {
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return float64(s[n/2])
	}
	return (float64(s[n/2-1]) + float64(s[n/2])) / 2
}

// medianFloat returns the median of a slice it may sort.
func medianFloat(xs []float64) float64 {
	n := len(xs)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return xs[n/2]
	}
	return (xs[n/2-1] + xs[n/2]) / 2
}

// relNoise returns MAD / median: the spread as a share of the value (0 when
// there is no median).
func (s stat) relNoise() float64 {
	if s.Median <= 0 {
		return 0
	}
	return s.MAD / s.Median
}
