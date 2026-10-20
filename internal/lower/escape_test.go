package lower

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// escapeSrc is the program of TestEscapeAnalysis. A line that ends with "// esc: a; b" must have
// exactly the notes a and b of the escape analysis (the diagnostics of gocpp -gp:m, in the order
// of their columns), and a line without that comment must have none. The functions named
// stays_* keep their object on the stack; the ones named heap_* must not: an object that escapes
// but stays on the stack is a dangling pointer, the failure that matters most.
const escapeSrc = `package main

type Counter struct {
	n    int
	hist [4]int
}

func (c *Counter) Inc()      { c.n++ }
func (c *Counter) Self() *Counter { return c }
func (c Counter) Total() int { return c.n }
func bump(c *Counter)        { c.n += 10 }
func keep(c *Counter) *Counter { return c }
func swap(a, b *Counter)     { a.n, b.n = b.n, a.n }

type Sink struct{ c *Counter }

var global *Counter
var sink Sink

type Ifc interface{ Inc() }

func useIfc(i Ifc) { i.Inc() }

// --- stays on the stack ---

func stays_method() int {
	var c Counter
	c.Inc() // esc: &c does not escape
	return c.n
}

func stays_borrowed_arg() int {
	var c Counter
	bump(&c) // esc: &c does not escape
	return c.n
}

func stays_pointer_variable() int {
	var c Counter
	p := &c // esc: &c does not escape
	p.Inc()
	p.n += 2
	return c.n + p.n
}

func stays_new() int {
	p := new(Counter) // esc: new(Counter) does not escape
	p.Inc()
	return p.n
}

func stays_literal() int {
	p := &Counter{n: 3} // esc: &Counter{...} does not escape
	bump(p)
	return p.n
}

func stays_temporary_argument() {
	bump(&Counter{n: 1}) // esc: &Counter{...} does not escape
}

func stays_field_address() int {
	var c Counter
	setInt(&c.n, 4) // esc: &c.n does not escape
	return c.n
}

func setInt(p *int, v int) { *p = v }

func stays_two_objects() int {
	var a, b Counter
	swap(&a, &b) // esc: &a does not escape; &b does not escape
	return a.n + b.n
}

func stays_in_loop() int {
	total := 0
	for i := 0; i < 3; i++ {
		var c Counter
		c.Inc() // esc: &c does not escape
		total += c.n
	}
	return total
}

func stays_compare() bool {
	var a, b Counter
	p := &a // esc: &a does not escape
	q := &b // esc: &b does not escape
	return p == q
}

func stays_reborrow() int {
	var c Counter
	p := &c // esc: &c does not escape
	bump(p)
	return p.Total()
}

// A C++ stack does not grow: objects are small, and a function keeps a few only.
type Mid struct{ data [200]int }  // 1600 bytes: kept, ten of them in a function
type Huge struct{ data [300]int } // 2400 bytes: never kept

func fillMid(m *Mid)   { m.data[0]++ }
func fillHuge(h *Huge) { h.data[0]++ }

func huge_object() int {
	var h Huge
	fillHuge(&h)
	return h.data[0]
}

func huge_new() int {
	h := &Huge{}
	fillHuge(h)
	return h.data[0]
}

func budget() int {
	var m1 Mid
	fillMid(&m1) // esc: &m1 does not escape
	var m2 Mid
	fillMid(&m2) // esc: &m2 does not escape
	var m3 Mid
	fillMid(&m3) // esc: &m3 does not escape
	var m4 Mid
	fillMid(&m4) // esc: &m4 does not escape
	var m5 Mid
	fillMid(&m5) // esc: &m5 does not escape
	var m6 Mid
	fillMid(&m6) // esc: &m6 does not escape
	var m7 Mid
	fillMid(&m7) // esc: &m7 does not escape
	var m8 Mid
	fillMid(&m8) // esc: &m8 does not escape
	var m9 Mid
	fillMid(&m9) // esc: &m9 does not escape
	var m10 Mid
	fillMid(&m10) // esc: &m10 does not escape
	var m11 Mid // esc: moved to heap: m11
	fillMid(&m11) // esc: &m11 escapes to heap
	return m1.data[0] + m11.data[0]
}

// --- escapes ---

func heap_returned() *Counter {
	var c Counter // esc: moved to heap: c
	return &c // esc: &c escapes to heap
}

func heap_returned_literal() *Counter {
	return &Counter{n: 1} // esc: &Counter{...} escapes to heap
}

func heap_returned_new() *Counter {
	return new(Counter) // esc: new(Counter) escapes to heap
}

func heap_returned_variable() *Counter {
	p := &Counter{} // esc: &Counter{} escapes to heap
	p.Inc()
	return p
}

func heap_global() {
	var c Counter // esc: moved to heap: c
	global = &c // esc: &c escapes to heap
}

func heap_field() {
	var c Counter // esc: moved to heap: c
	sink.c = &c // esc: &c escapes to heap
}

func heap_stored_variable() {
	p := &Counter{} // esc: &Counter{} escapes to heap
	global = p
}

func heap_copied_variable() *Counter {
	p := &Counter{} // esc: &Counter{} escapes to heap
	q := p
	return q
}

func heap_leaking_parameter() *Counter {
	var c Counter // esc: moved to heap: c
	return keep(&c) // esc: &c escapes to heap
}

func heap_receiver_returns_it() *Counter {
	var c Counter // esc: moved to heap: c
	return c.Self() // esc: &c escapes to heap
}

func heap_closure() func() int {
	var c Counter // esc: moved to heap: c
	p := &c // esc: &c escapes to heap
	return func() int { return p.n } // esc: func literal escapes to heap
}

func heap_captured_address() int {
	var c Counter // esc: moved to heap: c
	f := func() { c.Inc() } // esc: func literal does not escape; &c escapes to heap
	f()
	return c.n
}

// A variable that an escaping closure captures is boxed and shared by the box: no notes, and the
// address of it must not be a stack pointer (the closure would copy the variable).
func heap_captured_read_only() func() int {
	var c Counter
	get := func() int { return c.n } // esc: func literal escapes to heap
	c.Inc()
	return get
}

func heap_go() {
	var c Counter // esc: moved to heap: c
	go bump(&c) // esc: &c escapes to heap
}

func heap_defer() {
	var c Counter // esc: moved to heap: c
	defer bump(&c) // esc: &c escapes to heap
}

func heap_channel(ch chan *Counter) {
	var c Counter // esc: moved to heap: c
	ch <- &c // esc: &c escapes to heap
}

func heap_interface() {
	var c Counter // esc: moved to heap: c
	useIfc(&c) // esc: &c escapes to heap
}

func heap_interface_variable() Ifc {
	p := &Counter{} // esc: &Counter{} escapes to heap
	var i Ifc = p
	return i
}

func heap_reassigned_pointer() int {
	var a, b Counter // esc: moved to heap: a; moved to heap: b
	p := &a // esc: &a escapes to heap
	p = &b // esc: &b escapes to heap
	return p.n
}

func heap_slice_literal() []*Counter {
	var c Counter // esc: moved to heap: c
	return []*Counter{&c} // esc: &c escapes to heap
}

func heap_func_value(f func(*Counter)) {
	var c Counter // esc: moved to heap: c
	f(&c) // esc: &c escapes to heap
}

func heap_shadowed_allocation() int {
	p := &Counter{n: 2} // esc: &Counter{...} does not escape
	{
		p := &Counter{n: p.n + 1} // esc: &Counter{...} escapes to heap
		p.Inc()
		return p.n
	}
}

func heap_new_argument() {
	bump(new(Counter)) // esc: new(Counter) escapes to heap
}

// --- not considered: boxed for another reason (noteEscape reports them) ---

func boxed_sliced_array() int {
	var a [4]int
	s := a[:]
	setInt(&a[1], 2)
	return s[1]
}

func boxed_captured_variable() func() int {
	var c Counter
	f := func() int { c.n++; return c.n } // esc: func literal escapes to heap
	c.Inc()
	return f
}

func stays_captured_by_local_closure() int {
	var c Counter
	f := func() { c.n++ } // esc: func literal does not escape
	c.Inc() // esc: &c does not escape
	f()
	return c.n
}

func heap_pointer_to_pointer() **Counter {
	p := &Counter{} // esc: moved to heap: p; &Counter{} escapes to heap
	return &p // esc: &p escapes to heap
}
`

