package icheck

import (
	"fmt"
	"go/types"
	"os"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

func readLines(file string) []string {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	return strings.Split(string(b), "\n")
}

// checkFailed handles a reference whose probe failed: an error at each use
// when the failure is certain (exact.go), else a silent entry.
func (c *checker) checkFailed(r *collect.Ref, res *probe.Result) {
	why := c.failureWhy(r, res)
	for _, st := range r.Sites {
		w := why
		var info *astInfo
		if st.Ctx == collect.CtxCallee || st.Ctx == collect.CtxTemplate {
			// The name is called (or given template arguments): the call is the
			// expression, with its own reference. The probe of the bare name is of a
			// value use of the same name (a method value), which is another site.
			c.silent(st, r, "the name is only the function of a call: the call is the expression", res.Error)
			continue
		}
		if w == "" {
			info = c.ix.lookup(st.File, st.Line, st.Col, st.Text)
			w = siteWhy(info)
		}
		if w != "" {
			c.silent(st, r, w, res.Error)
			continue
		}
		fc := c.newFailCtx(r, res, st, info)
		msg, related, pos := c.explainFailure(fc)
		if fc.skip != "" {
			c.silent(st, r, fc.skip, res.Error)
			continue
		}
		c.report(pos, msg, related)
	}
}

// The contexts in which a value of C++ is used as a value (a void result is
// an error there): a statement is the only place where it may be void.
func valueContext(ctx collect.Context, target string) bool {
	switch ctx {
	case collect.CtxInit, collect.CtxAssign, collect.CtxArg, collect.CtxCond:
		return true
	case collect.CtxReturn:
		return target != "" // a function with results needs one
	}
	return false
}

// checkOK applies the typed rules to a reference whose probe compiled.
func (c *checker) checkOK(r *collect.Ref, res *probe.Result, v ftype.Value) {
	if v.Kind == ftype.Opaque {
		return
	}
	why, stand := inexactExpr(r.Expr, false)
	if why == "" && stand {
		why = "the probe uses the stand-in of string"
	}
	for _, st := range r.Sites {
		if why != "" || st.Ctx == collect.CtxCallee || st.Ctx == collect.CtxTemplate {
			continue // not a mirror, or only the name of a call: nothing to say about what the type is
		}
		info := c.ix.lookup(st.File, st.Line, st.Col, st.Text)
		if w := siteWhy(info); w != "" {
			c.silent(st, r, w, "")
			continue
		}
		text := userText(st.Text, c.hdrs)
		pos := c.pos(st)
		switch {
		case info.lhs:
			c.checkLHS(r, v, st, text)
		case v.Kind == ftype.Void && !v.IsType && valueContext(st.Ctx, st.Target):
			c.report(pos, fmt.Sprintf(fmtNoValue, text), nil)
		case !v.IsType && st.Target != "" && compatContext(st.Ctx):
			c.checkTarget(r, v, st, info, text)
		}
	}
}

func compatContext(ctx collect.Context) bool {
	switch ctx {
	case collect.CtxInit, collect.CtxAssign, collect.CtxReturn, collect.CtxArg:
		return true
	}
	return false
}

// checkLHS applies the rule for the left side of an assignment: a C++ value
// that is not an lvalue of a scalar type cannot be assigned to (a class
// prvalue can: C++ calls its operator=).
func (c *checker) checkLHS(r *collect.Ref, v ftype.Value, st collect.Site, text string) {
	if v.IsType {
		return
	}
	switch v.Kind {
	case ftype.Scalar, ftype.Pointer, ftype.Enum, ftype.FuncPtr:
	default:
		return
	}
	pos := c.pos(st)
	switch {
	case v.Category == probe.CatPRValue || v.Category == probe.CatXValue:
		c.report(pos, fmt.Sprintf(fmtNotAssign, text), nil)
	case v.Lvalue && !v.Assignable:
		c.report(pos, fmt.Sprintf(fmtConstAssign, text), nil)
	}
}

// checkTarget applies the compatibility rule to a value that goes to a Go type
// (st.Target): ftype.Value.Compat says No only when no C++ conversion exists.
func (c *checker) checkTarget(r *collect.Ref, v ftype.Value, st collect.Site, info *astInfo, text string) {
	verdict := v.Compat(st.Target)
	if verdict == ftype.Unknown && v.Kind == ftype.Record {
		verdict = c.recordCompat(v, st.Target)
	}
	if verdict != ftype.No {
		return
	}
	var where string
	switch st.Ctx {
	case collect.CtxInit:
		where = "variable declaration"
	case collect.CtxAssign:
		where = "assignment"
	case collect.CtxReturn:
		where = "return statement"
	default:
		where = "argument to " + userText(info.callee, c.hdrs)
		if info.callee == "" {
			where = "argument"
		}
	}
	what := "value of type " + c.disp(v.Mapped)
	if v.Lvalue {
		what = "variable of type " + c.disp(v.Mapped)
	}
	var rel []diag.Related
	if v.Kind == ftype.Void {
		rel = append(rel, diag.Related{Msg: "the C++ function returns void"})
	}
	c.report(c.pos(st), fmt.Sprintf(fmtCannotUse, text, what, st.Target, where), rel)
}

// recordCompat refines Value.Compat for a class: Compat says Unknown because a
// conversion operator may exist; the declaration says whether one does. A
// complete class that has no conversion operator (also in its bases) has no
// conversion to a number, a bool, a pointer or a function.
func (c *checker) recordCompat(v ftype.Value, target string) ftype.Verdict {
	if v.Type == nil || v.Incomplete {
		return ftype.Unknown
	}
	switch {
	case target == "string", strings.HasPrefix(target, "[]"), strings.HasPrefix(target, "map["):
		return ftype.Unknown // the runtime's types have constructors of their own
	case !isScalarGoType(target) && !strings.HasPrefix(target, "*") && !strings.HasPrefix(target, "func("):
		return ftype.Unknown
	}
	if c.hasConversion(v.Type, 0) {
		return ftype.Unknown
	}
	return ftype.No
}

// hasConversion reports whether a class, or a base of it, has a conversion
// operator or an operator template (or is not known well enough to tell).
func (c *checker) hasConversion(t types.Type, depth int) bool {
	ms := c.rep.Mapper.MembersOf(t)
	if ms == nil || depth > 8 {
		return true
	}
	for _, m := range ms.Methods {
		if m.Kind == probe.KindConversion || m.Kind == probe.KindOperator && strings.HasPrefix(m.Name, "operator ") || m.Template && strings.HasPrefix(m.Name, "operator") {
			return true
		}
	}
	for _, b := range ms.Bases {
		if b.Kind != ftype.Record || c.hasConversion(b.Type, depth+1) {
			return true
		}
	}
	return false
}

func isScalarGoType(t string) bool {
	switch t {
	case "bool", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "byte", "rune":
		return true
	}
	return false
}
