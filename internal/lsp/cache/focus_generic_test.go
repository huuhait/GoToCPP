package cache

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The focused check of a package with generic declarations (focus.go, "Generics"): edits of the
// body of an ordinary function, of a generic function and of a method of a generic type, in a file
// that uses the generics of another file and declares its own. The focused diagnostics of the
// edited file must be those of the full check, and the full check must leave the other files'
// diagnostics as they were. An edit whose instantiation gives go/types' graph of instantiation
// cycles an edge of weight 1 must decline.

// genericFiles are focusFiles with generic declarations in lib/g.gp, and a generic function and a
// method of a generic type of g.gp in lib/a.gp, whose bodies have the markers // GEDIT and // MEDIT.
func genericFiles() map[string]string {
	files := focusFiles()
	files["lib/g.gp"] = `package lib

import "strings"

type Number interface{ ~int | ~float64 }

func Sum[T Number](xs ...T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return s
}

func Pick[T any](x, y T) T { return x }

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

type Box[T any] struct{ v T }

func (b Box[T]) Get() T { return b.v }

func (b *Box[T]) Set(v T) { b.v = v }

func NewBox[T any](v T) *Box[T] { return &Box[T]{v: v} }

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

func (p Pair[K, V]) Swap() Pair[K, V] { return p }

type Named interface{ Name() string }

func NameAll[T Named](xs []T) string {
	return strings.Join(Map(xs, func(x T) string { return x.Name() }), ",")
}

type List[T any] struct{ items []T }

func (l *List[T]) Push(x T) { l.items = append(l.items, x) }

func (l *List[T]) Len() int { return len(l.items) }
`
	files["lib/a.gp"] += `
func First[T any](xs []T) T {
	var zero T
	_ = zero // GEDIT
	if len(xs) == 0 {
		return zero
	}
	return xs[0]
}

func (l *List[T]) Last() T {
	var zero T
	_ = zero // MEDIT
	if len(l.items) == 0 {
		return zero
	}
	return l.items[len(l.items)-1]
}
`
	return files
}

// genericEdits are the statements that the matrix puts at a marker: "c.n += k // EDIT" in the
// method Add of a.gp (ordinary), "_ = zero // GEDIT" in First and "_ = zero // MEDIT" in Last.
// decline is set for the ones that must decline (an edge of weight 1).
var genericEdits = []struct {
	stmt    string
	markers string // the markers it goes to: e (EDIT), g (GEDIT), m (MEDIT)
	decline bool
}{
	{stmt: "_ = Sum(1, 2, 3)", markers: "egm"},
	{stmt: "_ = Sum[float64](1, 2)", markers: "egm"},
	{stmt: "_ = Sum(\"a\")", markers: "egm"},
	{stmt: "_ = Pick(1, \"a\")", markers: "egm"},
	{stmt: "_ = Pick[int, int](1, 2)", markers: "egm"},
	{stmt: "b := NewBox(1)\n\t_ = b.Get()", markers: "egm"},
	{stmt: "var b Box[string]\n\tb.Set(1)", markers: "egm"},
	{stmt: "p := Pair[string, int]{Key: \"a\"}\n\t_ = p.Swap().Val", markers: "egm"},
	{stmt: "var p Pair[[]int, int]\n\t_ = p", markers: "egm"},
	{stmt: "_ = Map([]int{1}, func(i int) string { return \"\" })", markers: "egm"},
	{stmt: "_ = NameAll([]int{1})", markers: "egm"},
	{stmt: "var l List[int]\n\tl.Push(\"x\")", markers: "egm"},
	{stmt: "_ = Pick", markers: "egm"},
	{stmt: "_ = Box{}", markers: "egm"},
	{stmt: "_ = Missing[int]", markers: "egm"},
	{stmt: "var n Number\n\t_ = n", markers: "egm"},
	{stmt: "_ = First([]string{\"a\"})", markers: "egm"},
	{stmt: "var l List[int]\n\t_ = l.Last() + l.Len()", markers: "egm"},
	{stmt: "type local struct{ a int }\n\t_ = Pick(local{}, local{})", markers: "e"},
	{stmt: "x := 1", markers: "egm"},
	// With the type parameter of the generic function or method: weight 0.
	{stmt: "_ = Pick[T](zero, zero)", markers: "gm"},
	{stmt: "_ = Pick(zero, zero)", markers: "gm"},
	{stmt: "var b Box[T]\n\tb.Set(zero)", markers: "gm"},
	{stmt: "_ = NewBox(zero).Get()", markers: "gm"},
	{stmt: "_ = Map([]T{zero}, func(x T) T { return x })", markers: "gm"},
	{stmt: "_ = First([]T{zero})", markers: "gm"},
	{stmt: "_ = Sum(zero)", markers: "gm"},
	{stmt: "_ = zero + 1", markers: "gm"},
	{stmt: "_ = l.Len()", markers: "m"},
	{stmt: "l.Push(zero)", markers: "m"},
	{stmt: "l.Push(1)", markers: "m"},
	// Weight 1: the type argument is made of a type parameter, or of a type declared in a generic
	// function. Some make a cycle (First[[]T] in First), which go/types reports.
	{stmt: "_ = Pick[[]T](nil, nil)", markers: "gm", decline: true},
	{stmt: "var b Box[*T]\n\t_ = b", markers: "gm", decline: true},
	{stmt: "_ = Map([]T{zero}, func(x T) []T { return nil })", markers: "gm", decline: true},
	{stmt: "_ = First[[]T](nil)", markers: "gm", decline: true},
	{stmt: "var l2 List[Pair[int, T]]\n\t_ = l2", markers: "gm", decline: true},
	{stmt: "type local struct{ a int }\n\t_ = Pick(local{}, local{})", markers: "gm", decline: true},
}

