package tests

// This file has the parts of the debug test (TestDebug in debug_test.go)
// that need no gocpp binary and no debugger: the reader of a program in
// tests/debug and its // gdb: comments, the gdb session made from them with
// the checks of what gdb answers, and the check of a binary's DWARF line
// table. tests/debug/README.md describes the format.

import (
	"debug/dwarf"
	"debug/elf"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// gdbPrefix starts the comment that makes a line a gdb stop.
const gdbPrefix = "// gdb:"

// gdbLikeRx matches a comment that looks like a gdb stop but isn't written
// as one ("//gdb:", "// GDB :"): an error, so that a stop isn't lost.
var gdbLikeRx = regexp.MustCompile(`(?i)^//\s*gdb\s*:`)

// debugNamespace is the C++ namespace of package main, where gdb finds the
// functions of a test program (ADR-010: gpkg_<import path>).
const debugNamespace = "gpkg_main"

// A debugProgram is a program of tests/debug, read for the debug test.
type debugProgram struct {
	rel   string          // its path for messages: tests/debug/hello.gp
	paths map[string]bool // its absolute path as the debug info may name it (also with symlinks resolved)
	lines []string        // its source lines
	funcs []debugFunc     // its func declarations, in the order of the file
	lits  []debugFunc     // its func literals (step 3.11), named as Go names them: main.func1
	code  []int           // the lines that must have code (codeLines), sorted
	stops []gdbStop       // its // gdb: comments, sorted by line
}

// A debugFunc is a func declaration of a debugProgram, or a func literal
// (a lambda in C++, named main.func1 as Go names it in stack traces).
type debugFunc struct {
	name     string // the Go name; in C++ it is debugNamespace::name
	nameLine int    // the line of the name: the DW_AT_decl_line of its C++ function
	start    int    // the line of the func keyword
	end      int    // the line of the closing brace
	lit      bool   // a func literal
}

// cxxName returns the name that gdb shows for f.
func (f *debugFunc) cxxName() string { return debugNamespace + "::" + f.name }

// A gdbStop is a line with a // gdb: comment: gdb stops there (the first
// time the line runs) and runs the commands of the comment.
type gdbStop struct {
	line int
	cmds []gdbCmd
}

// A gdbCmd is one command of a // gdb: comment.
type gdbCmd struct {
	op     string         // "print", "bt" or "next"
	expr   string         // print: the expression
	want   string         // print: the value that gdb must print (after "$1 = ")
	rx     *regexp.Regexp // print with ~ instead of =: the value must match it
	frames []string       // bt: the C++ names of the program's frames, from the innermost
	lines  []int          // next: the line where each next must stop
}

// parseDebug reads the program src of tests/debug: rel is its path for
// messages, abs its absolute path. Like the other readers, it reports a
// malformed test as an error of the form "path:line: message".
func parseDebug(rel, abs string, src []byte) (*debugProgram, error) {
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		real = abs
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	p := &debugProgram{
		rel:   rel,
		paths: map[string]bool{abs: true, real: true},
		lines: sourceLines(src),
	}
	line := func(pos token.Pos) int { return fset.Position(pos).Line }
	code := make(map[int]bool)
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		fn := debugFunc{name: fd.Name.Name, nameLine: line(fd.Name.Pos()), start: line(fd.Pos()), end: line(fd.Body.Rbrace)}
		if fd.Recv != nil && len(fd.Recv.List) == 1 {
			// A method is a member of its type's struct (Phase 03 step 3.10): gpkg_main::T::M.
			t := fd.Recv.List[0].Type
			if s, ok := t.(*ast.StarExpr); ok {
				t = s.X
			}
			if id, ok := t.(*ast.Ident); ok {
				fn.name = id.Name + "::" + fn.name
			}
		}
		p.funcs = append(p.funcs, fn)
		for _, l := range codeLines(fset, fd) {
			code[l] = true
		}
		p.lits = append(p.lits, funcLits(fset, fd, fn.name)...)
	}
	for l := range code {
		p.code = append(p.code, l)
	}
	slices.Sort(p.code)
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			l := line(c.Pos())
			text, ok := strings.CutPrefix(c.Text, gdbPrefix)
			if !ok {
				if gdbLikeRx.MatchString(c.Text) {
					return nil, fmt.Errorf("%s:%d: write a gdb comment as %q, not %q", rel, l, gdbPrefix+" ...", c.Text)
				}
				continue
			}
			if !code[l] {
				return nil, fmt.Errorf("%s:%d: a %q comment belongs at the end of a line with a statement", rel, l, gdbPrefix)
			}
			cmds, err := parseGDBCommands(text)
			if err == nil {
				err = p.checkCommands(cmds)
			}
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %v", rel, l, err)
			}
			p.stops = append(p.stops, gdbStop{line: l, cmds: cmds})
		}
	}
	if len(p.stops) == 0 {
		return nil, fmt.Errorf("%s: no %q comment: a debug test needs at least one", rel, gdbPrefix)
	}
	slices.SortFunc(p.stops, func(a, b gdbStop) int { return a.line - b.line })
	return p, nil
}