func TestEscapeAnalysis(t *testing.T) {
	gp := filepath.Join(t.TempDir(), "main.gp")
	if err := os.WriteFile(gp, []byte(escapeSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	fset, pkg := checkFile(t, gp)
	g := newGen(fset, pkg)
	got := make(map[int][]string) // by line
	for _, d := range pkg.Load.Files[0].AST.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		for _, n := range g.escapeOf(fd).sorted() {
			line := fset.Position(n.pos).Line
			got[line] = append(got[line], n.msg)
		}
	}
	lines := strings.Split(escapeSrc, "\n")
	for i, text := range lines {
		_, comment, marked := strings.Cut(text, "// esc: ")
		var want []string
		if marked {
			want = strings.Split(comment, "; ")
		}
		if g, w := strings.Join(got[i+1], "; "), strings.Join(want, "; "); g != w {
			t.Errorf("line %d (%s): notes %q, want %q", i+1, strings.TrimSpace(text), g, w)
		}
	}
}

func TestEscapeSmallHolderBudget(t *testing.T) {
	// A byte costs 24 charged bytes: its value, a 16-byte gp::ptr base and padding.
	// Charging only the byte would keep all these holders and exceed the storage budget.
	const cost = 24
	const kept = maxStackObjectsSum / cost
	if got := stackObjectCost(types.Typ[types.Byte]); got != cost {
		t.Fatalf("byte holder costs %d bytes, want %d", got, cost)
	}
	var src strings.Builder
	src.WriteString("package main\nfunc work() {\n")
	for i := range kept + 1 {
		fmt.Fprintf(&src, "p%d := new(byte)\n*p%d = 1\n", i, i)
	}
	src.WriteString("}\n")
	gp := filepath.Join(t.TempDir(), "main.gp")
	if err := os.WriteFile(gp, []byte(src.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	fset, pkg := checkFile(t, gp)
	g := newGen(fset, pkg)
	work := pkg.Load.Files[0].AST.Decls[0].(*ast.FuncDecl)
	esc := g.escapeOf(work)
	if len(esc.holders) != kept {
		t.Errorf("kept %d byte holders, want %d", len(esc.holders), kept)
	}
}

// TestLocalClosureStackBudget checks storage newly moved out of boxes, including captures.
func TestLocalClosureStackBudget(t *testing.T) {
	for _, tt := range []struct {
		name    string
		size    int
		count   int
		changed bool
		kept    int
	}{
		{name: "large changed capture", size: 10000, count: 1, changed: true},
		{name: "large read-only capture", size: 10000, count: 1},
		{name: "captures share budget with addresses", size: 200, count: 11, changed: true, kept: 10},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var src strings.Builder
			fmt.Fprintf(&src, "package main\ntype Blob struct { data [%d]int }\n", tt.size)
			src.WriteString("func touch(p *Blob) { p.data[0]++ }\nfunc work() int {\n")
			for i := range tt.count {
				fmt.Fprintf(&src, "var c%d Blob\n", i)
				if tt.changed {
					fmt.Fprintf(&src, "func() { c%d.data[0]++ }()\n", i)
				} else {
					fmt.Fprintf(&src, "func() int { return c%d.data[0] }()\n", i)
				}
			}
			src.WriteString("var addressed Blob\ntouch(&addressed)\nreturn addressed.data[0]\n}\n")
			gp := filepath.Join(t.TempDir(), "main.gp")
			if err := os.WriteFile(gp, []byte(src.String()), 0o644); err != nil {
				t.Fatal(err)
			}
			fset, pkg := checkFile(t, gp)
			g := newGen(fset, pkg)
			var work *ast.FuncDecl
			for _, decl := range pkg.Load.Files[0].AST.Decls {
				if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "work" {
					work = fd
				}
			}
			if work == nil {
				t.Fatal("work was not parsed")
			}
			bi := g.boxInfoOf(work)
			esc := g.escapeOf(work)
			checked := 0
			for id, obj := range g.info.Defs {
				v, ok := obj.(*types.Var)
				if !ok || !strings.HasPrefix(id.Name, "c") {
					continue
				}
				var index int
				if _, err := fmt.Sscanf(id.Name, "c%d", &index); err != nil {
					continue
				}
				checked++
				boxed := bi.first[v] != 0 && bi.first[v] != boxRefCaptured
				if boxed != (index >= tt.kept) {
					t.Errorf("%s: reason %s, want boxed %v", id.Name, bi.first[v], index >= tt.kept)
				}
			}
			if checked != tt.count {
				t.Fatalf("checked %d captures, want %d", checked, tt.count)
			}
			if bi.stackBytes > maxStackObjectsSum {
				t.Errorf("captures reserve %d bytes, budget %d", bi.stackBytes, maxStackObjectsSum)
			}
			if tt.kept > 0 {
				for v := range esc.stack {
					if v.Name() == "addressed" {
						t.Error("addressed object ignores the captures' reservation")
					}
				}
			}
		})
	}
}

// elemRefSrc is the program of TestElemRefs. A line that declares an element variable ends with
// "// ref" when the variable must borrow the element, or with "// copy: reason" when it must be
// a copy for that reason (a substring of the reason that -gp:m=2 prints). The negative cases are
// the ones that matter: a reference where the slot can change or be freed is a dangling one.
// (Every method name is the key of the class hierarchy analysis: Size is only pure methods, Hurt
// has an implementation that writes the slice.)
const elemRefSrc = `package main

type Pure interface{ Size() int }

type P1 interface{ Size() int }
type P2 interface{ Size() int }
type P3 interface{ Size() int }
type P4 interface{ Size() int }
type P5 interface{ Size() int }
type P6 interface{ Size() int }
type P7 interface{ Size() int }
type P8 interface{ Size() int }
type P9 interface{ Size() int }
type P10 interface{ Size() int }
type P11 interface{ Size() int }
type P12 interface{ Size() int }

type P13 interface{ Size() int }
type P14 interface{ Size() int }

type Box struct{ n int }

func (b *Box) Size() int { return b.n }

type Pair struct{ a, b int }

func (p Pair) Size() int { return p.a + p.b }

type Evil interface{ Hurt() int }

type Hurter struct{ n int }

var evils []Evil

// Hurt frees the array that the element of evils lives in.
func (h *Hurter) Hurt() int {
	evils = nil
	return h.n
}

type Node struct {
	val  int
	next *Node
}

type Reg struct{ items []P2 }

func (r *Reg) wipe() { r.items[0] = nil }

type Reg4 struct{ items []P4 }

type Reg5 struct{ items []P5 }

var grid [3]int

// --- borrowed ---

func ref_range_interface(xs []Pure) int {
	sum := 0
	for _, x := range xs { // ref
		sum += x.Size()
	}
	return sum
}

func ref_index_interface(xs []Pure, i int) int {
	x := xs[i] // ref
	return x.Size()
}

func ref_range_string(xs []string) int {
	n := 0
	for _, s := range xs { // ref
		n += len(s)
	}
	return n
}

func ref_range_pointer(ns []*Node) int {
	sum := 0
	for _, n := range ns { // ref
		sum += n.val
	}
	return sum
}

func ref_index_pointer(ns []*Node) int {
	n := ns[0] // ref
	sum := 0
	for m := n; m != nil; m = m.next {
		sum += m.val
	}
	return sum
}

func ref_index_array_index(xs []Pure, k int) int {
	x := xs[grid[k&1]] // ref
	return x.Size()
}

func ref_field_slice(r *Reg5) int {
	x := r.items[0] // ref
	return x.Size()
}

func ref_pure_callee(xs []Pure) int {
	sum := 0
	for _, x := range xs { // ref
		sum += double(x.Size())
	}
	return sum
}

func double(n int) int { return 2 * n }

// --- copied ---

func copy_slot_overwritten(xs []P1) int {
	sum := 0
	for i, x := range xs { // copy: may store an owner
		xs[i] = nil
		sum += x.Size()
	}
	return sum
}

func copy_method_overwrites(r *Reg) int {
	x := r.items[0] // copy: may store an owner
	r.wipe()
	return x.Size()
}

func copy_root_reassigned(xs []P3) int {
	x := xs[0] // copy: assigns xs
	xs = nil
	return x.Size()
}

func copy_field_reassigned(r *Reg4) int {
	x := r.items[0] // copy: assigns r
	r.items = nil
	return x.Size()
}

func copy_unknown_call(xs []P5, f func()) int {
	x := xs[0] // copy: may store an owner
	f()
	return x.Size()
}

func copy_impure_interface_call() int {
	x := evils[0] // copy: may store an owner
	return x.Hurt()
}

func copy_returned(xs []P6) P6 {
	x := xs[0] // copy: it is returned
	return x
}

func copy_assigned(xs []P7) int {
	x := xs[0] // copy: it is assigned
	x = xs[1]
	return x.Size()
}

func copy_address_taken(xs []P8) *P8 {
	x := xs[0] // copy: it is assigned or its address is taken
	return &x
}

func copy_captured(xs []P9) func() int {
	x := xs[0] // copy: boxed or captured
	return func() int { return x.Size() }
}

func copy_goroutine(xs []P10) int {
	x := xs[0] // copy: may store an owner
	go double(1)
	return x.Size()
}

func copy_array_alias() int {
	var arr [2]P11
	arr[0] = &Box{1}
	xs := arr[:]
	x := xs[0] // copy: array of owners
	arr[0] = nil
	return x.Size()
}

func copy_struct_element(ps []Pair) int {
	p := ps[0] // copy: type is copied
	return p.a
}

func copy_append_in_region(xs []P12, out []P12) []P12 {
	for _, x := range xs { // copy: may store an owner
		out = append(out, x)
	}
	return out
}

// Another function stores elements of type P13 and P14: a second thread could do it at any time.
func store_elsewhere(a []P13, b []P14) {
	a[0] = nil
	b[0] = nil
}

func copy_stored_elsewhere_index(xs []P13) int {
	x := xs[0] // hot: another thread
	return x.Size()
}

func copy_stored_elsewhere_range(xs []P14) int {
	sum := 0
	for _, x := range xs { // hot: another thread
		sum += x.Size()
	}
	return sum
}

func copy_call_in_index(xs []Pure, i int) int {
	x := xs[double(i)] // copy: found by a call
	return x.Size()
}

func copy_map_value(m map[int]Pure) int {
	sum := 0
	for _, x := range m {
		sum += x.Size()
	}
	return sum
}
`

// TestElemRefs checks the element variables of elemRefSrc in a program with a second thread (the
// result of the whole-program thread analysis) and in one without: the second thread is a reason
// by itself, and it hides the others when it applies.
func TestElemRefs(t *testing.T) {
	for _, threaded := range []bool{true, false} {
		name := "threaded"
		if !threaded {
			name = "single thread"
		}
		t.Run(name, func(t *testing.T) { checkElemRefs(t, elemRefSrc, threaded) })
	}
}

// elemRefStringSrc is the strings of TestElemRefsStrings. A string is not an owner for the purity
// of ADR-086 (a store of one into a local variable is private), but a slice of a local array
// shares its slots with the reference, so a store into such an array is a store under the
// reference. (They are in a program of their own: the stores make string elements "stored" for
// the threaded program, which the other cases of strings must not see.)
const elemRefStringSrc = `package main

type Name string
type Record struct{ text string }
type Plain struct{ number int }

func copy_named_string_array_alias() int {
	arr := [2]Name{"a", "b"}
	xs := arr[:]
	x := xs[0] // copy: its type is copied
	arr[0] = "c"
	return len(x)
}

func copy_record_array_alias() int {
	arr := [2]Record{{"a"}, {"b"}}
	xs := arr[:]
	x := xs[0] // copy: its type is copied
	arr[0].text = "c"
	return len(x.text)
}

func copy_plain_array_alias() int {
	arr := [2]Plain{{1}, {2}}
	xs := arr[:]
	x := xs[0] // copy: its type is copied
	arr[0].number = 3
	return x.number
}

func copy_string_array_alias() int {
	arr := [2]string{"a", "b"}
	xs := arr[:]
	x := xs[0] // copy: array of owners
	arr[0] = "c"
	return len(x)
}

func copy_string_nested_array_alias() int {
	var arr [2][2]string
	xs := arr[0][:]
	x := xs[1] // copy: array of owners
	arr[0][1] = "c"
	return len(x)
}

func copy_string_array_assigned() int {
	arr := [2]string{"a", "b"}
	xs := arr[:]
	x := xs[0] // copy: array of owners
	arr = [2]string{}
	return len(x)
}

func copy_string_array_in_struct() int {
	var w struct{ names [2]string }
	xs := w.names[:]
	x := xs[0] // copy: array of owners
	w.names[0] = "c"
	return len(x)
}

func copy_string_array_range() string {
	arr := [2]string{"a", "b"}
	out := ""
	for _, s := range arr[:] { // copy: array of owners
		arr[1] = "c"
		out += s
	}
	return out
}

func copy_string_array_compound() int {
	arr := [2]string{"a", "b"}
	xs := arr[:]
	x := xs[1] // copy: array of owners
	arr[1] += "!"
	return len(x)
}

func copy_string_slice_element_store(xs, ys []string) int {
	x := xs[0] // copy: may store an owner
	ys[0] = "c"
	return len(x)
}

// The program can run a second thread.
func spawn() {
	go spawn()
}

func stays_string_read(xs []string) int {
	n := 0
	for _, s := range xs { // hot: another thread
		n += len(s)
	}
	return n
}
`

func TestElemRefsStrings(t *testing.T) {
	for _, threaded := range []bool{true, false} {
		name := "threaded"
		if !threaded {
			name = "single thread"
		}
		t.Run(name, func(t *testing.T) { checkElemRefs(t, elemRefStringSrc, threaded) })
	}
}

func checkElemRefs(t *testing.T, src string, threaded bool) {
	gp := filepath.Join(t.TempDir(), "main.gp")
	if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	fset, pkg := checkFile(t, gp)
	g := newGen(fset, pkg)
	if !threaded {
		g.thr = &threadInfo{
			mutGlobals: make(map[*types.Var]bool),
			mutFields:  make(map[*types.Var]bool),
			mutElems:   make(map[string]bool),
		}
	}
	refs := make(map[int]bool)     // the lines of the variables that borrow
	copies := make(map[int]string) // the lines of the variables that are copies: the reasons
	for _, d := range pkg.Load.Files[0].AST.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		e := g.escapeOf(fd)
		for v := range e.refs {
			refs[fset.Position(v.Pos()).Line] = true
		}
		for _, n := range e.notes {
			if n.only2 {
				line := fset.Position(n.pos).Line
				copies[line] += n.detail + "; "
			}
		}
	}
	for i, text := range strings.Split(src, "\n") {
		line := i + 1
		_, comment, marked := strings.Cut(text, "// ")
		short := strings.TrimSpace(text)
		if marked && strings.HasPrefix(comment, "hot: ") {
			// Copied for a second thread only: a reference without one.
			if threaded {
				comment = "copy: " + strings.TrimPrefix(comment, "hot: ")
			} else {
				comment = "ref"
			}
		}
		switch {
		case !marked:
			if refs[line] || copies[line] != "" {
				t.Errorf("line %d (%s): decided without a marker", line, short)
			}
		case comment == "ref":
			if !refs[line] {
				t.Errorf("line %d (%s): copies (%s), want a reference", line, short, copies[line])
			}
		case strings.HasPrefix(comment, "copy: "):
			want := strings.TrimPrefix(comment, "copy: ")
			if refs[line] {
				t.Errorf("line %d (%s): borrows, want a copy", line, short)
			} else if !strings.Contains(copies[line], want) {
				t.Errorf("line %d (%s): copies for %q, want %q", line, short, copies[line], want)
			}
		}
	}
}

