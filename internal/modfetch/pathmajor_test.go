package modfetch

import (
	"strings"
	"testing"
)

func TestParseModulePathMajor(t *testing.T) {
	for _, tc := range []struct {
		path, root, subdir string
		major              int
	}{
		{"github.com/o/r", "github.com/o/r", "", 0},
		{"github.com/o/r/v2", "github.com/o/r", "", 2},
		{"github.com/o/r/sub/v3", "github.com/o/r", "sub", 3},
		{"github.com/o/r/sub", "github.com/o/r", "sub", 0},
		{"example.com/lib/v2", "example.com/lib", "", 2},
		{"example.com/lib", "example.com/lib", "", 0},
		{"example.com/a/b/v12", "example.com/a/b", "", 12},
		{"gitlab.com/g/s/r.git/sub/v2", "gitlab.com/g/s/r.git", "sub", 2},
		{"example.com/v2", "example.com", "", 2},
	} {
		r, err := ParseModulePath(tc.path)
		if err != nil {
			t.Errorf("%s: %v", tc.path, err)
			continue
		}
		if r.Root != tc.root || r.Subdir != tc.subdir || r.Major != tc.major {
			t.Errorf("%s: root %q subdir %q major %d; want %q %q %d", tc.path, r.Root, r.Subdir, r.Major, tc.root, tc.subdir, tc.major)
		}
	}
}

func TestCheckModulePathLikeGo(t *testing.T) {
	for _, bad := range []string{"example.com/aux", "example.com/NUL/x", "example.com/com1", "example.com/lpt9.txt", "example.com/x/con.y", "example.com/lib/v1", "example.com/lib/v0", "example.com/lib/v02"} {
		if CheckModulePath(bad) == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
	for _, ok := range []string{"example.com/lib/v2", "example.com/lib/v10", "example.com/auxiliary", "example.com/console", "example.com/v1x", "github.com/o/r/v2"} {
		if err := CheckModulePath(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
}

func TestProxyListErrors(t *testing.T) {
	for _, s := range []string{",,", "direct,,off", "https://m.example/x,", "file://user:SECRET@host/dir", "file:///dir?token=SECRET", "file://host/dir"} {
		_, err := parseProxies(s)
		if err == nil {
			t.Errorf("%q was accepted", s)
			continue
		}
		if strings.Contains(err.Error(), "SECRET") {
			t.Errorf("%q: the error shows a secret: %v", s, err)
		}
	}
	if _, err := parseProxies("file:///dir,https://m.example,direct"); err != nil {
		t.Error(err)
	}
}