// codeLines returns the lines of fd that must have code in the line table,
// so that "break file.gp:N" stops there: the line of the func keyword (the
// prologue), the first line of each statement that does something, and the
// closing brace of a func that can run off its end (no results, and no
// return at the end: clang++ puts the epilogue of a func that ends with a
// return on the return's line). Not on the list: a block, a label, a const
// or type declaration, fallthrough, a for without a condition, a switch
// (a switch on a variable compares it at the case lines) and case clauses (a
// switch on an integer becomes a C++ switch, whose cases have no code of
// their own). They may have code; they don't have to.
func codeLines(fset *token.FileSet, fd *ast.FuncDecl) []int {
	lines := []int{fset.Position(fd.Pos()).Line}
	add := func(n ast.Node) { lines = append(lines, fset.Position(n.Pos()).Line) }
	end := func(t *ast.FuncType, b *ast.BlockStmt) {
		if body := b.List; t.Results == nil && (len(body) == 0 || !isReturn(body[len(body)-1])) {
			lines = append(lines, fset.Position(b.Rbrace).Line)
		}
	}
	end(fd.Type, fd.Body)
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			end(n.Type, n.Body) // a lambda (step 3.11): its statements, and its closing brace
		case *ast.AssignStmt, *ast.ExprStmt, *ast.IncDecStmt, *ast.ReturnStmt, *ast.SendStmt,
			*ast.GoStmt, *ast.DeferStmt, *ast.IfStmt, *ast.RangeStmt:
			add(n)
		case *ast.BranchStmt:
			if n.Tok != token.FALLTHROUGH {
				add(n)
			}
		case *ast.ForStmt:
			if n.Cond != nil {
				add(n)
			}
		case *ast.DeclStmt:
			if gd, ok := n.Decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
				for _, spec := range gd.Specs {
					add(spec)
				}
			}
		}
		return true
	})
	return lines
}

// funcLits returns the func literals of fd, whose name is name (T::M for a
// method), named as Go names them in stack traces: main.func1, main.func2 in
// source order, main.func1.1 for the first literal in main.func1 (T::M.func1 in
// a method).
func funcLits(fset *token.FileSet, fd *ast.FuncDecl, name string) []debugFunc {
	var lits []debugFunc
	var walk func(n ast.Node, name string, first bool)
	walk = func(n ast.Node, name string, first bool) {
		k := 0
		ast.Inspect(n, func(m ast.Node) bool {
			fl, ok := m.(*ast.FuncLit)
			if !ok || m == n {
				return true
			}
			k++
			sep := "."
			if first {
				sep = ".func"
			}
			lit := debugFunc{name: name + sep + strconv.Itoa(k), start: fset.Position(fl.Pos()).Line, end: fset.Position(fl.Body.Rbrace).Line, lit: true}
			lit.nameLine = lit.start
			lits = append(lits, lit)
			walk(fl, lit.name, false)
			return false
		})
	}
	walk(fd.Body, name, true)
	return lits
}

func isReturn(s ast.Stmt) bool {
	_, ok := s.(*ast.ReturnStmt)
	return ok
}

// parseGDBCommands reads the commands of a // gdb: comment (text is what
// follows "// gdb:"): commands separated by ";".
func parseGDBCommands(text string) ([]gdbCmd, error) {
	var cmds []gdbCmd
	for _, s := range strings.Split(text, ";") {
		s = strings.TrimSpace(s)
		op, rest, _ := strings.Cut(s, " ")
		rest = strings.TrimSpace(rest)
		var c gdbCmd
		switch op {
		case "print":
			// The first " = " or " ~ " ends the expression: the value or the
			// regexp after it may hold either.
			bad := fmt.Errorf("bad %q command: want \"print EXPR = VALUE\" or \"print EXPR ~ REGEXP\", not %q", op, s)
			eq, re := strings.Index(rest, " = "), strings.Index(rest, " ~ ")
			switch {
			case eq < 0 && re < 0:
				return nil, bad
			case re < 0 || eq >= 0 && eq < re:
				c = gdbCmd{op: op, expr: strings.TrimSpace(rest[:eq]), want: strings.TrimSpace(rest[eq+3:])}
				if c.want == "" {
					return nil, bad
				}
			default:
				rx, err := regexp.Compile(strings.TrimSpace(rest[re+3:]))
				if err != nil {
					return nil, fmt.Errorf("bad %q command %q: %v", op, s, err)
				}
				c = gdbCmd{op: op, expr: strings.TrimSpace(rest[:re]), rx: rx}
			}
			if c.expr == "" {
				return nil, bad
			}
		case "bt":
			c = gdbCmd{op: op, frames: strings.Fields(rest)}
			if len(c.frames) == 0 {
				return nil, fmt.Errorf("bad %q command: want the program's frames, innermost first (\"bt %s::f %s::main\"), not %q",
					op, debugNamespace, debugNamespace, s)
			}
		case "next":
			c = gdbCmd{op: op}
			for _, f := range strings.Fields(rest) {
				n, err := strconv.Atoi(f)
				if err != nil || n <= 0 {
					return nil, fmt.Errorf("bad %q command: want the line where each next stops (\"next 12 13\"), not %q", op, s)
				}
				c.lines = append(c.lines, n)
			}
			if len(c.lines) == 0 {
				return nil, fmt.Errorf("bad %q command: want the line where each next stops (\"next 12 13\"), not %q", op, s)
			}
		default:
			return nil, fmt.Errorf("unknown command %q in a %q comment: want print, bt or next", s, gdbPrefix)
		}
		cmds = append(cmds, c)
	}
	return cmds, nil
}

