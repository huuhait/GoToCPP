package diag

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestColorEnabled(t *testing.T) {
	env := func(kv ...string) func(string) string {
		m := map[string]string{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return func(k string) string { return m[k] }
	}
	var buf bytes.Buffer // not a terminal
	tests := []struct {
		name string
		mode string
		env  func(string) string
		want bool
	}{
		{"auto, not a terminal", "auto", env(), false},
		{"empty mode is auto", "", env(), false},
		{"flag always", "always", env("NO_COLOR", "1"), true},
		{"flag never", "never", env("GOCPP_COLOR", "always", "CLICOLOR_FORCE", "1"), false},
		{"GOCPP_COLOR always", "auto", env("GOCPP_COLOR", "always", "NO_COLOR", "1"), true},
		{"GOCPP_COLOR never", "auto", env("GOCPP_COLOR", "never", "CLICOLOR_FORCE", "1"), false},
		{"GOCPP_COLOR auto goes on", "auto", env("GOCPP_COLOR", "auto", "CLICOLOR_FORCE", "1"), true},
		{"GOCPP_COLOR bad goes on", "auto", env("GOCPP_COLOR", "sometimes"), false},
		{"NO_COLOR", "auto", env("NO_COLOR", "1", "CLICOLOR_FORCE", "1"), false},
		{"NO_COLOR true", "auto", env("NO_COLOR", "true", "CLICOLOR_FORCE", "1"), false},
		{"NO_COLOR 0 is still set", "auto", env("NO_COLOR", "0", "CLICOLOR_FORCE", "1"), false},
		{"NO_COLOR empty is unset", "auto", env("NO_COLOR", "", "CLICOLOR_FORCE", "1"), true},
		{"CLICOLOR_FORCE", "auto", env("CLICOLOR_FORCE", "1"), true},
		{"CLICOLOR_FORCE 0", "auto", env("CLICOLOR_FORCE", "0"), false},
		{"TERM dumb", "auto", env("TERM", "dumb", "CLICOLOR_FORCE", "1"), true}, // force comes first
	}
	for _, tt := range tests {
		if got := ColorEnabled(tt.mode, &buf, tt.env); got != tt.want {
			t.Errorf("%s: ColorEnabled(%q) = %v, want %v", tt.name, tt.mode, got, tt.want)
		}
	}
	// A file is no terminal, the null device is not one either.
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if IsTerminal(f) {
		t.Error("IsTerminal(regular file) = true")
	}
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err == nil {
		defer null.Close()
		if IsTerminal(null) {
			t.Error("IsTerminal(os.DevNull) = true")
		}
	}
}

func TestParseColorMode(t *testing.T) {
	for in, want := range map[string]string{"": "auto", "auto": "auto", "always": "always", "never": "never"} {
		if got, ok := ParseColorMode(in); !ok || got != want {
			t.Errorf("ParseColorMode(%q) = %q, %v", in, got, ok)
		}
	}
	if _, ok := ParseColorMode("yes"); ok {
		t.Error("ParseColorMode(yes) is ok")
	}
}

func TestPaint(t *testing.T) {
	s := &printState{Printer: &Printer{Color: true}}
	if got := s.paint(styleNote, "a\nb\n\nc"); got != styleNote+"a"+styleReset+"\n"+styleNote+"b"+styleReset+"\n\n"+styleNote+"c"+styleReset {
		t.Errorf("paint of lines = %q", got)
	}
	if got := s.paint(styleNote, ""); got != "" {
		t.Errorf("paint of nothing = %q", got)
	}
	s.Color = false
	if got := s.paint(styleNote, "a"); got != "a" || strings.Contains(got, "\x1b") {
		t.Errorf("paint without color = %q", got)
	}
}
