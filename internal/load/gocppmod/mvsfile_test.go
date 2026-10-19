package gocppmod

import (
	"strings"
	"testing"
)

func parseOK(t *testing.T, text string) *File {
	t.Helper()
	f, errs := Parse("gocpp.mod", []byte(text), nil)
	if len(errs) > 0 {
		t.Fatalf("%v\n%s", errs[0], text)
	}
	return f
}

func parseErr(text string) string {
	_, errs := Parse("gocpp.mod", []byte(text), nil)
	if len(errs) == 0 {
		return ""
	}
	return errs[0].Error()
}

func TestExcludeLines(t *testing.T) {
	f := parseOK(t, "module m\nexclude example.com/a v1.2.3\nexclude (\n\texample.com/b v2.0.0+incompatible\n\texample.com/c v1.0.0+meta\n)\n")
	var got []string
	for _, e := range f.Exclude {
		got = append(got, e.Path+"@"+e.Version)
	}
	if strings.Join(got, " ") != "example.com/a@v1.2.3 example.com/b@v2.0.0+incompatible example.com/c@v1.0.0" {
		t.Errorf("excludes = %v", got)
	}
	for text, want := range map[string]string{
		"module m\nexclude example.com/a\n":                   "usage: exclude module/path v1.2.3",
		"module m\nexclude example.com/a v1\n":                "invalid module version",
		"module m\nexclude example.com/a v1.0.0 v1.0.1\n":     "usage: exclude",
		"module m\nexclude example.com/./a v1.0.0\n":          "invalid module path",
		"module m\nexclude \"example.com/a v1.0.0\n":          "",
		"module m\nexclude example.com/a v01.0.0\n":           "invalid module version",
		"module m\nexclude example.com/a v1.0.0-rc.01\n":      "invalid module version",
		"module m\nexclude example.com/a latest\n":            "invalid module version",
		"module m\nexclude example.com/a ../b\n":              "invalid module version",
		"module m\nexclude example.com/a v1.0.0\nexclude x\n": "usage: exclude",
	} {
		if got := parseErr(text); want == "" && got == "" || want != "" && !strings.Contains(got, want) {
			t.Errorf("%q: error %q, want %q", text, got, want)
		}
	}
}

func TestRetractLines(t *testing.T) {
	f := parseOK(t, `module m

// Published by mistake.
retract v1.0.1

retract [v1.1.0, v1.1.5] // security fix needed

retract (
	v1.2.0 // too old
	// do not use
	[v1.3.0, v1.3.9]
)
`)
	type rr struct{ low, high, why string }
	var got []rr
	for _, r := range f.Retract {
		got = append(got, rr{r.Low, r.High, r.Rationale})
	}
	want := []rr{
		{"v1.0.1", "v1.0.1", "Published by mistake."},
		{"v1.1.0", "v1.1.5", "security fix needed"},
		{"v1.2.0", "v1.2.0", "too old"},
		{"v1.3.0", "v1.3.9", "do not use"},
	}
	if len(got) != len(want) {
		t.Fatalf("retracts = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("retract %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	r := f.Retract[1]
	for v, in := range map[string]bool{"v1.0.9": false, "v1.1.0": true, "v1.1.3": true, "v1.1.5": true, "v1.1.6": false, "v1.1.5-rc.1": true} {
		if r.Contains(v) != in {
			t.Errorf("[v1.1.0, v1.1.5] contains %s = %v", v, !in)
		}
	}
}

func TestRetractErrors(t *testing.T) {
	for text, want := range map[string]string{
		"module m\nretract\n":                          "usage: retract",
		"module m\nretract v1\n":                       "invalid module version",
		"module m\nretract [v1.0.0]\n":                 "usage: retract",
		"module m\nretract [v1.0.0, v1.0.1\n":          "usage: retract",
		"module m\nretract v1.0.0, v1.0.1]\n":          "usage: retract",
		"module m\nretract [v1.0.0, v1.0.1, v1.0.2]\n": "usage: retract",
		"module m\nretract [v1.2.0, v1.1.0]\n":         "the low version is above the high version",
		"module m\nretract v1.0.0 v1.0.1\n":            "usage: retract",
	} {
		if got := parseErr(text); !strings.Contains(got, want) {
			t.Errorf("%q: error %q, want %q", text, got, want)
		}
	}
	// A retract line with the range written with spaces and quotes.
	f := parseOK(t, "module m\nretract [ \"v1.0.0\" , v1.0.5 ]\n")
	if len(f.Retract) != 1 || f.Retract[0].Low != "v1.0.0" || f.Retract[0].High != "v1.0.5" {
		t.Errorf("%+v", f.Retract)
	}
}

func TestRequireTwiceKeepsTheHighest(t *testing.T) {
	f := parseOK(t, "module m\nrequire example.com/a v1.0.0\nrequire example.com/b v1.0.0\nrequire example.com/a v1.2.0 // indirect\nrequire example.com/a v1.1.0\n")
	if len(f.Require) != 2 || f.Require[0].Path != "example.com/a" || f.Require[0].Version != "v1.2.0" || !f.Require[0].Indirect || f.Require[1].Path != "example.com/b" {
		for _, r := range f.Require {
			t.Logf("%+v", *r)
		}
		t.Errorf("requires = %d", len(f.Require))
	}
}

func TestExcludeAndRetractRoundTrip(t *testing.T) {
	text := "module m\n\nexclude example.com/a v1.0.0\n\n// why\nretract v1.0.0\n"
	f := parseOK(t, text)
	if got := string(Format(f.Syntax)); got != text {
		t.Errorf("format = %q", got)
	}
}