// checkCommands checks the names and the lines in the commands of a stop:
// each frame of a bt is a func of the program, and each line of a next is
// in a func.
func (p *debugProgram) checkCommands(cmds []gdbCmd) error {
	for _, c := range cmds {
		for _, name := range c.frames {
			if p.funcNamed(name) == nil {
				return fmt.Errorf("bt: %s is not a func of the program (write the name as gdb shows it: %s::f)", name, debugNamespace)
			}
		}
		for _, l := range c.lines {
			if p.funcAt(l) == nil {
				return fmt.Errorf("next: line %d is not in a func", l)
			}
		}
	}
	return nil
}

// funcAt returns the func whose lines hold line, or nil.
func (p *debugProgram) funcAt(line int) *debugFunc {
	for i := range p.funcs {
		if f := &p.funcs[i]; f.start <= line && line <= f.end {
			return f
		}
	}
	return nil
}

// funcNamed returns the func or func literal whose C++ name is name, or nil.
func (p *debugProgram) funcNamed(name string) *debugFunc {
	for _, fs := range [][]debugFunc{p.funcs, p.lits} {
		for i := range fs {
			if f := &fs[i]; f.cxxName() == name {
				return f
			}
		}
	}
	return nil
}

// innermost returns the func literal or else the func whose lines hold line,
// or nil: the innermost literal (the last one that starts on its first line).
func (p *debugProgram) innermost(line int) *debugFunc {
	var in *debugFunc
	for i := range p.lits {
		if l := &p.lits[i]; l.start <= line && line <= l.end && (in == nil || l.start >= in.start && l.end <= in.end) {
			in = l
		}
	}
	if in != nil {
		return in
	}
	return p.funcAt(line)
}

// outerOf returns the func literal or func around the func literal lit, or nil.
func (p *debugProgram) outerOf(lit *debugFunc) *debugFunc {
	var in *debugFunc
	for i := range p.lits {
		if l := &p.lits[i]; l != lit && l.start <= lit.start && lit.end <= l.end && (in == nil || l.start >= in.start && l.end <= in.end) {
			in = l
		}
	}
	if in != nil {
		return in
	}
	return p.funcAt(lit.start)
}

// isLambdaFrame reports whether fn, a function as gdb shows it, is the call
// operator of a lambda: a func literal. gdb shows g++'s as
// "operator()<>(int64_t)", clang++'s as "gpkg_main::main()::$_0::operator()<1>(long)".
func isLambdaFrame(fn string) bool {
	return strings.Contains(fn, "operator()<")
}

// frameName returns the name of the frame f as the program's frames are
// written in bt commands: a lambda's is the name of the func literal at its
// line (gpkg_main::main.func1), and a func's is gdb's (gpkg_main::add). It
// returns "" for a frame that is not the program's.
func (p *debugProgram) frameName(f gdbFrame) string {
	if isLambdaFrame(f.fn) && p.paths[f.file] {
		if l := p.innermost(f.line); l != nil && l.lit {
			return l.cxxName()
		}
		return f.fn
	}
	if strings.HasPrefix(f.fn, debugNamespace+"::") {
		return f.fn
	}
	return ""
}

// source returns line n of the program, for messages.
func (p *debugProgram) source(n int) string {
	if n < 1 || n > len(p.lines) {
		return "(no such line)"
	}
	return strconv.Quote(strings.TrimSpace(p.lines[n-1]))
}

// base returns the file name of the program, as "break hello.gp:12" names it.
func (p *debugProgram) base() string { return filepath.Base(p.rel) }