// localClosureSrc is the program of TestLocalClosures. A line with "// lit: local" or
// "// lit: escapes" makes a func literal that must (not) be a local closure (localclosure.go), and
// a line with "// var x: boxed", "// var x: stack" or "// var x: plain" declares the variable x that
// must be in a box, kept on the stack as shared memory (boxRefCaptured: a local closure changes it), or
// an ordinary variable. A closure that escapes is a heap object that a variable it captured and
// changed must share, so the negative cases are the ones that matter.
const localClosureSrc = `package main

import "sync"

type Holder struct{ f func() int }

var saved func() int

func apply(f func() int) int { return f() }

func local_called() int {
	n := 0 // var n: stack
	inc := func() { n++ } // lit: local
	inc()
	inc()
	return n
}

func local_immediate() int {
	n := 0 // var n: stack
	func() { n += 2 }() // lit: local
	return n
}

func local_unchanged() int {
	k := 3 // var k: plain
	get := func() int { return k } // lit: local
	return get()
}

func local_two() int {
	a := 0 // var a: stack
	b := 0 // var b: stack
	f := func() { a++; b += 2 } // lit: local
	g := func() int { return a + b } // lit: local
	f()
	return g()
}

func local_nested() int {
	n := 0 // var n: stack
	outer := func() { // lit: local
		inner := func() { n++ } // lit: local
		inner()
	}
	outer()
	return n
}

func local_result_used() int {
	n := 1 // var n: stack
	x := func() int { n *= 2; return n }() // lit: local
	return x + n
}

func escapes_returned() func() int {
	n := 0 // var n: boxed
	return func() int { n++; return n } // lit: escapes
}

func escapes_stored() *Holder {
	n := 0 // var n: boxed
	inc := func() int { n++; return n } // lit: escapes
	return &Holder{f: inc}
}

func escapes_passed() int {
	n := 0 // var n: boxed
	inc := func() int { n++; return n } // lit: escapes
	return apply(inc)
}

func escapes_argument() int {
	n := 0 // var n: boxed
	return apply(func() int { n++; return n }) // lit: escapes
}

func escapes_global() {
	n := 0 // var n: boxed
	saved = func() int { n++; return n } // lit: escapes
}

func escapes_go() {
	n := 0 // var n: boxed
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { n++; wg.Done() }() // lit: escapes
	wg.Wait()
}

func escapes_defer() (r int) {
	n := 0 // var n: boxed
	defer func() { r = n }() // lit: escapes
	n = 5
	return
}

func escapes_defer_variable() int {
	n := 0 // var n: boxed
	inc := func() { n++ } // lit: escapes
	defer inc()
	return n
}

func escapes_in_closure() int {
	n := 0 // var n: boxed
	inc := func() { n++ } // lit: escapes
	twice := func() { inc(); inc() } // lit: local
	twice()
	return n
}

func escapes_compared() int {
	n := 0 // var n: boxed
	f := func() { n++ } // lit: escapes
	if f != nil {
		f()
	}
	return n
}

func escapes_reassigned() int {
	n := 0 // var n: boxed
	f := func() { n++ } // lit: escapes
	f()
	f = func() { n += 10 } // lit: escapes
	f()
	return n
}

func escapes_recursive() int {
	var fib func(int) int
	fib = func(k int) int { // lit: escapes
		if k < 2 {
			return k
		}
		return fib(k-1) + fib(k-2)
	}
	return fib(5)
}

func escapes_address() int {
	n := 0 // var n: boxed
	f := func() { n++ } // lit: escapes
	p := &f
	(*p)()
	return n
}

func escapes_recover() int {
	n := 0 // var n: boxed
	func() { // lit: escapes
		defer func() { recover(); n++ }() // lit: escapes
		panic("x")
	}()
	return n
}

func shared_with_escaping() int {
	n := 0 // var n: boxed
	local := func() { n++ } // lit: local
	saved = func() int { n += 10; return n } // lit: escapes
	local()
	return n
}

func seq(yield func(int) bool) {
	_ = yield(1) && yield(2)
}

func escapes_in_range_func_body() int {
	n := 0 // var n: boxed
	inc := func() { n++ } // lit: escapes
	for range seq {
		inc()
	}
	return n
}

func loop_variable() int {
	sum := 0
	for i := 0; i < 3; i++ { // var i: stack
		get := func() int { return i } // lit: local
		sum += get()
	}
	return sum
}

func range_variable() int {
	sum := 0
	for i, v := range []int{1, 2} { // var i: plain
		sum += func() int { return i + v }() // lit: local
		func() { v++ }() // lit: local
		sum += v
	}
	return sum
}

func loop_variable_escapes() int {
	var fs []func() int
	for i := 0; i < 3; i++ { // var i: boxed
		get := func() int { return i } // lit: local
		_ = get()
		fs = append(fs, func() int { return i }) // lit: escapes
	}
	return len(fs)
}

func loop_variable_go() int {
	n := 0
	for i := 0; i < 3; i++ { // var i: boxed
		get := func() int { return i } // lit: escapes
		go func() { n += get() }() // lit: escapes
	}
	return n
}

func loop_variable_range_func() int {
	n := 0
	for i := 0; i < 2; i++ { // var i: boxed
		for range seq {
			func() { n += i }() // lit: local
		}
	}
	return n
}

func range_func_variable() int {
	n := 0
	for x := range seq { // var x: boxed
		func() { x++; n += x }() // lit: local
	}
	return n
}

func switch_header_variable(k int) int {
	switch v := k * 2; { // var v: boxed
	case v > 2:
		func() { v++ }() // lit: local
		return v
	}
	return 0
}

func parameter_captured(p int) int { // var p: boxed
	inc := func() { p++ } // lit: local
	inc()
	return p
}
`

