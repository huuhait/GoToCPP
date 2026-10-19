package lower

import (
	"go/ast"
	"go/token"
	"sort"
	"sync"

	"github.com/huuhait/gocpp/internal/check"
)

// The reason log (Phase 13 step 13.5: gocpp emit -annotate, gocpp explain).
//
// Lowering makes many decisions that the C++ shows but does not say: why a variable is a
// gp::ptr box, why a call is stored in the defer frame, which conversion allocates, where the
// runtime checks. A Why records them, each as a Reason at a .gp place, while lowering runs:
// internal/explain turns the C++ and the reasons into the annotated view that gocpp emit
// -annotate prints.
//
// The log never changes the C++. Options.Why is nil unless the annotated view was asked for,
// and every hook is `g.why(pos, kind, msg)`, a method that returns at once for the nil log; the
// hooks that format their message first check g.noting(). A test (cmd/gocpp) compares the
// output of gocpp emit with and without a log byte for byte.

// A Kind says what a Reason is about. It is also the name of the gocpp explain topic that
// describes the mechanism (internal/explain/topics.go).
type Kind string

// The kinds of reasons. Each one is the name of an explain topic.
const (
	WhyBoxing     Kind = "boxing"         // a variable lives in a gp::ptr box on the heap
	WhyClosures   Kind = "closures"       // a func literal is a lambda; what it captures, and how
	WhyDefer      Kind = "defer"          // the defer frame and the single exit
	WhyRecover    Kind = "recover"        // a function that calls recover, and the calls that can recover
	WhyChecks     Kind = "checks"         // an operation that checks at run time (and -nochecks removes)
	WhyRefcount   Kind = "refcount"       // where a reference count is touched or an owner is shared
	WhyInterfaces Kind = "interfaces"     // conversions to interfaces (allocation), assertions, calls
	WhyGenerics   Kind = "generics"       // an instance of a generic function
	WhySelect     Kind = "select"         // a select statement
	WhyGoroutines Kind = "goroutines"     // a go statement
	WhyChannels   Kind = "channels"       // sends, receives, close and range over a channel
	WhyAlloc      Kind = "allocation"     // an operation that allocates
	WhyOrder      Kind = "order"          // a temporary that keeps Go's order of evaluation
	WhyInterop    Kind = "interop"        // a C or C++ name, value or call
	WhyEmbedding  Kind = "embedding"      // promoted fields and methods
	WhyRange      Kind = "range"          // how a range loop is written
	WhyNames      Kind = "names"          // a name that is spelled differently in C++
	WhyMethods    Kind = "methods"        // how a method is written and called
	WhyPanics     Kind = "panics"         // a panic, and the checks that panic with Go's message
	WhyStrings    Kind = "strings"        // strings: literals, concatenation, conversions
	WhySlices     Kind = "slices"         // slices: append, make, copy, slicing
	WhyMaps       Kind = "maps"           // maps: reads, writes, literals
	WhyPackages   Kind = "initialization" // package variables and init functions
)

// AllKinds lists the kinds, in the order that gocpp explain -list shows them.
var AllKinds = []Kind{
	WhyBoxing, WhyClosures, WhyDefer, WhyRecover, WhyChecks, WhyRefcount, WhyInterfaces, WhyGenerics,
	WhySelect, WhyGoroutines, WhyChannels, WhyAlloc, WhyOrder, WhyInterop, WhyEmbedding, WhyRange,
	WhyNames, WhyMethods, WhyPanics, WhyStrings, WhySlices, WhyMaps, WhyPackages,
}

// A Reason is one decision of the lowering at a .gp place.
type Reason struct {
	File string // the .gp file, as the #line directives name it
	Line int
	Col  int
	Kind Kind
	Msg  string // one line, without a trailing period

	// Inst is the C++ name of the generic function's instance whose lowering made the reason
	// (gp_inst_<hash>_Show), or "". The instances of one generic function share its .gp lines,
	// and what holds for one (boxed or not, allocating or not) can be false for another.
	Inst string

	// StmtLine is the line where the innermost statement that contains the reason starts, in the
	// same file, or 0 (a reason about a declaration). A statement can span several lines (a call
	// with its arguments on lines of their own); the annotated view shows the reason with the
	// C++ of the statement.
	StmtLine int
}

// A Why is the log of reasons. The zero value is not usable: NewWhy makes one. A nil *Why is
// the disabled log: its methods do nothing.
type Why struct {
	mu      sync.Mutex
	reasons []Reason
	seen    map[Reason]bool
	once    map[string]bool
	files   map[*token.File]*ast.File  // the files of the packages that lowering saw
	index   map[*token.File]*stmtIndex // their statements, built when a reason first needs them

	// NoChecks says that the build removes the bounds and nil checks (gocpp build -nochecks):
	// the messages of the checks then say so.
	NoChecks bool
}

