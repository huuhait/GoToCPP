package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func ms(n ...int) []time.Duration {
	out := make([]time.Duration, len(n))
	for i, v := range n {
		out[i] = time.Duration(v) * time.Millisecond
	}
	return out
}

func TestMedianMAD(t *testing.T) {
	if got := median(ms(5, 1, 3)); got != 3*time.Millisecond {
		t.Errorf("median odd = %v", got)
	}
	if got := median(ms(4, 1, 3, 2)); got != 2500*time.Microsecond {
		t.Errorf("median even = %v", got)
	}
	if got := mad(ms(10, 10, 10, 50)); got != 0 {
		t.Errorf("mad with one outlier = %v, want 0", got)
	}
	if got := mad(ms(8, 10, 12)); got != 2*time.Millisecond {
		t.Errorf("mad = %v", got)
	}
	if median(nil) != 0 {
		t.Error("median of nothing")
	}
}

func TestReport(t *testing.T) {
	rows := []row{
		{"c", "macro", ms(120, 120, 120), ms(110, 110, 110), "1\n"},
		{"c", "hand", ms(100, 100, 100), ms(100, 100, 100), "1\n"},
		{"d", "bang", ms(50, 50, 50), ms(50, 50, 50), "2\n"},
		{"d", "hand", ms(50, 50, 50), ms(50, 50, 50), "3\n"},
	}
	var buf bytes.Buffer
	report(&buf, rows)
	got := buf.String()
	for _, want := range []string{"| c | macro | 110 | 120 | 0.0 | 1.10 |", "| c | hand | 100 | 100 | 0.0 | 1.00 |", "| d | bang | 50 | 50 | 0.0 | 1.00 |", "MISMATCH"} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(strings.SplitN(got, "| d | bang", 2)[0], "MISMATCH") {
		t.Errorf("case c is reported as a mismatch:\n%s", got)
	}
}
