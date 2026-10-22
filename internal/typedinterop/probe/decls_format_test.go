package probe

import (
	"context"
	"testing"
)

const formatHeader = `#pragma once
#include <cstdarg>
int say(const char* format, ...) __attribute__((format(printf, 1, 2)));
int say_to(int level, const char* format, ...) __attribute__((format(printf, 2, 3)));
int scan_from(const char* text, const char* format, ...) __attribute__((format(scanf, 2, 3)));
int plain(const char* text, ...);
int fixed(const char* text);
namespace logging { void note(const char* format, ...) __attribute__((format(printf, 1, 2))); }
`

// TestFormatKind checks that the probe finds the parameter that clang knows as a format string
// (the attribute is asked for with a call that has a deliberately incomplete format), and its kind.
func TestFormatKind(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "fmt.hpp", formatHeader)
	cfg := clangConfig(t, dir)
	dc := DeclConfig{Includes: []string{`"./fmt.hpp"`}}
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{
		{DeclFunc, "say"}, {DeclFunc, "say_to"}, {DeclFunc, "scan_from"}, {DeclFunc, "plain"}, {DeclFunc, "fixed"},
		{DeclFunc, "logging::note"},
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		kind  string
		param int
	}{
		{"say", "printf", 1},
		{"say_to", "printf", 2},
		{"scan_from", "scanf", 2},
		{"plain", "", 0},
		{"fixed", "", 0},
		{"logging::note", "printf", 1},
	}
	for _, tt := range tests {
		fs := tab.Funcs[tt.name]
		if fs == nil || len(fs.Overloads) != 1 {
			t.Errorf("%s: no overload set: %+v", tt.name, fs)
			continue
		}
		ov := fs.Overloads[0]
		if ov.FormatKind != tt.kind || ov.FormatParam != tt.param {
			t.Errorf("%s: format %q at %d, want %q at %d", tt.name, ov.FormatKind, ov.FormatParam, tt.kind, tt.param)
		}
	}
}