// isCPP reports whether path is the C++ file that gocpp generated for the
// program: hello.gp.cpp in the work folder (cppgen's Config.OutputPath).
func (p *debugProgram) isCPP(path string) bool {
	return filepath.IsAbs(path) && filepath.Base(path) == p.base()+".cpp"
}

// A gdbStep is one gdb command of the session, with the check of what gdb
// printed for it.
type gdbStep struct {
	cmd   string                 // the gdb command
	where string                 // what it is for, for messages: "tests/debug/hello.gp:11: print a"
	check func(out string) error // checks the output of the command; nil checks nothing
}

// gdbSteps returns the gdb session for the program. For each stop, in the
// order of the lines: a temporary breakpoint at the line (break hello.gp:11),
// run (the first time) or continue, a check that gdb stopped at the line, then
// the stop's commands; each next is followed by a frame command, to see where
// it stopped. When the last next of a stop already stopped at the line of the
// next stop, there is no breakpoint and no continue for it. After the last
// stop, continue must run the program to its end.
func (p *debugProgram) gdbSteps() []gdbStep {
	var steps []gdbStep
	at := 0 // the line where the last next stopped, or 0
	started := false
	for _, stop := range p.stops {
		where := fmt.Sprintf("%s:%d", p.rel, stop.line)
		if at != stop.line {
			steps = append(steps, gdbStep{
				cmd:   fmt.Sprintf("tbreak %s:%d", p.base(), stop.line),
				where: where,
				check: p.checkBreakpoint(stop.line),
			})
			resume := "continue"
			if !started {
				resume, started = "run", true
			}
			steps = append(steps, gdbStep{cmd: resume, where: where, check: checkStopped(stop.line)})
		}
		steps = append(steps, gdbStep{cmd: "frame", where: where + ": the stop", check: p.checkFrame(stop.line)})
		for _, c := range stop.cmds {
			switch c.op {
			case "print":
				steps = append(steps, gdbStep{cmd: "print " + c.expr, where: where + ": print " + c.expr, check: checkValue(c)})
			case "bt":
				steps = append(steps, gdbStep{cmd: "bt", where: where + ": bt", check: p.checkBacktrace(c.frames, stop.line)})
			case "next":
				from := stop.line
				for _, l := range c.lines {
					w := fmt.Sprintf("%s: next from line %d", where, from)
					steps = append(steps, gdbStep{cmd: "next", where: w}, gdbStep{cmd: "frame", where: w, check: p.checkFrame(l)})
					from = l
				}
			}
		}
		at = lastNext(stop)
	}
	return append(steps, gdbStep{cmd: "continue", where: p.rel + ": the end", check: checkExited})
}

// lastNext returns the line where the last next of stop stops, or 0 if
// stop has no next (then gdb is still at stop.line, which no later stop has).
func lastNext(stop gdbStop) int {
	for _, c := range slices.Backward(stop.cmds) {
		if c.op == "next" {
			return c.lines[len(c.lines)-1]
		}
	}
	return 0
}

// breakpointRx matches gdb's answer to tbreak: "Temporary breakpoint 1 at
// 0x3412: file /abs/hello.gp, line 13."; breakpointsRx matches it for a
// line with code in several functions (a statement with a lambda):
// "Temporary breakpoint 1 at 0x34ce: results.gp:31. (2 locations)", which
// names the file as the tbreak command did.
var (
	breakpointRx  = regexp.MustCompile(`(?m)^Temporary breakpoint \d+ at 0x[0-9a-f]+: file (.+), line (\d+)\.$`)
	breakpointsRx = regexp.MustCompile(`(?m)^Temporary breakpoint \d+ at 0x[0-9a-f]+: (.+):(\d+)\. \(\d+ locations\)$`)
)

// checkBreakpoint checks that gdb put the breakpoint at line, in the .gp
// file: gdb moves a breakpoint on a line without code to the next line
// that has code.
func (p *debugProgram) checkBreakpoint(line int) func(string) error {
	return func(out string) error {
		m := breakpointRx.FindStringSubmatch(out)
		inGP := func(file string) bool { return p.paths[file] }
		if m == nil {
			m = breakpointsRx.FindStringSubmatch(out)
			inGP = func(file string) bool { return file == p.base() }
		}
		if m == nil {
			return errors.New("gdb set no breakpoint")
		}
		if n, _ := strconv.Atoi(m[2]); !inGP(m[1]) || n != line {
			return fmt.Errorf("gdb put the breakpoint at %s:%s, not at line %d of %s (%s)", m[1], m[2], line, p.rel, p.source(line))
		}
		return nil
	}
}