var genericMarkers = map[byte]string{
	'e': "c.n += k // EDIT",
	'g': "_ = zero // GEDIT",
	'm': "_ = zero // MEDIT",
}

func TestFocusedGenerics(t *testing.T) {
	files := genericFiles()
	for _, phase := range focusPhases {
		answered, edits := 0, 0
		for _, tt := range genericEdits {
			for i := range len(tt.markers) {
				marker := genericMarkers[tt.markers[i]]
				edits++
				t.Run(phaseName(phase)+"/"+string(tt.markers[i])+"/"+tt.stmt, func(t *testing.T) {
					w, base := focusRunAt(t, files, phase)
					w.open("lib/g.gp", files["lib/g.gp"])
					base = w.pkg("lib")
					if !base.focusEligible() {
						t.Fatal("the base is not eligible")
					}
					for _, d := range base.Check.Errors.Diagnostics() {
						if !d.Gate {
							t.Fatalf("the base has an error (go/types checks instantiation cycles only without): %+v", d)
						}
					}
					// The marker stays, so that the line before it is new text.
					text := strings.Replace(files["lib/a.gp"], marker, tt.stmt+"\n\t"+marker, 1)

					focused, full := focusedAndFull(t, w, base, "lib", "lib/a.gp", text)

					if tt.decline {
						if focused != nil {
							t.Fatal("a focused check of an instantiation with an edge of weight 1")
						}
						return
					}
					if focused == nil {
						t.Fatal("the focused check declined")
					}
					answered++
					path := filepath.Join(w.root, "lib", "a.gp")
					if got, want := diagsIn(focused, path), diagsIn(full, path); !reflect.DeepEqual(got, want) {
						t.Errorf("edited file:\nfocused %+v\nfull    %+v", got, want)
					}
					for _, other := range []string{"b.gp", "c.gp", "g.gp"} {
						file := filepath.Join(w.root, "lib", other)
						if got, want := diagsIn(full, file), diagsIn(base, file); !reflect.DeepEqual(got, want) {
							t.Errorf("diagnostics of %s changed:\nbase %+v\nfull %+v", other, want, got)
						}
					}
				})
			}
		}
		t.Logf("%s: %d of %d edits answered", phaseName(phase), answered, edits)
	}
}

