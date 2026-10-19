package server

import (
	"strings"
	"testing"
)

// renamed returns the names of the files that a rename changes, or the error.
func (c *refClient) renamed(t *testing.T, file, mark, name string) (map[string]string, string) {
	t.Helper()
	got, rerr := c.rename(file, mark, name)
	if rerr != nil {
		return nil, rerr.Message
	}
	return got, ""
}

// TestFamilyErrorMethod: a method named Error that a use ties to the universe
// interface error (return E{} as an error) is no crash (objectName of the
// method of a type without a package), its references are the method's own
// (not every Error method of the module, which the tie to error would join),
// and a rename of it is refused: error is outside the workspace.
func TestFamilyErrorMethod(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/errm\n",
		"m/m.gp": `package m

type E struct{}

func (E) Error() string { return "e" }

type F struct{}

func (F) Error() string { return "f" }

func Fail() error { return E{} }

func Fail2() error { return F{} }

func Use(err error) string { return err.Error() + E{}.Error() }
`,
	}
	for _, wide := range []bool{false, true} {
		c := startRefs(t, files, plainInit)
		c.s.wideFamily = wide
		got := c.references("m/m.gp", "(E) Err|or()", true)
		want := []string{"m/m.gp:5:10 Error", "m/m.gp:15:55 Error"}
		if !wide {
			sameList(t, "references of E.Error", got, want)
		}
		_, msg := c.renamed(t, "m/m.gp", "(E) Err|or()", "Fail3")
		if !wide && !strings.Contains(msg, "outside the workspace") || wide && msg == "" {
			t.Errorf("wide %v: rename of E.Error: %q, want the refusal that error is outside the workspace", wide, msg)
		}
		// The server is alive and answers.
		if got := c.references("m/m.gp", "func Fa|il()", true); len(got) != 1 {
			t.Errorf("references of Fail: %q", got)
		}
	}
}

// TestFamilyBrokenCode: shapes of broken code that leave an expression without
// a type (a send to an undefined channel, a call of an undefined function, a
// composite literal of an undefined type) don't stop the search of a method.
func TestFamilyBrokenCode(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/brk\n",
		"m/m.gp": `package m

type T struct{}

func (T) M() {}

type I interface{ M() }

func f() {
	undefinedch <- 1
	undefinedfn(T{})
	_ = Undefined{T{}}
	_ = []Undefined{T{}}
	var x I = undefinedv
	_ = x.(Undefined)
	switch undefinedv.(type) {
	case T:
	}
}

func g() I { return T{} }
`,
	}
	c := startRefs(t, files, plainInit)
	got := c.references("m/m.gp", "(T) |M()", true)
	sameList(t, "references of T.M", got, []string{"m/m.gp:5:10 M", "m/m.gp:7:19 M"})
}

// TestFamilyChannelSend: a value sent on a channel of an interface type ties
// its type's methods to the interface's.
func TestFamilyChannelSend(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/snd\n",
		"m/m.gp": `package m

type S interface{ M() }

type T struct{}

func (T) M() {}

type U struct{}

func (U) M() {}

func send(ch chan S) { ch <- T{} }
`,
	}
	c := startRefs(t, files, plainInit)
	got := c.references("m/m.gp", "(T) |M()", true)
	sameList(t, "references of T.M", got, []string{"m/m.gp:3:19 M", "m/m.gp:7:10 M"})
	// U is not sent anywhere: not in the family.
	got = c.references("m/m.gp", "(U) |M()", true)
	sameList(t, "references of U.M", got, []string{"m/m.gp:11:10 M"})
}

// TestFamilyUnnamedTypes: an interface literal, the constraint of a type
// parameter and a struct that embeds a type (and so has its methods) take part
// in the family like named types do.
func TestFamilyUnnamedTypes(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/unn\n",
		"base/base.gp": `package base

type Getter interface{ Get() int }

type Base struct{ n int }

func (b Base) Get() int { return b.n }
`,
		"m/m.gp": `package m

import "example.com/unn/base"

type Unrelated struct{ n int }

func (u Unrelated) Len() int { return u.n }

func AnonUse(x interface{ Len() int }) int { return x.Len() }

func CallAnon() int { return AnonUse(Unrelated{n: 9}) }

type Sq struct{}

func (Sq) Area() float64 { return 1 }

func Total[S interface{ Area() float64 }](xs ...S) float64 {
	t := 0.0
	for _, x := range xs {
		t += x.Area()
	}
	return t
}

func UseTotal() float64 { return Total(Sq{}) }

func Tagged() struct {
	base.Base
	Tag string
} {
	return struct {
		base.Base
		Tag string
	}{base.Base{}, "t"}
}

func AsGetter(g base.Getter) int { return g.Get() }

func UseTagged() int { return AsGetter(Tagged()) }
`,
	}
	c := startRefs(t, files, plainInit)
	for _, tc := range []struct {
		file, mark, name string
		n                int
	}{
		{"m/m.gp", "(u Unrelated) Le|n()", "Size", 3}, // the method, the literal's, the call through it
		{"m/m.gp", "interface{ Le|n() int }", "Size", 3},
		{"m/m.gp", "(Sq) Ar|ea()", "Surface", 3}, // the method, the constraint's, the call through it
		{"m/m.gp", "interface{ Ar|ea() float64 }", "Surface", 3},
		{"base/base.gp", "(b Base) G|et()", "Fetch", 3}, // the method, the interface's, the call through it
	} {
		got, msg := c.renamed(t, tc.file, tc.mark, tc.name)
		if msg != "" {
			t.Errorf("rename at %q: %s", tc.mark, msg)
			continue
		}
		n := 0
		for _, text := range got {
			n += strings.Count(text, tc.name)
		}
		if n != tc.n {
			t.Errorf("rename at %q to %s: %d occurrences, want %d (%v)", tc.mark, tc.name, n, tc.n, got)
		}
	}
}

