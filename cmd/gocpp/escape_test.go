package main

import (
	"flag"
	"io"
	"strings"
	"testing"
)

// escapeProg is a program with one object that stays on the stack and one that escapes.
const escapeProg = `package main

import "fmt"

type Counter struct{ n int }

func (c *Counter) Inc() { c.n++ }

var keep *Counter

func main() {
	var local Counter
	local.Inc()
	var shared Counter
	keep = &shared
	fmt.Println(local.n, shared.n)
}
`

func TestEscapeFlagValues(t *testing.T) {
	tests := []struct {
		args    []string
		level   escapeLevel
		noEsc   bool
		wantErr bool
	}{
		{nil, 0, false, false},
		{[]string{"-gp:m"}, 1, false, false},
		{[]string{"-gp:m=2"}, 2, false, false},
		{[]string{"-gp:m=1"}, 1, false, false},
		{[]string{"-gp:m=0"}, 0, false, false},
		{[]string{"-gp:m=3"}, 0, false, true},
		{[]string{"-gp:m=x"}, 0, false, true},
		{[]string{"-gp:noescape"}, 0, true, false},
		{[]string{"-gp:m", "-gp:noescape"}, 1, true, false},
	}
	for _, tt := range tests {
		var f buildFlags
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		f.registerEscape(fs)
		err := fs.Parse(tt.args)
		if (err != nil) != tt.wantErr {
			t.Errorf("%v: error %v, want error %v", tt.args, err, tt.wantErr)
			continue
		}
		if err == nil && (f.escape != tt.level || f.noEscape != tt.noEsc) {
			t.Errorf("%v: level %d, noescape %v, want %d, %v", tt.args, f.escape, f.noEscape, tt.level, tt.noEsc)
		}
	}
}

func TestEscapeDiagnostics(t *testing.T) {
	inDir(t, map[string]string{"main.gp": escapeProg})
	t.Run("off", func(t *testing.T) {
		r := gocppRun("", false, "emit", "main.gp")
		if r.code != 0 || r.stderr != "" {
			t.Errorf("gocpp emit = %d, stderr %q, want no diagnostics", r.code, r.stderr)
		}
		if !strings.Contains(r.stdout, "\n    Counter local{};\n") || strings.Contains(r.stdout, "gp::ptr<Counter> local") {
			t.Errorf("the C++ does not keep local on the stack:\n%s", r.stdout)
		}
		if !strings.Contains(r.stdout, "gp::ptr<Counter> shared = gp::new_<Counter>();") {
			t.Errorf("the C++ does not box shared:\n%s", r.stdout)
		}
	})
	t.Run("m", func(t *testing.T) {
		r := gocppRun("", false, "emit", "-gp:m", "main.gp")
		for _, line := range []string{
			"# command-line-arguments\n",
			"./main.gp:7:7: c does not escape\n",
			"./main.gp:13:2: &local does not escape\n",
			"./main.gp:14:6: moved to heap: shared\n",
			"./main.gp:15:9: &shared escapes to heap\n",
		} {
			if !strings.Contains(r.stderr, line) {
				t.Errorf("gocpp emit -gp:m: stderr has no line %q:\n%s", line, r.stderr)
			}
		}
		if strings.Contains(r.stderr, ": it ") || strings.Contains(r.stderr, "because") {
			t.Errorf("gocpp emit -gp:m says why:\n%s", r.stderr)
		}
		if strings.Contains(r.stderr, "fmt") || strings.Count(r.stderr, "# ") != 1 {
			t.Errorf("gocpp emit -gp:m reports a package of the standard library:\n%s", r.stderr)
		}
	})
	t.Run("m=2", func(t *testing.T) {
		r := gocppRun("", false, "emit", "-gp:m=2", "main.gp")
		if !strings.Contains(r.stderr, "./main.gp:14:6: moved to heap: shared: ") {
			t.Errorf("gocpp emit -gp:m=2 does not say why shared moved to the heap:\n%s", r.stderr)
		}
	})
	t.Run("noescape", func(t *testing.T) {
		r := gocppRun("", false, "emit", "-gp:noescape", "main.gp")
		if r.code != 0 || !strings.Contains(r.stdout, "gp::ptr<Counter> local = gp::new_<Counter>();") {
			t.Errorf("gocpp emit -gp:noescape still keeps local on the stack:\n%s", r.stdout)
		}
		r = gocppRun("", false, "emit", "-gp:m", "-gp:noescape", "main.gp")
		if strings.Contains(r.stderr, "&local does not escape") || !strings.Contains(r.stderr, "moved to heap: local") {
			t.Errorf("gocpp emit -gp:m -gp:noescape reports stack objects:\n%s", r.stderr)
		}
	})
}

