package check

import (
	"go/ast"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
)

// maxMustRounds bounds the checks of a package with must functions: each
// round finds the calls that the types of the round before could not tell
// (v := os.Open(p); v.Close(): in the first round v has no type), so the
// number of rounds is the depth of such a chain. The README names two
// checks, which is the usual case.
const maxMustRounds = 8

// rewriteDropped is the second step of the pass (see mustPass): with the
// types of the check that has just run, it wraps in a helper call every
// call in a must function that drops its error, and fixes the helpers that
// give a wrong number of values for the argument of a call. It reports
// whether it changed anything: then the package has to be checked again.
func (mp *mustPass) rewriteDropped(info *types.Info) bool {
	if mp.rounds >= maxMustRounds {
		return false
	}
	changed := false
	for _, f := range mp.files {
		mp.scanFile(f, func(st *site) {
			if st.helper != nil {
				if mp.fixArity(info, st) {
					changed = true
				}
				return
			}
			if st.fn == nil || st.fn.must == nil {
				return // not in a must function: a closure inside one is not either
			}
			if arity, ok := mp.dropped(info, st); ok {
				mp.wrap(st, arity, false)
				changed = true
			}
		})
	}
	if changed {
		mp.rounds++
	}
	return changed
}

// results returns the types of the results of the call c as go/types
// recorded it, or ok == false if the call has no type (an error).
func callResults(info *types.Info, c *ast.CallExpr) (res []types.Type, ok bool) {
	tv, found := info.Types[c]
	if !found || tv.Type == nil {
		return nil, false
	}
	if tup, isTuple := tv.Type.(*types.Tuple); isTuple {
		for v := range tup.Variables() {
			res = append(res, v.Type())
		}
		return res, true
	}
	if b, isBasic := tv.Type.(*types.Basic); isBasic && b.Kind() == types.Invalid {
		return nil, false
	}
	return []types.Type{tv.Type}, true
}

// calleeSig returns the signature of the function that c calls, and false
// for a conversion, a built-in function and a call with no type.
func calleeSig(info *types.Info, c *ast.CallExpr) (*types.Signature, bool) {
	tv, ok := info.Types[c.Fun]
	if !ok || tv.Type == nil || tv.IsType() || tv.IsBuiltin() {
		return nil, false
	}
	sig, ok := tv.Type.Underlying().(*types.Signature)
	return sig, ok
}

// concreteError reports whether t is a type that is not error but can be
// assigned to it, and is not an interface: the typed-nil trap.
func concreteError(t types.Type) bool {
	return !mustIsError(t) && !types.IsInterface(t) && types.AssignableTo(t, mustErrorType)
}

// dropped says whether the call of the site drops its error, and how many
// values the helper has to give so that the call fits where it is.
func (mp *mustPass) dropped(info *types.Info, st *site) (arity int, ok bool) {
	sig, ok := calleeSig(info, st.call)
	if !ok || sig.Results().Len() == 0 {
		return 0, false
	}
	res := sig.Results()
	n := res.Len()
	last := res.At(n - 1).Type()
	isErr := mustIsError(last)
	// take reports that a call drops a last result that is a concrete error type.
	take := func() {
		if concreteError(last) {
			ts := types.TypeString(last, func(pkg *types.Package) string {
				if pkg.Path() == mp.lp.ImportPath || pkg.Name() == mp.lp.Name && mp.lp.Name == "main" {
					return ""
				}
				return pkg.Name()
			})
			mp.once(st.call, "the last result of %s is %s, not error: a must function does not handle it "+
				"(a nil %s would be a non-nil error); take it yourself, as in err := %s, or write _ = ...",
				types.ExprString(st.call.Fun), ts, ts, types.ExprString(st.call.Fun)+"(...)")
		}
	}
	value := func() (int, bool) {
		if isErr && n == 2 {
			return 1, true
		}
		return 0, false
	}
	switch st.ctx {
	case ctxStmt, ctxDefer:
		if isErr {
			return 0, true
		}
		take()
	case ctxGo:
	case ctxAssign:
		if st.n >= 1 && st.n <= 3 && n == st.n+1 {
			if isErr {
				return st.n, true
			}
			take()
		}
	case ctxReturn:
		if n == st.n {
			return 0, false // the results are passed on, the error too
		}
		return value()
	case ctxArgs:
		psig, isSig := calleeSig(info, st.parent)
		if !isSig {
			return value()
		}
		if psig.Variadic() {
			return 0, false // all results are passed on
		}
		np := psig.Params().Len()
		if n == np {
			return 0, false
		}
		if isErr && n-1 == np && np >= 1 && np <= 3 {
			return np, true
		}
	default:
		return value()
	}
	return 0, false
}

