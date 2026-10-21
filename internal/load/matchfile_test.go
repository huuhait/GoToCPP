package load

import (
	"fmt"
	"go/build"
	"io"
	"strings"
	"testing"
)

// MatchFile is the rule of Load for one file, for the tools that read a
// folder without Load (gocpp doc). It is go/build's rule (file name suffixes
// and //go:build lines) for the compiler "gocpp": the tags gc and cgo are
// false, so a file that needs them is hidden (go doc shows it).
func TestMatchFile(t *testing.T) {
	targets := []struct{ goos, goarch string }{
		{"linux", "amd64"}, {"linux", "arm64"}, {"linux", "386"}, {"windows", "amd64"},
		{"windows", "arm64"}, {"darwin", "arm64"}, {"darwin", "amd64"}, {"freebsd", "amd64"},
	}
	cases := []struct {
		name string
		src  string
		// gcpp reports the files that differ from go/build with the
		// compiler gc, by design: set want for them.
		differs bool
		want    bool
	}{
		{name: "plain.gp", src: "package p\n"},
		{name: "x_linux.gp", src: "package p\n"},
		{name: "x_windows.gp", src: "package p\n"},
		{name: "x_darwin.gp", src: "package p\n"},
		{name: "x_amd64.gp", src: "package p\n"},
		{name: "x_arm64.gp", src: "package p\n"},
		{name: "x_linux_amd64.gp", src: "package p\n"},
		{name: "x_windows_arm64.gp", src: "package p\n"},
		{name: "x_unix.gp", src: "package p\n"}, // not a suffix: always in
		{name: "x_test.gp", src: "package p\n"},
		{name: "x_linux_test.gp", src: "package p\n"},
		{name: "c.gp", src: "//go:build linux\n\npackage p\n"},
		{name: "c.gp", src: "//go:build !linux\n\npackage p\n"},
		{name: "c.gp", src: "//go:build linux && amd64\n\npackage p\n"},
		{name: "c.gp", src: "//go:build linux || windows\n\npackage p\n"},
		{name: "c.gp", src: "//go:build unix\n\npackage p\n"},
		{name: "c.gp", src: "//go:build !windows && !plan9\n\npackage p\n"},
		{name: "c.gp", src: "//go:build (linux || darwin) && arm64\n\npackage p\n"},
		{name: "c.gp", src: "//go:build go1.18\n\npackage p\n"},
		{name: "c.gp", src: "//go:build go1.99\n\npackage p\n"},
		{name: "c.gp", src: "//go:build ignore\n\npackage p\n"},
		{name: "c.gp", src: "//go:build fancy\n\npackage p\n"},
		{name: "c.gp", src: "// Copyright.\n\n//go:build linux\n\npackage p\n"},
		{name: "c_windows.gp", src: "//go:build linux\n\npackage p\n"},
		{name: "c.gp", src: "/* block */\n//go:build windows\n\npackage p\n"},
		// The compiler is gocpp: gc and cgo are false.
		{name: "c.gp", src: "//go:build gc\n\npackage p\n", differs: true, want: false},
		{name: "c.gp", src: "//go:build !gc\n\npackage p\n", differs: true, want: true},
		{name: "c.gp", src: "//go:build cgo\n\npackage p\n", differs: true, want: false},
		{name: "c.gp", src: "//go:build !cgo\n\npackage p\n", differs: true, want: true},
		{name: "c.gp", src: "//go:build gc && linux\n\npackage p\n", differs: true, want: false},
		{name: "c.gp", src: "//go:build gccgo\n\npackage p\n", differs: false},
	}
	for _, tags := range [][]string{nil, {"fancy"}} {
		for _, tg := range targets {
			ref := build.Context{GOOS: tg.goos, GOARCH: tg.goarch, Compiler: "gc", BuildTags: tags}
			for i := 1; i <= goMinor; i++ {
				ref.ReleaseTags = append(ref.ReleaseTags, fmt.Sprintf("go1.%d", i))
			}
			for _, c := range cases {
				src := c.src
				ref.OpenFile = func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(src)), nil }
				want := c.want
				if !c.differs {
					var err error
					want, err = ref.MatchFile("", strings.TrimSuffix(c.name, ".gp")+".go")
					if err != nil {
						t.Fatal(err)
					}
				}
				got, msg := MatchFile(tg.goos, tg.goarch, tags, c.name, []byte(src))
				if got != want || msg != "" {
					t.Errorf("%s/%s %v %s %q: got %v %q, want %v", tg.goos, tg.goarch, tags, c.name, src, got, msg, want)
				}
			}
		}
	}
}

// A broken constraint is a message and false.
func TestMatchFileBroken(t *testing.T) {
	src := "//go:build linux\n//go:build windows\n\npackage p\n"
	ok, msg := MatchFile("linux", "amd64", nil, "x.gp", []byte(src))
	if ok || !strings.Contains(msg, "multiple //go:build comments") {
		t.Errorf("got %v %q", ok, msg)
	}
	// A file that is not a .gp file keeps its name: x_linux.c.
	ok, _ = MatchFile("windows", "amd64", nil, "x_linux.c", []byte("int x;\n"))
	if ok {
		t.Errorf("x_linux.c is in a windows package")
	}
}

// BuildTags is what the if blocks of a cxx recipe ask (ADR-129): the tags of a
// //go:build line for the target, with the same answers as go/build for the
// ones that it knows, and false for a tag that nobody knows.
func TestBuildTags(t *testing.T) {
	targets := []struct{ goos, goarch string }{
		{"linux", "amd64"}, {"linux", "arm64"}, {"windows", "amd64"}, {"windows", "arm64"},
		{"darwin", "arm64"}, {"freebsd", "amd64"}, {"android", "arm64"}, {"ios", "arm64"},
		{"illumos", "amd64"},
	}
	tags := []string{
		"linux", "windows", "darwin", "freebsd", "android", "ios", "illumos", "solaris",
		"amd64", "arm64", "386", "unix", "go1.18", "go1.99", "fancy", "ignore", "widnows", "cgo", "gc",
	}
	for _, extra := range [][]string{nil, {"fancy"}} {
		for _, tg := range targets {
			ref := build.Context{GOOS: tg.goos, GOARCH: tg.goarch, Compiler: "gocpp", BuildTags: extra}
			for i := 1; i <= goMinor; i++ {
				ref.ReleaseTags = append(ref.ReleaseTags, fmt.Sprintf("go1.%d", i))
			}
			tag := BuildTags(tg.goos, tg.goarch, extra)
			for _, name := range tags {
				src := "//go:build " + name + "\n\npackage p\n"
				ref.OpenFile = func(string) (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader(src)), nil
				}
				want, err := ref.MatchFile("", "x.go")
				if err != nil {
					t.Fatal(err)
				}
				// Asked twice: the second answer is the remembered one.
				if got, again := tag(name), tag(name); got != want || again != want {
					t.Errorf("%s/%s %v %s = %v, %v; go/build says %v",
						tg.goos, tg.goarch, extra, name, got, again, want)
				}
			}
		}
	}
	// The names that a recipe uses most.
	tag := BuildTags("linux", "arm64", nil)
	answers := map[string]bool{
		"linux": true, "unix": true, "arm64": true, "gocpp": true,
		"windows": false, "amd64": false, "cgo": false, "nonsense": false,
	}
	for name, want := range answers {
		if got := tag(name); got != want {
			t.Errorf("linux/arm64 %s = %v, want %v", name, got, want)
		}
	}
}