func TestEscapeStagedAllocation(t *testing.T) {
	for _, tt := range []struct {
		name    string
		binding string
		result  string
		staged  bool
	}{
		{"short declaration", "p := &Cnt{n: p.n + 1}", "p.n", true},
		{"var declaration", "var p = &Cnt{n: p.n + 1}", "p.n", true},
		{"multiple values", "p, n := &Cnt{n: p.n + 1}, p.n", "p.n + n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := "package main\ntype Cnt struct{ n int }\n" +
				"func (c *Cnt) Inc() { c.n++ }\nfunc probe() int {\n" +
				"p := &Cnt{n: 2}\n{\n" + tt.binding + "\np.Inc()\nreturn " +
				tt.result + "\n}\n}\nfunc main() { println(probe()) }\n"
			inDir(t, map[string]string{"main.gp": source})
			for _, flag := range []string{"-gp:m", "-gp:m=2"} {
				r := gocppRun("", false, "emit", flag, "main.gp")
				if r.code != 0 {
					t.Fatalf("gocpp emit %s: exit %d:\n%s", flag, r.code, r.stderr)
				}
				if strings.Contains(r.stdout, "static_cast<gp::local_ptr") {
					t.Errorf("staging moves a local_ptr:\n%s", r.stdout)
				}
				if strings.Count(r.stdout, "gp::local_new<Cnt>") != 1 ||
					strings.Count(r.stdout, "gp::new_<Cnt>") != 1 {
					t.Errorf("want outer stack object and inner heap object:\n%s", r.stdout)
				}
				if !strings.Contains(r.stderr, "&Cnt{...} escapes to heap") {
					t.Errorf("%s does not report the heap fallback:\n%s", flag, r.stderr)
				}
				if tt.staged && flag == "-gp:m=2" &&
					!strings.Contains(r.stderr, "staging before the shadowing declaration") {
					t.Errorf("%s does not explain the staging fallback:\n%s", flag, r.stderr)
				}
			}
		})
	}
}

// newToBorrowed pins that the text of -gp:m is true for new(T) and &T{} that go straight to a
// borrowed parameter: &T{} is a temporary of the call (no allocation), new(T) still allocates.
const newToBorrowed = `package main

type Cnt struct{ n int }

func bump(c *Cnt) { c.n++ }

func main() {
	bump(new(Cnt))
	bump(&Cnt{})
}
`

func TestEscapeNewToBorrowed(t *testing.T) {
	inDir(t, map[string]string{"main.gp": newToBorrowed})
	for _, flag := range []string{"-gp:m", "-gp:m=2"} {
		t.Run(flag, func(t *testing.T) {
			r := gocppRun("", false, "emit", flag, "main.gp")
			if r.code != 0 {
				t.Fatalf("gocpp emit %s: exit %d:\n%s", flag, r.code, r.stderr)
			}
			why := ""
			if flag == "-gp:m=2" {
				why = ": it is not held by a local variable"
			}
			for _, line := range []string{
				"./main.gp:8:7: new(Cnt) escapes to heap" + why + "\n",
				"./main.gp:9:7: &Cnt{} does not escape\n",
			} {
				if !strings.Contains(r.stderr, line) {
					t.Errorf("gocpp emit %s: missing %q:\n%s", flag, line, r.stderr)
				}
			}
			for _, code := range []string{"bump(gp::new_<Cnt>());", "bump(gp::borrow_tmp(Cnt{}));"} {
				if !strings.Contains(r.stdout, code) {
					t.Errorf("gocpp emit: the C++ has no %q:\n%s", code, r.stdout)
				}
			}
		})
	}
}