// A gdbFrame is a line of gdb's backtrace (or of its frame command):
// "#0  gpkg_main::add (a=7, b=5) at /abs/hello.gp:11".
type gdbFrame struct {
	num  int
	fn   string // the function, "gpkg_main::add"
	file string // the file, or "" without debug info
	line int
}

// frameRx matches a frame line. A frame without debug info ends with
// "from /lib/libc.so.6" or with nothing after the arguments.
var frameRx = regexp.MustCompile(`^#(\d+)\s+(?:0x[0-9a-f]+ in )?(.+?) \(.*\)(?: at (.+):(\d+)| from .*)?$`)

// parseFrames returns the frame lines of out, in order.
func parseFrames(out string) []gdbFrame {
	var frames []gdbFrame
	for _, l := range strings.Split(out, "\n") {
		m := frameRx.FindStringSubmatch(strings.TrimRight(l, "\r"))
		if m == nil {
			continue
		}
		f := gdbFrame{fn: m[2], file: m[3]}
		f.num, _ = strconv.Atoi(m[1])
		f.line, _ = strconv.Atoi(m[4])
		frames = append(frames, f)
	}
	return frames
}

// String returns the frame as a message shows it.
func (f gdbFrame) String() string {
	if f.file == "" {
		return f.fn + " (no line information)"
	}
	return fmt.Sprintf("%s at %s:%d", f.fn, f.file, f.line)
}

// checkFrame checks the answer to gdb's frame command: the program stopped at
// line of the .gp file, in the func that holds the line.
func (p *debugProgram) checkFrame(line int) func(string) error {
	return func(out string) error {
		frames := parseFrames(out)
		if len(frames) == 0 || frames[0].num != 0 {
			return errors.New("gdb shows no frame: the program is not stopped")
		}
		f := frames[0]
		want := p.innermost(line)
		if want == nil {
			return fmt.Errorf("line %d is not in a func", line)
		}
		// The line where a func literal starts is also a line of the code around it (the
		// statement that makes the closure).
		if want.lit && want.start == line && p.frameName(f) != want.cxxName() {
			if outer := p.outerOf(want); outer != nil {
				want = outer
			}
		}
		if !p.paths[f.file] || f.line != line || p.frameName(f) != want.cxxName() {
			return fmt.Errorf("stopped in %v, want %s at line %d of %s (%s)", f, want.cxxName(), line, p.rel, p.source(line))
		}
		return nil
	}
}

// checkBacktrace checks gdb's backtrace at a stop at line: the frames of
// the program (in debugNamespace, and the lambdas of its func literals, named
// by frameName) are want, from frame #0 on, each at a line of its func (or
// func literal) in the .gp file, and frame #0 at line; the outermost frame is
// the main function that gocpp generates, at a line of the generated C++
// file. Between two of the program's frames, only a deferred call's frames or
// a func value's may come: a run of frames of the runtime and the C++ library
// (gp::, std::, __) whose outermost one, just inside the next program frame,
// is gp::defer_frame::gp_run (which calls the deferred function through
// std::apply) or a frame of gp::func (runtimeRun).
func (p *debugProgram) checkBacktrace(want []string, line int) func(string) error {
	return func(out string) error {
		frames := parseFrames(out)
		var got []string
		for _, f := range frames {
			if n := p.frameName(f); n != "" {
				got = append(got, n)
			}
		}
		if !slices.Equal(got, want) {
			return fmt.Errorf("the program's frames are %v, want %v", got, want)
		}
		k := 0 // the next frame of want
		for i, f := range frames {
			if k == len(want) {
				break
			}
			if f.num != i || p.frameName(f) != want[k] {
				if k > 0 && f.num == i && p.runtimeRun(frames[i:], want[k]) {
					continue // a deferred call, or a call of a func value
				}
				return fmt.Errorf("frame #%d is %v; the program's frames must come first", i, f)
			}
			fn := p.funcNamed(want[k])
			switch {
			case fn == nil:
				return fmt.Errorf("%s is not a func of %s", want[k], p.rel)
			case !p.paths[f.file] || f.line < fn.start || f.line > fn.end:
				return fmt.Errorf("frame #%d is %v, not at a line of %s in %s (lines %d to %d)", i, f, fn.name, p.rel, fn.start, fn.end)
			case i == 0 && f.line != line:
				return fmt.Errorf("frame #0 is %v, not at line %d", f, line)
			}
			k++
		}
		last := frames[len(frames)-1]
		if last.fn != "main" || !p.isCPP(last.file) {
			return fmt.Errorf("the outermost frame is %v, want main at a line of the generated %s.cpp", last, p.base())
		}
		return nil
	}
}

