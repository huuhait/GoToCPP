package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRecipe(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want Recipe
	}{
		{"run", "// run\n\npackage main\n", Recipe{Action: "run"}},
		{"runoutput", "// runoutput\npackage main", Recipe{Action: "runoutput"}},
		{"errorcheck", "// errorcheck\npackage p", Recipe{Action: "errorcheck", WantError: true}},
		{"errorcheck -0", "// errorcheck -0 -m\npackage p", Recipe{Action: "errorcheck", Flags: []string{"-m"}}},
		{"compile", "// compile\npackage p", Recipe{Action: "compile"}},
		{"compile flags", "// compile -p=main -l\npackage p", Recipe{Action: "compile", Flags: []string{"-p=main", "-l"}}},
		{"rundir", "// rundir\npackage ignored", Recipe{Action: "rundir"}},
		{"errorcheckdir", "// errorcheckdir\npackage ignored", Recipe{Action: "errorcheckdir", WantError: true}},
		{"errorcheckandrundir -1", "// errorcheckandrundir -1\n", Recipe{Action: "errorcheckandrundir", WantError: true}},
		{"errorcheckoutput", "// errorcheckoutput\n", Recipe{Action: "errorcheckoutput", WantError: true}},
		{"errorcheckwithauto", "// errorcheckwithauto -0\n", Recipe{Action: "errorcheck", Flags: []string{"-auto"}, WantError: false}},
		{"build", "// build\n", Recipe{Action: "build"}},
		{"buildrun", "// buildrun\n", Recipe{Action: "buildrun"}},
		{"buildrundir", "// buildrundir\n", Recipe{Action: "buildrundir"}},
		{"builddir", "// builddir\n", Recipe{Action: "builddir"}},
		{"runindir", "// runindir -gomodversion \"1.21\"\n", Recipe{Action: "runindir", ModVers: "1.21"}},
		{"asmcheck", "// asmcheck\n", Recipe{Action: "asmcheck"}},
		{"skip", "// skip\n", Recipe{Action: "skip"}},
		{"timeout", "// run -t 30 arg1 arg2\n", Recipe{Action: "run", Timeout: 30, Args: []string{"arg1", "arg2"}}},
		{"single", "// compiledir -s\n", Recipe{Action: "compiledir", Single: true}},
		{"goexperiment", "// run -goexperiment rangefunc\n", Recipe{Action: "run", GoExp: []string{"rangefunc"}}},
		{"godebug", "// run -godebug x=1\n", Recipe{Action: "run", GoDebug: []string{"x=1"}}},
		{"quoted", "// run 'a b' \"c d\"\n", Recipe{Action: "run", Args: []string{"a b", "c d"}}},
		{"after build line", "//go:build !js\n\n// run\n\npackage main\n", Recipe{Action: "run"}},
		{"after plus build", "// +build !js\n\n// run\n", Recipe{Action: "run"}},
		{"blank lines first", "\n\n// run\n", Recipe{Action: "run"}},
	}
	for _, c := range cases {
		got, err := ParseRecipe(c.src)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		got = normalizeRecipe(got)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestParseRecipeErrors(t *testing.T) {
	for src, want := range map[string]string{
		"package p\n":     "unknown pattern",
		"// frobnicate\n": "unknown pattern",
		"":                "not found",
		"// run 'x\n":     "unclosed quote",
		"// run -t abc\n": "need number of seconds",
		"// run -t\n":     "needs an argument",
		"// run \\\n":     "",
	} {
		_, err := ParseRecipe(src)
		if want == "" {
			continue
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error %v, want one with %q", src, err, want)
		}
	}
}

func TestSplitQuoted(t *testing.T) {
	got, err := splitQuoted(`a b:"c d" 'e''f'  "g\""`)
	if err != nil || !reflect.DeepEqual(got, []string{"a", "b:c d", "ef", `g"`}) {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := splitQuoted(`"open`); err == nil {
		t.Error("unclosed quote: no error")
	}
}

func TestShouldTest(t *testing.T) {
	tgt := Target{GOOS: "linux", GOARCH: "amd64", Minor: 25}
	cases := []struct {
		src string
		ok  bool
	}{
		{"// run\n\npackage main\n", true},
		{"// run\n\n//go:build !windows\n\npackage main\n", true},
		{"// run\n\n//go:build windows\n\npackage main\n", false},
		{"//go:build linux && amd64\n\n// run\n\npackage main\n", true},
		{"//go:build linux && arm64\n\n// run\n\npackage main\n", false},
		{"//go:build cgo\n\n// run\n\npackage main\n", false},
		{"//go:build !cgo\n\n// run\n\npackage main\n", true},
		{"//go:build go1.20\n\n// run\n\npackage main\n", true},
		{"//go:build go1.26\n\n// run\n\npackage main\n", false},
		{"//go:build gc\n\n// run\n\npackage main\n", true},
		{"//go:build goexperiment.arenas\n\n// run\n\npackage main\n", false},
		{"//go:build !goexperiment.arenas\n\n// run\n\npackage main\n", true},
		{"//go:build ignore\n\n// run\n\npackage main\n", false},
		// a constraint after the package clause is not one
		{"// run\n\npackage main\n\n//go:build windows\n", true},
	}
	for _, c := range cases {
		if ok, why := tgt.shouldTest(c.src); ok != c.ok {
			t.Errorf("%q: ok = %v (%s), want %v", c.src, ok, why, c.ok)
		}
	}
}

// normalizeRecipe turns empty slices into nil so DeepEqual compares contents.
func normalizeRecipe(r Recipe) Recipe {
	if len(r.Flags) == 0 {
		r.Flags = nil
	}
	if len(r.Args) == 0 {
		r.Args = nil
	}
	if len(r.GoExp) == 0 {
		r.GoExp = nil
	}
	if len(r.GoDebug) == 0 {
		r.GoDebug = nil
	}
	return r
}