// once adds an error at the node n, once.
func (mp *mustPass) once(n ast.Node, format string, args ...any) {
	pos := mp.fset.Position(n.Pos())
	for _, d := range mp.errs {
		if d.Pos == pos {
			return
		}
	}
	mp.errorf(n, format, args...)
}

// fixArity changes the helper of a call that is the only argument of
// another call, such as g(F!(a)) with F returning (A, B, error) and g taking
// (A, B): the helper has to give two values, not the one that the syntax
// guessed.
func (mp *mustPass) fixArity(info *types.Info, st *site) bool {
	if st.ctx != ctxArgs || st.helper.arity() != 1 {
		return false
	}
	res, ok := callResults(info, st.helper.inner)
	if !ok || len(res) < 3 || len(res) > 4 || !mustIsError(res[len(res)-1]) {
		return false
	}
	st.helper.helper.Fun.(*ast.SelectorExpr).Sel.Name = mustHelper(len(res) - 1)
	return true
}

// calleeText returns the function of the call as the user wrote it, with
// the ! of a call written with one: pkg.F!, parse![int].
func (c *mustCall) calleeText() string {
	id := calleeIdent(c.inner.Fun)
	if id == nil || !c.explicit {
		return types.ExprString(c.inner.Fun)
	}
	old := id.Name
	id.Name += "!"
	s := types.ExprString(c.inner.Fun)
	id.Name = old
	return s
}

// origText returns the call as the user wrote it: F!(a), or F(a) for a call
// that a must function handles by itself.
func (c *mustCall) origText() string {
	id := calleeIdent(c.inner.Fun)
	if id == nil || !c.explicit {
		return types.ExprString(c.inner)
	}
	old := id.Name
	id.Name += "!"
	s := types.ExprString(c.inner)
	id.Name = old
	return s
}

// postCheck is the third step of the pass: it checks every helper call
// against what go/types found. The last result of the call must be exactly
// error (and not only assignable to it), and the call must have as many
// other results as the helper gives. A call that fails gets an error in the
// words of the user's code, and the go/types errors about its helper (which
// say things like "not enough arguments in call to gp_must.M1") are dropped.
func (mp *mustPass) postCheck(p *Package) {
	if p.Info == nil {
		return
	}
	info := p.Info
	qual := func(pkg *types.Package) string {
		if pkg == p.Types {
			return ""
		}
		return pkg.Name()
	}
	var bad []*mustCall
	for _, c := range mp.calls {
		if !c.explicit {
			continue // made for a call that drops an error that is error: nothing to check
		}
		msg := mp.checkCall(info, c, qual)
		if _, known := callResults(info, c.inner); msg == "" && !known && !mp.hasOtherError(p, c) {
			// go/types has no types for the call, and no error of its own about it: the
			// callee is a C or C++ function (the checker does not know its types).
			msg = strings.TrimPrefix(c.calleeText(), "C.") + "(...): ! is not supported on C and C++ functions yet: " +
				"they do not return Go errors; call it without ! and check its result"
		}
		if msg != "" {
			mp.errorf(c.inner, "%s", msg)
			bad = append(bad, c)
		}
	}
	mp.fixErrors(p, bad)
}

// hasOtherError reports whether go/types has an error in the helper call c that is not about the
// helper itself (an undefined name in it, for example).
func (mp *mustPass) hasOtherError(p *Package, c *mustCall) bool {
	pos, end := mp.fset.Position(c.helper.Pos()), mp.fset.Position(c.helper.End())
	for _, d := range p.Errors.Diagnostics() {
		if d.Severity == diag.Error && d.Pos.Filename == pos.Filename && d.Pos.Offset >= pos.Offset && d.Pos.Offset <= end.Offset &&
			!strings.Contains(d.Msg, mp.name+".") {
			return true
		}
	}
	return false
}