// runtimeRun reports whether frames start with a run of the runtime's and the
// C++ library's frames (gp::, std::) that ends with gp::defer_frame::gp_run (a
// deferred call) or with a frame of gp::func (gp::func<...>::operator(), the
// call of a func value), right before the program's frame next.
func (p *debugProgram) runtimeRun(frames []gdbFrame, next string) bool {
	for i, f := range frames {
		if p.frameName(f) == next {
			return i > 0 && (frames[i-1].fn == "gp::defer_frame::gp_run" || strings.HasPrefix(frames[i-1].fn, "gp::func<"))
		}
		// (clang++'s debug info names some of the C++ library's templates without their
		// namespace: __apply_impl<...>; names that start with __ are the library's.)
		if !strings.HasPrefix(f.fn, "gp::") && !strings.HasPrefix(f.fn, "std::") && !strings.HasPrefix(f.fn, "__") {
			return false
		}
	}
	return false
}

// valueRx matches gdb's answer to print: "$1 = 12".
var valueRx = regexp.MustCompile(`^\$\d+ = `)

// checkValue checks gdb's answer to the print command c.
func checkValue(c gdbCmd) func(string) error {
	return func(out string) error {
		out = strings.TrimSpace(out)
		loc := valueRx.FindStringIndex(out)
		if loc == nil {
			return fmt.Errorf("gdb printed no value: %q", out)
		}
		got := out[loc[1]:]
		switch {
		case c.rx != nil && !c.rx.MatchString(got):
			return fmt.Errorf("print %s = %s, which doesn't match %q", c.expr, got, c.rx)
		case c.rx == nil && got != c.want:
			return fmt.Errorf("print %s = %s, want %s", c.expr, got, c.want)
		}
		return nil
	}
}

// exitedRx matches gdb's line for a program that exited with status 0;
// endedRx matches it for any end of the program.
var (
	exitedRx = regexp.MustCompile(`(?m)^\[Inferior 1 \(process \d+\) exited normally\]$`)
	endedRx  = regexp.MustCompile(`(?m)^\[Inferior 1 \(process \d+\) exited.*\]$|^Program terminated with signal`)
)

// checkStopped checks gdb's answer to run or continue: the program must
// not have ended before it reached the breakpoint at line.
func checkStopped(line int) func(string) error {
	return func(out string) error {
		if endedRx.MatchString(out) {
			return fmt.Errorf("the program ended before it reached line %d: the stops run in the order of their lines, "+
				"so line %d must run after the line of the stop before it", line, line)
		}
		return nil
	}
}

// checkExited checks that the program ran to its end and exited with 0.
func checkExited(out string) error {
	if !exitedRx.MatchString(out) {
		return errors.New("the program didn't exit normally after the last stop")
	}
	return nil
}

// ptraceRx matches what gdb says when the system doesn't let it trace the
// program (a container or sandbox without ptrace, or Yama's ptrace_scope 3);
// stoppedRx matches gdb's line for a stop at a temporary breakpoint
// ("Temporary breakpoint 1, gpkg_main::add (a=7, b=5) at ...").
var (
	ptraceRx  = regexp.MustCompile(`(?i)ptrace[^\n]*(not permitted|permission denied)|could not trace the inferior`)
	stoppedRx = regexp.MustCompile(`(?m)^Temporary breakpoint \d+(\.\d+)?, `)
)

// ptraceProblem returns what gdb said about ptrace in run, its output for
// the session's run step, when the program didn't stop because gdb may not
// trace it; or "". gdb's output elsewhere doesn't count: a failure there is
// a failure of the test, and a ptrace warning in a session that stopped is
// harmless.
func ptraceProblem(run string) string {
	if stoppedRx.MatchString(run) {
		return ""
	}
	return ptraceRx.FindString(run)
}

// gdbStepMark is the line that gdb echoes before the output of each step,
// so that the output can be split by step.
func gdbStepMark(i int) string { return fmt.Sprintf("@@gocpp-debug-step %d@@", i) }

// splitGDBOutput splits gdb's output at the step marks: out[i] is what gdb
// printed for step i. A step without its mark (gdb stopped early) gets "".
func splitGDBOutput(out string, steps int) []string {
	parts := make([]string, steps)
	for i := range steps {
		mark := gdbStepMark(i) + "\n"
		_, rest, ok := strings.Cut(out, mark)
		if !ok {
			continue
		}
		if j := strings.Index(rest, gdbStepMark(i+1)+"\n"); j >= 0 {
			rest = rest[:j]
		}
		parts[i] = rest
	}
	return parts
}

// A lineRow is a row of a DWARF line table.
type lineRow struct {
	addr   uint64
	file   string
	line   int
	isStmt bool
}

