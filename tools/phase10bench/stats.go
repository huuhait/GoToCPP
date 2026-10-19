package main

import (
	"fmt"
	"io"
	"slices"
	"time"
)

// median returns the median of ds (the mean of the two middle values for an even count).
func median(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := slices.Clone(ds)
	slices.Sort(s)
	if n := len(s); n%2 == 0 {
		return (s[n/2-1] + s[n/2]) / 2
	}
	return s[len(s)/2]
}

// mad returns the median absolute deviation of ds around its median.
func mad(ds []time.Duration) time.Duration {
	m := median(ds)
	dev := make([]time.Duration, len(ds))
	for i, d := range ds {
		dev[i] = max(d-m, m-d)
	}
	return median(dev)
}

// A row is the result of one variant of one case.
type row struct {
	caseName, variant string
	times, cpus       []time.Duration
	output            string
}

func all2rows[T any](all []T, times, cpus [][]time.Duration, outputs []string, name func(T) (string, string)) []row {
	rows := make([]row, len(all))
	for i, b := range all {
		c, v := name(b)
		rows[i] = row{c, v, times[i], cpus[i], outputs[i]}
	}
	return rows
}

// report writes the table: for each case the median CPU time and wall time of each variant, the MAD of the CPU time
// in percent, and the CPU time over the CPU time of the variant "hand". It also checks that the variants of a case print the same.
func report(w io.Writer, rows []row) {
	fmt.Fprintln(w, "| case | variant | CPU ms | wall ms | MAD % | CPU over hand |")
	fmt.Fprintln(w, "|---|---|---:|---:|---:|---:|")
	for i := 0; i < len(rows); {
		j := i
		for j < len(rows) && rows[j].caseName == rows[i].caseName {
			j++
		}
		group := rows[i:j]
		var hand time.Duration
		for _, r := range group {
			if r.variant == "hand" {
				hand = median(r.cpus)
			}
		}
		for _, r := range group {
			m := median(r.cpus)
			ratio := "-"
			if hand > 0 {
				ratio = fmt.Sprintf("%.2f", float64(m)/float64(hand))
			}
			pct := 0.0
			if m > 0 {
				pct = 100 * float64(mad(r.cpus)) / float64(m)
			}
			fmt.Fprintf(w, "| %s | %s | %.0f | %.0f | %.1f | %s |\n", r.caseName, r.variant, float64(m)/float64(time.Millisecond), float64(median(r.times))/float64(time.Millisecond), pct, ratio)
			if r.output != group[0].output {
				fmt.Fprintf(w, "| %s | %s | MISMATCH: prints %q, not %q | | | |\n", r.caseName, r.variant, r.output, group[0].output)
			}
		}
		i = j
	}
}