func TestLocalClosures(t *testing.T) {
	gp := filepath.Join(t.TempDir(), "main.gp")
	if err := os.WriteFile(gp, []byte(localClosureSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	fset, pkg := checkFile(t, gp)
	g := newGen(fset, pkg)
	lines := strings.Split(localClosureSrc, "\n")
	lits := make(map[int][]bool) // the func literals of each line: local or not
	reasons := make(map[string]boxReason)
	declared := make(map[string]bool)
	for _, d := range pkg.Load.Files[0].AST.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		g.escapeOf(fd)
		bi := g.boxInfoOf(fd)
		ast.Inspect(fd, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncLit:
				line := fset.Position(x.Pos()).Line
				_, local := g.localLits[x]
				lits[line] = append(lits[line], local)
			case *ast.Ident:
				if v, ok := pkg.Info.Defs[x].(*types.Var); ok {
					key := fmt.Sprintf("%d %s", fset.Position(x.Pos()).Line, v.Name())
					declared[key] = true
					reasons[key] = bi.first[v]
				}
			}
			return true
		})
	}
	for i, text := range lines {
		line := i + 1
		_, comment, marked := strings.Cut(text, "// ")
		switch {
		case marked && strings.HasPrefix(comment, "lit: "):
			want := strings.TrimPrefix(comment, "lit: ") == "local"
			if len(lits[line]) == 0 {
				t.Errorf("line %d: no func literal", line)
			}
			for _, local := range lits[line] {
				if local != want {
					t.Errorf("line %d (%s): local = %v, want %v", line, strings.TrimSpace(text), local, want)
				}
			}
		case marked && strings.HasPrefix(comment, "var "):
			name, kind, _ := strings.Cut(strings.TrimPrefix(comment, "var "), ": ")
			key := fmt.Sprintf("%d %s", line, name)
			if !declared[key] {
				t.Errorf("line %d: no variable %s", line, name)
				continue
			}
			got := "plain"
			switch r := reasons[key]; {
			case r == boxRefCaptured:
				got = "stack"
			case r != 0:
				got = "boxed"
			}
			if got != kind {
				t.Errorf("line %d (%s): %s is %s, want %s", line, strings.TrimSpace(text), name, got, kind)
			}
		}
	}
}