// TestFocusedGenericCycle: the cycle that an edit of a generic function's body makes is reported by
// the full check, at the edited file, and the focused check declines.
func TestFocusedGenericCycle(t *testing.T) {
	files := genericFiles()
	w, base := focusRun(t, files)
	w.open("lib/g.gp", files["lib/g.gp"])
	base = w.pkg("lib")
	text := strings.Replace(files["lib/a.gp"], "_ = zero // GEDIT", "_ = First[[]T](nil) // GEDIT", 1)
	focused, full := focusedAndFull(t, w, base, "lib", "lib/a.gp", text)
	path := filepath.Join(w.root, "lib", "a.gp")
	found := false
	for _, d := range diagsIn(full, path) {
		found = found || strings.Contains(d.Msg, "instantiation cycle")
	}
	if !found {
		t.Fatalf("the full check reports no instantiation cycle: %+v", full.Check.Errors.Diagnostics())
	}
	if focused != nil {
		t.Errorf("a focused check of the edit that makes the cycle:\nfocused %+v\nfull    %+v",
			diagsIn(focused, path), diagsIn(full, path))
	}
}

// TestFocusedGenericFile: edits in the bodies of the file of the generic declarations (g.gp).
func TestFocusedGenericFile(t *testing.T) {
	files := genericFiles()
	edits := []struct {
		old, new string
		decline  bool
	}{
		{old: "return b.v }", new: "return b.v + 1 }"},
		{old: "\tvar s T\n", new: "\tvar s T\n\ts = \"x\"\n"},
		{old: "\tvar s T\n", new: "\tvar s T\n\tunused := s\n"},
		{old: "func Pick[T any](x, y T) T { return x }", new: "func Pick[T any](x, y T) T { return y }"},
		{old: "l.items = append(l.items, x) }", new: "l.items = append(l.items, x, x) }"},
		{old: "l.items = append(l.items, x) }", new: "l.items = append(l.items, 1) }"},
		{old: "return len(l.items) }", new: "return len(l.items) + First(l.items) }"},
		{old: "return strings.Join(", new: "_ = Pick[[]T](nil, nil)\n\treturn strings.Join(", decline: true},
		// A signature or a constraint is not a body.
		{old: "func Pick[T any](x, y T) T", new: "func Pick[T comparable](x, y T) T", decline: true},
		{old: "type Box[T any] struct{ v T }", new: "type Box[T comparable] struct{ v T }", decline: true},
		{old: "func (b Box[T]) Get() T {", new: "func (b Box[T]) Get() (T, bool) {", decline: true},
	}
	for _, phase := range focusPhases {
		for _, tt := range edits {
			t.Run(phaseName(phase)+"/"+tt.new, func(t *testing.T) {
				w, base := focusRunAt(t, files, phase)
				w.open("lib/g.gp", files["lib/g.gp"])
				base = w.pkg("lib")
				if !strings.Contains(files["lib/g.gp"], tt.old) {
					t.Fatalf("g.gp has no %q", tt.old)
				}
				text := strings.Replace(files["lib/g.gp"], tt.old, tt.new, 1)

				focused, full := focusedAndFull(t, w, base, "lib", "lib/g.gp", text)

				if tt.decline {
					if focused != nil {
						t.Fatal("a focused check")
					}
					return
				}
				if focused == nil {
					t.Fatal("the focused check declined")
				}
				path := filepath.Join(w.root, "lib", "g.gp")
				if got, want := diagsIn(focused, path), diagsIn(full, path); !reflect.DeepEqual(got, want) {
					t.Errorf("edited file:\nfocused %+v\nfull    %+v", got, want)
				}
				for _, other := range []string{"a.gp", "b.gp", "c.gp"} {
					file := filepath.Join(w.root, "lib", other)
					if got, want := diagsIn(full, file), diagsIn(base, file); !reflect.DeepEqual(got, want) {
						t.Errorf("diagnostics of %s changed:\nbase %+v\nfull %+v", other, want, got)
					}
				}
			})
		}
	}
}

