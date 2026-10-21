package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/lsp/bench"
)

// A column is the pooled samples of one scenario in the reports of one configuration.
type column struct {
	name    string
	samples map[string][]float64 // "project/scenario" -> milliseconds
	fails   map[string]int
}

// readColumns reads "name=file,file" groups: the samples of several report files of one
// configuration (one per round) are pooled.
func readColumns(spec string) ([]*column, error) {
	var columns []*column
	for _, group := range strings.Split(spec, ";") {
		name, files, ok := strings.Cut(group, "=")
		if !ok {
			return nil, fmt.Errorf("%q is not name=file[,file]", group)
		}
		col := &column{name: name, samples: make(map[string][]float64), fails: make(map[string]int)}
		for _, file := range strings.Split(files, ",") {
			f, err := os.Open(file)
			if err != nil {
				return nil, err
			}
			rep, err := bench.ReadJSON(f)
			f.Close()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", file, err)
			}
			for _, res := range rep.Results {
				for _, sc := range res.Scenarios {
					key := res.Project + "/" + sc.Name
					for _, run := range sc.Samples {
						col.samples[key] = append(col.samples[key], run...)
					}
					col.fails[key] += sc.Fail
				}
			}
		}
		columns = append(columns, col)
	}
	return columns, nil
}

// percentile is the nearest-rank percentile of the values (the slice is sorted in place).
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	slices.Sort(values)
	rank := int(math.Ceil(p / 100 * float64(len(values))))
	return values[max(rank, 1)-1]
}

// compareReports writes a table of n, p50 and p95 per scenario for each configuration and the
// ratios of the later configurations to the first.
func compareReports(out io.Writer, spec string) error {
	columns, err := readColumns(spec)
	if err != nil {
		return err
	}
	var keys []string
	for _, col := range columns {
		for key := range col.samples {
			if !slices.Contains(keys, key) {
				keys = append(keys, key)
			}
		}
	}
	slices.Sort(keys)
	fmt.Fprint(out, "| row |")
	for _, col := range columns {
		fmt.Fprintf(out, " %s n | p50 ms | p95 ms |", col.name)
		if col != columns[0] {
			fmt.Fprint(out, " p50 ratio | p95 ratio |")
		}
	}
	fmt.Fprintln(out)
	fmt.Fprint(out, "|---|")
	for col := range columns {
		fmt.Fprint(out, "---:|---:|---:|")
		if col > 0 {
			fmt.Fprint(out, "---:|---:|")
		}
	}
	fmt.Fprintln(out)
	for _, key := range keys {
		fmt.Fprintf(out, "| %s |", key)
		var base50, base95 float64
		for i, col := range columns {
			values := slices.Clone(col.samples[key])
			p50, p95 := percentile(values, 50), percentile(values, 95)
			fmt.Fprintf(out, " %d | %.2f | %.2f |", len(values), p50, p95)
			if i == 0 {
				base50, base95 = p50, p95
			} else {
				fmt.Fprintf(out, " %.2f | %.2f |", p50/base50, p95/base95)
			}
		}
		fmt.Fprintln(out)
	}
	return nil
}