// NewWhy returns an empty log.
func NewWhy() *Why {
	return &Why{seen: make(map[Reason]bool), once: make(map[string]bool), files: make(map[*token.File]*ast.File), index: make(map[*token.File]*stmtIndex)}
}

// add records a reason, once: lowering visits a node more than once (the instances of a
// generic function share their lines).
func (w *Why) add(pos token.Position, stmt int, inst string, k Kind, msg string) {
	if w == nil || pos.Line == 0 {
		return
	}
	r := Reason{File: pos.Filename, Line: pos.Line, Col: pos.Column, Kind: k, Msg: msg, Inst: inst, StmtLine: stmt}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.seen[r] {
		return
	}
	w.seen[r] = true
	w.reasons = append(w.reasons, r)
}

// first reports whether key is new, and remembers it: a note that is worth saying once for a
// file asks for its own key.
func (w *Why) first(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.once[key] {
		return false
	}
	w.once[key] = true
	return true
}

// Reasons returns the reasons in the order of their places (file, line, column), and, for the
// same place, in the order in which lowering found them. The order is the same on every run.
func (w *Why) Reasons() []Reason {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	out := append([]Reason(nil), w.reasons...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		return a.Inst < b.Inst
	})
	return out
}

// noting reports whether the reason log is on: the hooks that build a message check it first, so
// that the default emit pays for the nil check only.
func (g *gen) noting() bool { return g.opts.Why != nil && g.mute == 0 }

// why records the reason msg of the kind k at pos, when the log is on.
func (g *gen) why(pos token.Pos, k Kind, msg string) {
	if w := g.opts.Why; w != nil && g.mute == 0 && pos.IsValid() {
		if !g.whyReg {
			g.whyReg = true
			w.register(g.fset, g.pkg)
		}
		w.add(g.fset.Position(pos), w.stmtLine(g.fset, pos), g.curInst, k, msg)
	}
}

// noChecks reports whether the log says that the checks are removed (-nochecks).
func (g *gen) noChecks() bool { return g.opts.Why != nil && g.opts.Why.NoChecks }

// quiet runs f without recording reasons: lowering sometimes builds a piece of C++ only to
// replace it (the read m[k] of a store's target), and a reason for the piece that is not in the
// output would be false.
func (g *gen) quiet(f func()) {
	g.mute++
	defer func() { g.mute-- }()
	f()
}

// register remembers the files of the package pkg, to find the statements that reasons are in.
func (w *Why) register(fset *token.FileSet, pkg *check.Package) {
	if w == nil || pkg == nil || pkg.Load == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, f := range pkg.Load.Files {
		if f.AST != nil {
			if tf := fset.File(f.AST.Pos()); tf != nil {
				w.files[tf] = f.AST
			}
		}
	}
}

// A stmtSpan is a statement of a file: where it starts and ends, and the span that contains it.
type stmtSpan struct {
	pos, end token.Pos
	parent   int // the index of the enclosing statement, or -1
}

// A stmtIndex lists the statements of a file in the order of their start (an enclosing statement
// before the ones in it).
type stmtIndex struct {
	file  *ast.File
	spans []stmtSpan
}

func newStmtIndex(f *ast.File) *stmtIndex {
	ix := &stmtIndex{file: f}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.BlockStmt, *ast.EmptyStmt, *ast.BadStmt:
			return true // a block is not a statement of its own for this: its statements are
		}
		if s, ok := n.(ast.Stmt); ok {
			ix.spans = append(ix.spans, stmtSpan{pos: s.Pos(), end: s.End(), parent: -1})
		}
		return true
	})
	sort.SliceStable(ix.spans, func(i, j int) bool {
		a, b := ix.spans[i], ix.spans[j]
		if a.pos != b.pos {
			return a.pos < b.pos
		}
		return a.end > b.end
	})
	var stack []int
	for i := range ix.spans {
		for len(stack) > 0 && ix.spans[stack[len(stack)-1]].end <= ix.spans[i].pos {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			ix.spans[i].parent = stack[len(stack)-1]
		}
		stack = append(stack, i)
	}
	return ix
}

// innermost returns the innermost statement that contains pos, or -1.
func (ix *stmtIndex) innermost(pos token.Pos) int {
	i := sort.Search(len(ix.spans), func(i int) bool { return ix.spans[i].pos > pos }) - 1
	for i >= 0 && ix.spans[i].end <= pos {
		i = ix.spans[i].parent
	}
	return i
}

// stmtLine returns the line where the innermost statement containing pos starts, or 0.
func (w *Why) stmtLine(fset *token.FileSet, pos token.Pos) int {
	tf := fset.File(pos)
	if tf == nil {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	f := w.files[tf]
	if f == nil {
		return 0
	}
	ix := w.index[tf]
	if ix == nil {
		ix = newStmtIndex(f)
		w.index[tf] = ix
	}
	if i := ix.innermost(pos); i >= 0 {
		return fset.Position(ix.spans[i].pos).Line
	}
	return 0
}