// A dwarfFunc is a function of the binary with code: a DW_TAG_subprogram
// with an address range.
type dwarfFunc struct {
	name   string      // its name, "main" or "add" (without namespaces)
	file   string      // the file of its declaration (DW_AT_decl_file), or ""
	line   int         // the line of its declaration (DW_AT_decl_line)
	ranges [][2]uint64 // its code
	gp     *debugFunc  // the func of the program it was generated for, or nil
}

func (f *dwarfFunc) contains(addr uint64) bool {
	for _, r := range f.ranges {
		if r[0] <= addr && addr < r[1] {
			return true
		}
	}
	return false
}

// readLineTable reads the line table rows and the functions of the ELF
// binary bin.
func readLineTable(bin string) ([]lineRow, []dwarfFunc, error) {
	ef, err := elf.Open(bin)
	if err != nil {
		return nil, nil, err
	}
	defer ef.Close()
	d, err := ef.DWARF()
	if err != nil {
		return nil, nil, fmt.Errorf("%s: no debug info: %v", bin, err)
	}
	var rows []lineRow
	var funcs []dwarfFunc
	var files []*dwarf.LineFile // the files of the current compile unit
	r := d.Reader()
	for {
		e, err := r.Next()
		if err != nil {
			return nil, nil, err
		}
		if e == nil {
			break
		}
		switch e.Tag {
		case dwarf.TagCompileUnit:
			lr, err := d.LineReader(e)
			if err != nil {
				return nil, nil, err
			}
			files = nil
			if lr == nil {
				continue
			}
			for {
				var le dwarf.LineEntry
				if err := lr.Next(&le); err == io.EOF {
					break
				} else if err != nil {
					return nil, nil, err
				}
				if le.EndSequence {
					continue
				}
				row := lineRow{addr: le.Address, line: le.Line, isStmt: le.IsStmt}
				if le.File != nil {
					row.file = le.File.Name
				}
				rows = append(rows, row)
			}
			files = lr.Files()
		case dwarf.TagSubprogram:
			ranges, err := d.Ranges(e)
			if err != nil {
				return nil, nil, err
			}
			if len(ranges) == 0 {
				continue // a declaration
			}
			f := dwarfFunc{ranges: ranges}
			if err := subprogramDecl(d, e, files, &f); err != nil {
				return nil, nil, err
			}
			funcs = append(funcs, f)
		}
	}
	return rows, funcs, nil
}

// subprogramDecl sets the name, the file and the line of f from the
// subprogram e, or from the declaration that e completes (a definition after
// a prototype has a DW_AT_specification; it names only what differs).
func subprogramDecl(d *dwarf.Data, e *dwarf.Entry, files []*dwarf.LineFile, f *dwarfFunc) error {
	for range 3 { // a definition, its declaration, an abstract origin
		if f.name == "" {
			f.name, _ = e.Val(dwarf.AttrName).(string)
		}
		if f.file == "" {
			if i, ok := e.Val(dwarf.AttrDeclFile).(int64); ok && i >= 0 && int(i) < len(files) && files[i] != nil {
				f.file = files[i].Name
			}
		}
		if f.line == 0 {
			if l, ok := e.Val(dwarf.AttrDeclLine).(int64); ok {
				f.line = int(l)
			}
		}
		off, ok := e.Val(dwarf.AttrSpecification).(dwarf.Offset)
		if !ok {
			off, ok = e.Val(dwarf.AttrAbstractOrigin).(dwarf.Offset)
		}
		if !ok {
			return nil
		}
		r := d.Reader()
		r.Seek(off)
		next, err := r.Next()
		if err != nil {
			return err
		}
		if next == nil {
			return nil
		}
		e = next
	}
	return nil
}