// checkCall returns the error of the helper call c, or "".
func (mp *mustPass) checkCall(info *types.Info, c *mustCall, qual types.Qualifier) string {
	fun := c.calleeText()
	name := types.ExprString(c.inner.Fun)
	if tv, ok := info.Types[c.inner.Fun]; ok {
		switch {
		case tv.IsType():
			return fun + "(...) is a conversion, not a call: ! needs a function whose last result is error"
		case tv.IsBuiltin():
			return fun + "(...) is a call of a built-in function: ! needs a function whose last result is error"
		}
	}
	res, ok := callResults(info, c.inner)
	if !ok {
		return "" // go/types reports the reason
	}
	if len(res) == 0 {
		return fun + " requires the last result of " + name + " to be error, but " + name + " returns no result"
	}
	last := res[len(res)-1]
	if !mustIsError(last) {
		ts := types.TypeString(last, qual)
		if concreteError(last) {
			return fun + " requires the last result of " + name + " to be exactly error, got " + ts +
				": a nil " + ts + " would turn into a non-nil error; convert it explicitly"
		}
		return fun + " requires the last result of " + name + " to be error, got " + ts
	}
	have := len(res) - 1
	want := c.arity()
	if want == 0 || have == want {
		return ""
	}
	switch {
	case have == 0:
		return fun + "(...) returns only an error: use it as a statement"
	case want == 1 && have > 1:
		return fun + "(...) gives " + plural(have, "value") + " besides the error: " +
			"assign them to " + plural(have, "variable") + " (a, b := " + fun + "(...))"
	}
	return fun + "(...) gives " + plural(have, "value") + " besides the error, but here " + plural(want, "value") + " are wanted"
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

// fixErrors changes the list of errors of p: it drops the errors of
// go/types about the helpers of the calls that have an error of the pass,
// writes the helpers in the other messages as the user wrote them, and
// adds the errors of the pass.
func (mp *mustPass) fixErrors(p *Package, bad []*mustCall) {
	type span struct {
		file   string
		lo, hi int
	}
	var spans []span
	for _, c := range bad {
		pos, end := mp.fset.Position(c.helper.Pos()), mp.fset.Position(c.helper.End())
		spans = append(spans, span{pos.Filename, pos.Offset, end.Offset})
	}
	// A helper call that go/types rejects because something else in it is
	// wrong (an undefined callee, a callee that is not a function) also gets
	// "not enough arguments in call to gp_must.M1": that error is about the
	// helper, and goes when another error is in the same call.
	hidden := mp.name + "."
	for _, c := range mp.calls {
		if !c.explicit {
			continue
		}
		pos, end := mp.fset.Position(c.helper.Pos()), mp.fset.Position(c.helper.End())
		for _, d := range p.Errors.Diagnostics() {
			if d.Pos.Filename == pos.Filename && d.Pos.Offset >= pos.Offset && d.Pos.Offset <= end.Offset &&
				!strings.Contains(d.Msg, hidden) {
				spans = append(spans, span{pos.Filename, pos.Offset, end.Offset})
				break
			}
		}
	}
	type repl struct{ from, to string }
	var repls []repl
	for _, c := range mp.calls {
		repls = append(repls, repl{types.ExprString(c.helper), c.origText()})
	}
	// The longest first: a helper inside the text of another one.
	slices.SortStableFunc(repls, func(a, b repl) int { return len(b.from) - len(a.from) })
	fix := func(s string) string {
		for _, r := range repls {
			s = strings.ReplaceAll(s, r.from, r.to)
		}
		return s
	}
	touched := len(spans) > 0 || len(repls) > 0
	if touched {
		p.Errors = p.Errors.Rewrite(func(d diag.Diagnostic) []diag.Diagnostic {
			for _, s := range spans {
				if d.Pos.Filename == s.file && d.Pos.Offset >= s.lo && d.Pos.Offset <= s.hi &&
					(strings.Contains(d.Msg, hidden) || strings.HasSuffix(d.Msg, "(no value) used as value")) {
					return nil // about the helper, not about the user's code
				}
			}
			d.Msg = fix(d.Msg)
			for i := range d.Related {
				d.Related[i].Msg = fix(d.Related[i].Msg)
			}
			return []diag.Diagnostic{d}
		})
	}
	for _, d := range mp.errs {
		p.Errors.Add(d)
	}
}
