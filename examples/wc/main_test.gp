package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCount(t *testing.T) {
	tests := []struct {
		in   string
		want Counts
	}{
		{"", Counts{}},
		{"hello", Counts{Words: 1, Runes: 5, Bytes: 5}},
		{"hello world\n", Counts{Lines: 1, Words: 2, Runes: 12, Bytes: 12}},
		{"  two\tspaces  \n\n end\n", Counts{Lines: 3, Words: 3, Runes: 21, Bytes: 21}},
		{"héllo wörld\n", Counts{Lines: 1, Words: 2, Runes: 12, Bytes: 14}},
		{"no newline\nat the end", Counts{Lines: 1, Words: 5, Runes: 21, Bytes: 21}},
	}
	for _, tt := range tests {
		got, err := Count(strings.NewReader(tt.in))
		if err != nil || got != tt.want {
			t.Errorf("Count(%q) = %+v, %v; want %+v", tt.in, got, err, tt.want)
		}
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		args  []string
		stdin string
		want  string
	}{
		{nil, "one two\nthree\n", "       2       3      14\n"},
		{[]string{"-w"}, "one two\nthree\n", "       3\n"},
		{[]string{"-l", "-c", "-", "-"}, "a b\n", "       1       4 -\n       0       0 -\n       1       4 total\n"},
		{[]string{"-m", "-c", "testdata/poem.txt"}, "", "     136     138 testdata/poem.txt\n"},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		code := run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
		if code != 0 || stdout.String() != tt.want || stderr.Len() != 0 {
			t.Errorf("run(%q) = %d, stdout %q, stderr %q; want 0, %q", tt.args, code, stdout.String(), stderr.String(), tt.want)
		}
	}
}

func TestRunMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"testdata/poem.txt", "testdata/missing.txt"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit status %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "missing.txt") {
		t.Errorf("stderr %q does not name the missing file", stderr.String())
	}
	if !strings.HasSuffix(stdout.String(), " total\n") {
		t.Errorf("stdout %q: want a total line for the files that could be read", stdout.String())
	}
}

func TestBadFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-x"}, strings.NewReader(""), &stdout, &stderr); code != 2 {
		t.Errorf("exit status %d, want 2", code)
	}
}