// TestSemanticTokensTwoFiles: the tokens of a file come from that file, also
// when the package has another file and the tokens of both were asked for (the
// whole-file tokens are kept per package, by file).
func TestSemanticTokensTwoFiles(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/sem2\n",
		"p/a.gp":    "package p\n\nfunc A() int { return B() }\n",
		"p/b.gp":    "package p\n\nvar Long = 1\n\nfunc B() int { return Long }\n",
	}
	c := startRefs(t, files, plainInit)
	a1, b1 := c.semTokens("p/a.gp", nil), c.semTokens("p/b.gp", nil)
	a2, b2 := c.semTokens("p/a.gp", nil), c.semTokens("p/b.gp", nil)
	if len(a1) == 0 || len(b1) == 0 || strings.Join(a1, "|") == strings.Join(b1, "|") {
		t.Fatalf("the tokens of the two files: %q and %q", a1, b1)
	}
	if strings.Join(a1, "|") != strings.Join(a2, "|") || strings.Join(b1, "|") != strings.Join(b2, "|") {
		t.Errorf("the second ask differs: %q %q / %q %q", a1, a2, b1, b2)
	}
	if !strings.Contains(strings.Join(b1, "|"), "Long") || strings.Contains(strings.Join(a1, "|"), "Long") {
		t.Errorf("tokens of b.gp %q, of a.gp %q", b1, a1)
	}
}

// TestRenameWideMessageLiteral: in the wide family a type whose only use is an
// interface literal (AnonUse(Unrelated{})) is not listed as one that no use
// needs.
func TestRenameWideMessageLiteral(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/wl\n",
		"m/m.gp": `package m

type Lener interface{ Len() int }

type Real struct{}

func (Real) Len() int { return 1 }

type Unrelated struct{}

func (Unrelated) Len() int { return 2 }

type Idle struct{}

func (Idle) Len() int { return 3 }

func Use(l Lener) int { return l.Len() }

func AnonUse(x interface{ Len() int }) int { return x.Len() }

func f() int { return Use(Real{}) + AnonUse(Unrelated{}) }
`,
	}
	c := startRefs(t, files, plainInit)
	c.s.wideFamily = true
	if _, rerr := c.rename("m/m.gp", "x interface{ L|en() int", "Size"); rerr != nil {
		t.Fatal(rerr)
	}
	shown := c.takeShown()
	if len(shown) != 1 {
		t.Fatalf("messages: %+v", shown)
	}
	m := shown[0].Message
	i := strings.Index(m, "Not needed by any use")
	if i < 0 {
		t.Fatalf("no list of unneeded types: %s", m)
	}
	unused := m[i:]
	if strings.Contains(unused, "Unrelated") || !strings.Contains(unused, "Idle") {
		t.Errorf("the types not needed: %s", unused)
	}
}

// TestFamilyConstraintAcrossPackages: a type argument in another package is
// tied to the constraint literal of the generic function it instantiates.
func TestFamilyConstraintAcrossPackages(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/xp\n",
		"gen/gen.gp": `package gen

func Total[S interface{ Area() float64 }](xs ...S) float64 {
	t := 0.0
	for _, x := range xs {
		t += x.Area()
	}
	return t
}
`,
		"app/app.gp": `package app

import "example.com/xp/gen"

type Sq struct{}

func (Sq) Area() float64 { return 1 }

func Use() float64 { return gen.Total(Sq{}) }
`,
	}
	c := startRefs(t, files, plainInit)
	for _, tc := range []struct{ file, mark string }{
		{"app/app.gp", "(Sq) Ar|ea"},
		{"gen/gen.gp", "interface{ Ar|ea"},
	} {
		got, msg := c.renamed(t, tc.file, tc.mark, "Surface")
		if msg != "" {
			t.Errorf("rename at %q: %s", tc.mark, msg)
			continue
		}
		n := 0
		for _, text := range got {
			n += strings.Count(text, "Surface")
		}
		if n != 3 { // the method, the literal's, the call through it
			t.Errorf("rename at %q: %d occurrences of Surface, want 3 (%v)", tc.mark, n, got)
		}
	}
}