// TestFocusedErrorOrder: go/types leaves out a later error whose message has "invalid operand" or
// "invalid type" after its start once it has reported an error (errors.go, report). The focused
// check declines when the errors before the edited file's could differ from the full check's:
// an error in a body of another file that it leaves out (case 1), and an error of another file
// with that text, which a new first error would make the full check leave out (case 2). Each case
// in a plain package and in one where the body is a method of a generic type; case 1 also with
// errors.Trace, which go/types reports as undefined: at the product's phase a gate error takes the
// place of that error (diag.Diagnostic.Replaces), at the last phase it stays.
func TestFocusedErrorOrder(t *testing.T) {
	const ret = "return \"invalid type\""
	tests := []struct {
		name          string
		files         map[string]string
		edit          string // the file that the test edits
		old, new      string
		otherChanges  bool // the full check changes the diagnostics of the other file
		editedDropped bool // the full check leaves out the edited file's error
	}{
		{
			name: "case 1, plain",
			files: map[string]string{
				"lib/a.gp": "package lib\n\nfunc A() {\n\tunusedA := 1\n}\n",
				"lib/b.gp": "package lib\n\nfunc B() error {\n\treturn nil\n}\n",
			},
			edit: "lib/b.gp", old: "return nil", new: ret, editedDropped: true,
		},
		{
			name: "case 1, a method of a generic type",
			files: map[string]string{
				"lib/a.gp": "package lib\n\ntype Box[T any] struct{ v T }\n\nfunc (b Box[T]) Get() T {\n\tunusedA := 1\n\treturn b.v\n}\n",
				"lib/b.gp": "package lib\n\nfunc B() error {\n\treturn nil\n}\n",
			},
			edit: "lib/b.gp", old: "return nil", new: ret, editedDropped: true,
		},
		{
			name: "case 1, a gate error in the place of an error of go/types",
			files: map[string]string{
				"lib/a.gp": "package lib\n\nimport \"errors\"\n\nfunc A() {\n\t_ = errors.Trace\n}\n",
				"lib/b.gp": "package lib\n\nfunc B() error {\n\treturn nil\n}\n",
			},
			edit: "lib/b.gp", old: "return nil", new: ret, editedDropped: true,
		},
		{
			name: "case 2, plain",
			files: map[string]string{
				"lib/a.gp": "package lib\n\nfunc A() {\n\t_ = 1\n}\n",
				"lib/b.gp": "package lib\n\nfunc B() error {\n\t" + ret + "\n}\n",
			},
			edit: "lib/a.gp", old: "_ = 1", new: "unusedA := 1", otherChanges: true,
		},
		{
			name: "case 2, a method of a generic type",
			files: map[string]string{
				"lib/a.gp": "package lib\n\ntype Box[T any] struct{ v T }\n\nfunc (b Box[T]) Get() T {\n\t_ = 1\n\treturn b.v\n}\n",
				"lib/b.gp": "package lib\n\ntype Pair[T any] struct{ v T }\n\nfunc (p Pair[T]) B() error {\n\t" + ret + "\n}\n",
			},
			edit: "lib/a.gp", old: "_ = 1", new: "unusedA := 1", otherChanges: true,
		},
	}
	for _, phase := range focusPhases {
		for _, tt := range tests {
			t.Run(phaseName(phase)+"/"+tt.name, func(t *testing.T) {
				files := map[string]string{"gocpp.mod": "module example.com/focus\n"}
				for name, text := range tt.files {
					files[name] = text
				}
				w, base := focusRunAt(t, files, phase)
				if !base.focusEligible() {
					t.Fatal("the base is not eligible")
				}
				text := strings.Replace(files[tt.edit], tt.old, tt.new, 1)
				focused, full := focusedAndFull(t, w, base, "lib", tt.edit, text)
				other := "lib/a.gp"
				if tt.edit == other {
					other = "lib/b.gp"
				}
				edited := filepath.Join(w.root, filepath.FromSlash(tt.edit))
				otherPath := filepath.Join(w.root, filepath.FromSlash(other))
				// The case is real: the full check shows what a focused answer would get wrong.
				if tt.otherChanges && reflect.DeepEqual(diagsIn(full, otherPath), diagsIn(base, otherPath)) {
					t.Fatalf("the full check leaves %s as it was: %+v", other, diagsIn(full, otherPath))
				}
				if tt.editedDropped {
					for _, d := range diagsIn(full, edited) {
						if strings.Contains(d.Msg, "cannot use") {
							t.Fatalf("the full check reports %q", d.Msg)
						}
					}
				}
				if focused != nil {
					t.Errorf("a focused check:\nfocused %+v\nfull    %+v\nother: base %+v, full %+v",
						diagsIn(focused, edited), diagsIn(full, edited), diagsIn(base, otherPath), diagsIn(full, otherPath))
				}
			})
		}
	}
}
