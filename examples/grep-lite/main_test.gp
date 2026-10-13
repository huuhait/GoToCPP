package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewMatcher(t *testing.T) {
	tests := []struct {
		pattern string
		opt     options
		line    string
		want    bool
	}{
		{"go", options{}, "gocpp", true},
		{"^go$", options{}, "gocpp", false},
		{"GO", options{}, "gocpp", false},
		{"GO", options{ignoreCase: true}, "gocpp", true},
		{"a.c", options{}, "abc", true},
		{"a.c", options{fixed: true}, "abc", false},
		{"a.c", options{fixed: true}, "xa.cx", true},
		{"A.C", options{fixed: true, ignoreCase: true}, "xa.cx", true},
	}
	for _, tt := range tests {
		m, err := newMatcher(tt.pattern, tt.opt)
		if err != nil {
			t.Fatalf("newMatcher(%q, %+v): %v", tt.pattern, tt.opt, err)
		}
		if got := m(tt.line); got != tt.want {
			t.Errorf("pattern %q, options %+v, line %q: got %v, want %v", tt.pattern, tt.opt, tt.line, got, tt.want)
		}
	}
	if _, err := newMatcher("(", options{}); err == nil {
		t.Error(`newMatcher("(") gave no error`)
	}
}

func TestGrep(t *testing.T) {
	const in = "alpha\nbeta\ngamma\ndelta\n"
	tests := []struct {
		opt  options
		want string
		n    int
	}{
		{options{}, "x:alpha\nx:beta\nx:gamma\nx:delta\n", 4},
		{options{lineNumbers: true}, "x:2:beta\nx:3:gamma\n", 2},
		{options{invert: true}, "x:alpha\nx:delta\n", 2},
		{options{count: true}, "x:2\n", 2},
	}
	for _, tt := range tests {
		pattern := "a"
		if tt.opt != (options{}) {
			pattern = "^(beta|gamma)$"
		}
		m, err := newMatcher(pattern, tt.opt)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		n, err := grep(strings.NewReader(in), "x:", m, tt.opt, &out)
		if err != nil || n != tt.n || out.String() != tt.want {
			t.Errorf("options %+v: got %d, %v, %q; want %d, %q", tt.opt, n, err, out.String(), tt.n, tt.want)
		}
	}
}

func TestRunExitStatus(t *testing.T) {
	tests := []struct {
		args []string
		want int
	}{
		{[]string{"ERROR", "testdata/app.log"}, 0},
		{[]string{"no such text", "testdata/app.log"}, 1},
		{[]string{"ERROR", "testdata/missing.log"}, 2},
		{[]string{"(", "testdata/app.log"}, 2},
		{nil, 2},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		if got := run(tt.args, strings.NewReader(""), &stdout, &stderr); got != tt.want {
			t.Errorf("run(%q) = %d, want %d (stderr %q)", tt.args, got, tt.want, stderr.String())
		}
	}
}

func TestRunStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-c", "-i", "go"}, strings.NewReader("Go\ngocpp\nC++\n"), &stdout, &stderr)
	if code != 0 || stdout.String() != "2\n" {
		t.Errorf("got %d, %q; want 0, %q", code, stdout.String(), "2\n")
	}
}
