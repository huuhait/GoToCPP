package typedinterop

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// TestResolveMode pins the order of the sources of the mode: the flag, GOCPP_INTEROP,
// GOCPP_TYPED_INTEROP (the older spelling), the default.
func TestResolveMode(t *testing.T) {
	tests := []struct {
		name string
		flag string
		env  map[string]string
		want Setting
		err  string
	}{
		{"nothing", "", nil, Setting{DefaultMode, "default"}, ""},
		{"flag typed", "typed", nil, Setting{Typed, "-interop"}, ""},
		{"flag opaque", "opaque", nil, Setting{Opaque, "-interop"}, ""},
		{"flag auto", "auto", nil, Setting{Auto, "-interop"}, ""},
		{"env typed", "", map[string]string{"GOCPP_INTEROP": "typed"}, Setting{Typed, "GOCPP_INTEROP"}, ""},
		{"env opaque", "", map[string]string{"GOCPP_INTEROP": "opaque"}, Setting{Opaque, "GOCPP_INTEROP"}, ""},
		{"env auto", "", map[string]string{"GOCPP_INTEROP": " auto "}, Setting{Auto, "GOCPP_INTEROP"}, ""},
		{"flag wins over env", "opaque", map[string]string{"GOCPP_INTEROP": "typed"}, Setting{Opaque, "-interop"}, ""},
		{"flag wins over legacy", "opaque", map[string]string{"GOCPP_TYPED_INTEROP": "1"}, Setting{Opaque, "-interop"}, ""},
		{"env wins over legacy", "", map[string]string{"GOCPP_INTEROP": "opaque", "GOCPP_TYPED_INTEROP": "1"}, Setting{Opaque, "GOCPP_INTEROP"}, ""},
		{"legacy 1 is typed", "", map[string]string{"GOCPP_TYPED_INTEROP": "1"}, Setting{Typed, "GOCPP_TYPED_INTEROP"}, ""},
		{"legacy 0 is opaque", "", map[string]string{"GOCPP_TYPED_INTEROP": "0"}, Setting{Opaque, "GOCPP_TYPED_INTEROP"}, ""},
		{"legacy other is ignored", "", map[string]string{"GOCPP_TYPED_INTEROP": "yes"}, Setting{DefaultMode, "default"}, ""},
		{"empty env is unset", "", map[string]string{"GOCPP_INTEROP": "", "GOCPP_TYPED_INTEROP": "1"}, Setting{Typed, "GOCPP_TYPED_INTEROP"}, ""},
		{"bad flag", "fast", nil, Setting{}, `invalid -interop value "fast": want typed, opaque or auto`},
		{"bad env", "", map[string]string{"GOCPP_INTEROP": "fast"}, Setting{}, `GOCPP_INTEROP=fast: want typed, opaque or auto`},
		{"bad env hidden by the flag", "typed", map[string]string{"GOCPP_INTEROP": "fast"}, Setting{Typed, "-interop"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveMode(tt.flag, envOf(tt.env))
			if tt.err != "" {
				if err == nil || err.Error() != tt.err {
					t.Fatalf("error = %v, want %q", err, tt.err)
				}
				var me *ModeError
				if !errors.As(err, &me) {
					t.Fatalf("error %T is not a *ModeError", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSettingString(t *testing.T) {
	for _, tt := range []struct {
		s    Setting
		want string
	}{
		{Setting{Typed, "-interop"}, "-interop=typed"},
		{Setting{Auto, "GOCPP_INTEROP"}, "GOCPP_INTEROP=auto"},
		{Setting{Typed, "GOCPP_TYPED_INTEROP"}, "GOCPP_TYPED_INTEROP=1"},
		{Setting{Opaque, "GOCPP_TYPED_INTEROP"}, "GOCPP_TYPED_INTEROP=0"},
		{Setting{Auto, "default"}, "the default mode"},
	} {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("%+v: %q, want %q", tt.s, got, tt.want)
		}
	}
}

// TestDecide pins the clang rule: opaque never; nothing to probe never; typed without clang++ is
// an error that wraps ErrNoClang; auto without clang++ falls back with a note; with clang++ both
// are on.
func TestDecide(t *testing.T) {
	ctx := context.Background()
	t.Run("opaque", func(t *testing.T) {
		if d := Decide(ctx, Setting{Opaque, "-interop"}, true); d.On || d.Err != nil {
			t.Fatalf("%+v", d)
		}
	})
	t.Run("nothing to probe", func(t *testing.T) {
		t.Setenv("GOCPP_CLANG", "/nonexistent/clang++")
		for _, m := range []Mode{Typed, Auto} {
			if d := Decide(ctx, Setting{m, "-interop"}, false); d.On || d.Err != nil {
				t.Fatalf("%v: %+v: a program of Go alone needs no clang", m, d)
			}
		}
	})
	t.Run("typed without clang", func(t *testing.T) {
		t.Setenv("GOCPP_CLANG", "/nonexistent/clang++")
		d := Decide(ctx, Setting{Typed, "-interop"}, true)
		if d.On || !errors.Is(d.Err, ErrNoClang) {
			t.Fatalf("%+v", d)
		}
		for _, want := range []string{"-interop=typed", "install clang++", "GOCPP_CLANG", "-interop=opaque"} {
			if !contains(d.Err.Error(), want) {
				t.Errorf("the error %q lacks %q", d.Err, want)
			}
		}
	})
	t.Run("auto without clang", func(t *testing.T) {
		t.Setenv("GOCPP_CLANG", "/nonexistent/clang++")
		d := Decide(ctx, Setting{Auto, "default"}, true)
		if d.On || d.Err != nil || d.Note == "" {
			t.Fatalf("%+v: want off, no error, a note for -v", d)
		}
	})
	t.Run("auto on a Windows host", func(t *testing.T) {
		defer func(old string) { hostOS = old }(hostOS)
		hostOS = "windows"
		if d := Decide(ctx, Setting{Auto, "default"}, true); d.On || d.Err != nil || d.Note == "" {
			t.Fatalf("%+v: want off with a note, whatever clang++ is there", d)
		}
		if _, err := exec.LookPath("clang++"); err == nil {
			if d := Decide(ctx, Setting{Typed, "-interop"}, true); !d.On {
				t.Fatalf("%+v: typed asks clang++ on any host", d)
			}
		}
	})
	t.Run("with clang", func(t *testing.T) {
		if _, err := exec.LookPath("clang++"); err != nil {
			t.Skip("no clang++")
		}
		t.Setenv("GOCPP_CLANG", "")
		for _, m := range []Mode{Typed, Auto} {
			if d := Decide(ctx, Setting{m, "-interop"}, true); !d.On || d.Err != nil {
				t.Fatalf("%v: %+v", m, d)
			}
		}
	})
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestParseMode(t *testing.T) {
	for _, s := range []string{"typed", "opaque", "auto"} {
		m, ok := ParseMode(s)
		if !ok || m.String() != s {
			t.Errorf("ParseMode(%q) = %v, %v", s, m, ok)
		}
	}
	for _, s := range []string{"", "Typed", "on", "1"} {
		if _, ok := ParseMode(s); ok {
			t.Errorf("ParseMode(%q) accepts", s)
		}
	}
}

func pkgWith(std bool, headers ...string) *load.Package {
	f := &load.File{Info: &load.FileInfo{}}
	for _, h := range headers {
		kind := load.SystemHeader
		if strings.HasPrefix(h, ".") {
			kind = load.LocalHeader
		}
		f.Info.Includes = append(f.Info.Includes, &load.Include{Kind: kind, Path: h})
	}
	return &load.Package{Std: std, Files: []*load.File{f, {}}}
}

// TestNeeded: a program has something to probe when a package that is not the standard library's imports a
// header; a file without header information (a test file, a syntax error) counts as none.
func TestNeeded(t *testing.T) {
	if Needed(nil) || Needed([]*load.Package{nil, pkgWith(false)}) {
		t.Error("a program without header imports needs no probe")
	}
	if !Needed([]*load.Package{pkgWith(false), pkgWith(false, "<vector>")}) {
		t.Error("a header import needs the probe")
	}
	if Needed([]*load.Package{pkgWith(true, "<vector>")}) {
		t.Error("the standard library's own packages are not probed")
	}
}

// TestStdHeadersOnly: a cross build is typed for the headers of the C++ library only.
func TestStdHeadersOnly(t *testing.T) {
	for _, tt := range []struct {
		headers []string
		want    bool
	}{
		{nil, true},
		{[]string{"<vector>", "<cmath>", "<unordered_map>"}, true},
		{[]string{"<vector>", "<raylib.h>"}, false},
		{[]string{"<stdio.h>"}, false},
		{[]string{"./lib.hpp"}, false},
	} {
		if got := StdHeadersOnly(&check.Package{Load: pkgWith(false, tt.headers...)}); got != tt.want {
			t.Errorf("%v: %v, want %v", tt.headers, got, tt.want)
		}
	}
}

// TestHooksCross: for a target other than this machine, the members hook answers only for packages whose
// headers are all the C++ library's (the probes read this machine's headers); without clang++ the hook says
// so (check then goes on in opaque mode). The test has no clang++: GOCPP_CLANG names a program that is not there.
func TestHooksCross(t *testing.T) {
	t.Setenv("GOCPP_CLANG", "/nonexistent/clang++")
	qs := []check.MemberQuery{{Name: "size"}}
	other := "windows"
	if runtime.GOOS == "windows" {
		other = "linux"
	}
	_, members := Hooks(HookOptions{GOOS: other})
	std := &check.Package{Load: pkgWith(false, "<vector>")}
	own := &check.Package{Load: pkgWith(false, "<vector>", "./lib.hpp")}
	if a, err := members(context.Background(), own, qs); a != nil || err != nil {
		t.Errorf("a cross build with a header of its own: %v, %v; want nothing", a, err)
	}
	if _, err := members(context.Background(), std, qs); !errors.Is(err, ErrNoClang) {
		t.Errorf("a cross build with C++ library headers asks the compiler: %v, want ErrNoClang", err)
	}
	_, here := Hooks(HookOptions{})
	for _, p := range []*check.Package{std, own} {
		if _, err := here(context.Background(), p, qs); !errors.Is(err, ErrNoClang) {
			t.Errorf("a build for this machine asks the compiler: %v, want ErrNoClang", err)
		}
	}
}
