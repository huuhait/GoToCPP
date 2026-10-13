package version

import (
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	if Version == "" || strings.ContainsAny(Version, " \t\n") {
		t.Errorf("Version = %q, want one non-empty word", Version)
	}
}

func TestPhase(t *testing.T) {
	// The plan has phases 00 to 15.
	if Phase < 0 || Phase > 15 {
		t.Errorf("Phase = %d, want 0 to 15", Phase)
	}
}

func TestString(t *testing.T) {
	got := String()
	// A development Go toolchain has spaces in its version, so (.+) for it.
	re := regexp.MustCompile(`^gocpp (\S+) \(phase (\d\d)\) (.+) (\S+)/(\S+)$`)
	m := re.FindStringSubmatch(got)
	if m == nil {
		t.Fatalf("String() = %q, want the form \"gocpp dev (phase 00) go1.25.7 linux/amd64\"", got)
	}
	if m[1] != Version {
		t.Errorf("String() = %q: version %q, want %q", got, m[1], Version)
	}
	if n, _ := strconv.Atoi(m[2]); n != Phase {
		t.Errorf("String() = %q: phase %s, want %02d", got, m[2], Phase)
	}
	if m[3] != runtime.Version() {
		t.Errorf("String() = %q: Go version %q, want %q", got, m[3], runtime.Version())
	}
	if m[4] != runtime.GOOS || m[5] != runtime.GOARCH {
		t.Errorf("String() = %q: platform %s/%s, want %s/%s", got, m[4], m[5], runtime.GOOS, runtime.GOARCH)
	}
}
