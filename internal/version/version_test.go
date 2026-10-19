package version

import (
	"regexp"
	"runtime"
	"runtime/debug"
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
		t.Fatalf("String() = %q, want the form \"gocpp dev (phase 00) go1.25.14 linux/amd64\"", got)
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

func TestFromBuildInfo(t *testing.T) {
	const mod = "github.com/huuhait/gocpp"
	vcs := []debug.BuildSetting{{Key: "vcs", Value: "git"}, {Key: "vcs.revision", Value: "0123456789ab"}}
	for _, c := range []struct {
		name string
		bi   *debug.BuildInfo
		want string
	}{
		{"nil", nil, "dev"},
		{"go install @v1.2.3", &debug.BuildInfo{Main: debug.Module{Path: mod, Version: "v1.2.3"}}, "v1.2.3"},
		{"go install @latest, no tag", &debug.BuildInfo{Main: debug.Module{Path: mod, Version: "v0.0.0-20261019101700-0123456789ab"}}, "v0.0.0-20261019101700-0123456789ab"},
		{"go build in a checkout", &debug.BuildInfo{Main: debug.Module{Path: mod, Version: "(devel)"}}, "dev"},
		{"go build in a git checkout", &debug.BuildInfo{Main: debug.Module{Path: mod, Version: "v1.2.4-0.20261019101700-0123456789ab+dirty"}, Settings: vcs}, "dev"},
		{"empty", &debug.BuildInfo{Main: debug.Module{Path: mod}}, "dev"},
		{"go test binary", &debug.BuildInfo{Main: debug.Module{}}, "dev"},
		{"another main module", &debug.BuildInfo{Main: debug.Module{Path: "example.com/other", Version: "v1.0.0"}}, "dev"},
		{"a space in the version", &debug.BuildInfo{Main: debug.Module{Path: mod, Version: "v1 2"}}, "dev"},
	} {
		if got := fromBuildInfo(c.bi); got != c.want {
			t.Errorf("%s: fromBuildInfo = %q, want %q", c.name, got, c.want)
		}
	}
}
