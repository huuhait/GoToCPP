package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/bench"
)

func TestPercentileIsTheNearestRank(t *testing.T) {
	values := []float64{9, 1, 5, 3, 7, 2, 8, 4, 6, 10}
	if got := percentile(values, 50); got != 5 {
		t.Errorf("p50 = %v, want 5", got)
	}
	if got := percentile(values, 95); got != 10 {
		t.Errorf("p95 = %v, want 10", got)
	}
	if got := percentile(values, 10); got != 1 {
		t.Errorf("p10 = %v, want 1", got)
	}
	if got := percentile(nil, 50); !math.IsNaN(got) {
		t.Errorf("p50 of nothing = %v, want NaN", got)
	}
}

func writeReport(t *testing.T, name string, samples ...[]float64) string {
	t.Helper()
	report := bench.Report{Results: []*bench.ProjectResult{{
		Project: "cpp", Target: "gocpp",
		Scenarios: []bench.ScenarioResult{{Name: "hover", Samples: samples}},
	}}}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCompareReportsPoolsTheFilesOfAConfiguration(t *testing.T) {
	base := writeReport(t, "a.json", []float64{1, 2, 3, 4})
	first := writeReport(t, "b1.json", []float64{2, 4})
	second := writeReport(t, "b2.json", []float64{6, 8})
	var out bytes.Buffer
	spec := "A=" + base + ";B=" + first + "," + second
	if err := compareReports(&out, spec); err != nil {
		t.Fatal(err)
	}
	// A: n=4, p50=2, p95=4. B pools four samples: p50=4, p95=8, ratios 2.00 and 2.00.
	want := "| cpp/hover | 4 | 2.00 | 4.00 | 4 | 4.00 | 8.00 | 2.00 | 2.00 |"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("table:\n%s\nwant the row\n%s", out.String(), want)
	}
}

func TestCompareReportsRejectsAGroupWithoutAName(t *testing.T) {
	if err := compareReports(&bytes.Buffer{}, "no-equals-sign"); err == nil {
		t.Fatal("a group without name=file was accepted")
	}
}