// checkLineTable checks the DWARF line table of bin, the binary that gocpp
// built from p, and returns one message per problem:
//   - each func of p has code: a C++ function declared at the line of its
//     name;
//   - each address of such a function maps to its func's lines in the .gp
//     file (line 0, "no line", is allowed: debuggers skip it). So does each
//     address of a lambda that gocpp writes into a statement (f(g()) with
//     several results): a function that is no func's, but some of whose code
//     maps to the .gp file; all of its code must then map to the lines of one
//     func;
//   - no other address maps to the .gp file: generated-only code doesn't
//     land on unrelated .gp lines;
//   - the functions that gocpp generates (main, later the init helpers) map to
//     the generated .cpp file (Config.OutputPath);
//   - each line of p.code has a row with is_stmt, so "break file.gp:N" stops
//     at line N.
//
// Code that a compiler inlines from a runtime header into a func would map
// to that header and break the second rule. At -O0 there is none today (the
// programs' binaries have inlined subroutines, but none inside a func); when
// there is, the check should skip the addresses of DW_TAG_inlined_subroutine
// entries whose DW_AT_call_file is the .gp file.
//
// It also returns a summary of what it checked, for the log.
func (p *debugProgram) checkLineTable(bin string) (problems []string, summary string, err error) {
	rows, all, err := readLineTable(bin)
	if err != nil {
		return nil, "", err
	}
	// user: the funcs' functions; generated: the ones declared in the .cpp
	// file; other: the runtime, the C++ library and the lambdas (clang
	// declares a lambda at its .gp line, g++ names no line).
	var user, generated, other, allp []*dwarfFunc
	found := make(map[*debugFunc]bool)
	for i := range all {
		f := &all[i]
		allp = append(allp, f)
		switch {
		case p.paths[f.file]:
			for j := range p.funcs {
				if p.funcs[j].nameLine == f.line {
					f.gp = &p.funcs[j]
				}
			}
			switch {
			case f.gp != nil:
				found[f.gp] = true
				user = append(user, f)
			case p.funcAt(f.line) == nil:
				problems = append(problems, fmt.Sprintf("the C++ function %s is declared at line %d of %s, outside the funcs", f.name, f.line, p.rel))
			default:
				other = append(other, f)
			}
		case p.isCPP(f.file):
			generated = append(generated, f)
		default:
			other = append(other, f)
		}
	}
	for i := range p.funcs {
		if !found[&p.funcs[i]] {
			problems = append(problems, fmt.Sprintf("func %s (line %d) has no C++ function with code declared at its line", p.funcs[i].name, p.funcs[i].nameLine))
		}
	}
	if !slices.ContainsFunc(generated, func(f *dwarfFunc) bool { return f.name == "main" }) {
		problems = append(problems, fmt.Sprintf("no function main declared in the generated %s.cpp", p.base()))
	}
	funcAt := func(fs []*dwarfFunc, addr uint64) *dwarfFunc {
		for _, f := range fs {
			if f.contains(addr) {
				return f
			}
		}
		return nil
	}
	// A function of other with code that maps to the .gp file is a lambda
	// of the func of the first such line.
	var lambdas []*dwarfFunc
	for _, row := range rows {
		if o := funcAt(other, row.addr); o != nil && o.gp == nil && p.paths[row.file] && p.funcAt(row.line) != nil {
			o.gp = p.funcAt(row.line)
			lambdas = append(lambdas, o)
		}
	}
	stmt := make(map[int]bool) // the .gp lines with an is_stmt row
	nUser, nGenerated := 0, 0
	for _, row := range rows {
		inGP := p.paths[row.file]
		if inGP && row.isStmt {
			stmt[row.line] = true
		}
		where := fmt.Sprintf("address %#x", row.addr)
		if u := funcAt(append(user, lambdas...), row.addr); u != nil {
			nUser++
			what := "func " + u.gp.name
			if slices.Contains(lambdas, u) {
				what = fmt.Sprintf("the C++ function %s (a lambda in func %s)", u.name, u.gp.name)
			}
			switch {
			case !inGP:
				problems = append(problems, fmt.Sprintf("%s in %s maps to %s:%d, not to %s", where, what, row.file, row.line, p.rel))
			case row.line != 0 && (row.line < u.gp.start || row.line > u.gp.end):
				problems = append(problems, fmt.Sprintf("%s in %s maps to line %d (%s), outside func %s (lines %d to %d)",
					where, what, row.line, p.source(row.line), u.gp.name, u.gp.start, u.gp.end))
			}
			continue
		}
		g := funcAt(generated, row.addr)
		switch {
		case g != nil:
			nGenerated++
			if !p.isCPP(row.file) {
				problems = append(problems, fmt.Sprintf("%s in the generated function %s maps to %s:%d, not to the generated %s.cpp", where, g.name, row.file, row.line, p.base()))
			}
		case inGP:
			in := "no function"
			if f := funcAt(allp, row.addr); f != nil {
				in = "the C++ function " + f.name
			}
			problems = append(problems, fmt.Sprintf("%s (in %s) maps to line %d of %s (%s), but only the code of its funcs may map to the .gp file",
				where, in, row.line, p.rel, p.source(row.line)))
		}
	}
	for _, l := range p.code {
		if !stmt[l] {
			problems = append(problems, fmt.Sprintf("line %d (%s) has no is_stmt row in the line table, so \"break %s:%d\" doesn't stop there",
				l, p.source(l), p.base(), l))
		}
	}
	const maxProblems = 20
	if len(problems) > maxProblems {
		problems = append(problems[:maxProblems], fmt.Sprintf("(and %d more problems)", len(problems)-maxProblems))
	}
	summary = fmt.Sprintf("%d rows: %d in %d funcs and %d lambdas, %d in %d generated functions; %d lines with code",
		len(rows), nUser, len(user), len(lambdas), nGenerated, len(generated), len(p.code))
	return problems, summary, nil
}
