package icheck

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// What makes a failed probe an error. The probe writes an expression as C++
// and asks clang whether it compiles. It is evidence of a compile error of the
// program only when the generated code compiles the same expression with the
// same types. The reasons it may not (each is a reason to stay silent):
//
//   - a stand-in: Go's slice, array and map have no C++ name, so the probe file
//     declares stand-ins with the operations of the runtime's types. They are
//     close, not exact (std::hash<gp::string> exists in the runtime and not in
//     the probe: an unordered_map of strings would fail only in the probe). The
//     stand-in of string is the one that is checked more closely
//     (standErrorOK): only the plain conversion failures that come from the
//     string's own conversions (to std::string_view, and nothing else, as the
//     runtime's) count.
//   - a composite literal: the opaque build initializes a keyed C struct by
//     assignments, not with braces, so narrowing and aggregate rules differ.
//   - a part that has no C++ spelling (Unknown), the element or key of a range
//     (a helper of the probe), a tuple element, a slice expression, a type alias
//     of a local variable (its error belongs to the variable).
//   - a name that can be read two ways (a template or an index, a type or a
//     value): both readings were probed and neither compiled, or one did.
//   - a use inside a defer or go statement (the call is made from a lambda that
//     gets the arguments from the frame, as forwarding references), and inside a
//     generic function (compiled only for the instances that are used).

// inexactExpr returns why the expression is not an exact mirror, "" when it is,
// and whether it has the stand-in of string.
func inexactExpr(e collect.Expr, directArg bool) (why string, stand bool) {
	switch e := e.(type) {
	case nil:
	case *collect.Stand:
		if e.Name == "string" && directArg {
			return "", true
		}
		return "a stand-in of Go's " + e.Name + " is not exactly the runtime's type", false
	case *collect.Composite:
		return "a composite literal is built by assignments in the generated code", false
	case *collect.Unknown:
		return "a part has no C++ spelling: " + e.Why, false
	case *collect.DeclRef:
		return "the type of a local variable named by an alias", false
	case *collect.TupleElem:
		return "the element of a tuple result", false
	case *collect.RangeElem:
		return "the element of a range", false
	case *collect.SliceX:
		return "C++ has no slice expression", false
	case *collect.Arg:
		return inexactExpr(e.T, true)
	case *collect.Decay:
		return inexactExpr(e.X, false)
	case *collect.Inst:
		return firstInexact(append([]collect.Expr{e.X}, e.Args...))
	case *collect.IndexOrInst:
		return firstInexact(append([]collect.Expr{e.X}, e.Args...))
	case *collect.Call:
		return firstInexact(append([]collect.Expr{e.Fun}, e.Args...))
	case *collect.Select:
		return inexactExpr(e.X, false)
	case *collect.Unary:
		return inexactExpr(e.X, false)
	case *collect.Binary:
		return firstInexact([]collect.Expr{e.X, e.Y})
	case *collect.Star:
		return inexactExpr(e.X, false)
	case *collect.Addr:
		return inexactExpr(e.X, false)
	case *collect.Paren:
		return inexactExpr(e.X, false)
	}
	return "", false
}

func firstInexact(es []collect.Expr) (string, bool) {
	stand := false
	for _, e := range es {
		why, s := inexactExpr(e, false)
		if why != "" {
			return why, false
		}
		stand = stand || s
	}
	return "", stand
}

// failureWhy returns why the failure of the probe of r is not reported as an
// error ("" when it is one): it is the same for every use of the reference.
func (c *checker) failureWhy(r *collect.Ref, res *probe.Result) string {
	switch {
	case res.ErrorAt == nil || res.ErrorAt.Severity == "note":
		return "the compiler printed no diagnostic for the expression: " + res.Error
	case strings.HasPrefix(res.Error, "depends on a local variable"):
		return "follows from an error in the declaration of a local variable"
	case res.AltError != "":
		return "the name could be read two ways (the other reading failed too: " + res.AltError + ")"
	case c.arrowMemberError(res.Error):
		return "the member is the pointee's of an operator->: typed code generation writes x->m"
	}
	why, stand := inexactExpr(r.Expr, false)
	if why != "" {
		return why
	}
	if stand && !standErrorOK(res, c.rep.StringToCStr) {
		if c.rep.StringToCStr {
			return "the probe uses the stand-in of string, and the typed code generation converts a Go string to a std::string or a const char* parameter itself"
		}
		return "the probe uses the stand-in of string, and this error is not one of its conversions"
	}
	if c.dependsOnFailed(r) {
		return "follows from an error in a part of the expression"
	}
	return ""
}

// dependsOnFailed reports whether a part of the reference is a reference whose
// probe failed: its error is the cause, and this one repeats it.
func (c *checker) dependsOnFailed(r *collect.Ref) bool {
	found := false
	root := true
	collect.Walk(r.Expr, func(e collect.Expr) bool {
		if root {
			root = false
			return true
		}
		if c.failed[e.String()] {
			found = true
		}
		return !found
	})
	return found
}

// siteWhy returns why a use of a reference is not a mirror of the generated
// code ("" when it is).
func siteWhy(in *astInfo) string {
	switch {
	case in == nil:
		return "the use was not found in the syntax tree"
	case in.inDeferGo:
		return "inside a defer or go statement: the call is made from a lambda with the arguments stored in the frame"
	case in.inGeneric:
		return "inside a generic function: it is compiled only for the instances that are used"
	}
	return ""
}

const standName = "gocpp_probe_internal_pr::string"

var (
	standInitRe = regexp.MustCompile(`^cannot initialize a parameter of type '.*' with an? (lvalue|rvalue) of type '` + standName + `'`)
	arityNoteRe = regexp.MustCompile(`^candidate (function|constructor|function template|constructor template)?.*not viable: requires `)
	arityRe     = regexp.MustCompile(`^too (many|few) arguments`)
)

// standErrorOK reports whether an error of a probe that uses the stand-in of
// string is one of the failures that the stand-in has exactly as the runtime's
// string: a conversion that is refused, or the number of arguments. With typed (the report was made
// for the typed code generation, which converts a Go string to a const char* or std::string parameter
// by itself where every overload agrees) only the number of arguments is.
func standErrorOK(res *probe.Result, typed bool) bool {
	if res.ErrorAt == nil || filepath.Base(res.ErrorAt.File) != "gp_probe.cpp" {
		return false
	}
	msg := res.Error
	switch {
	case strings.HasPrefix(msg, "no viable conversion from '"+standName+"'"),
		standInitRe.MatchString(msg):
		return !typed
	case arityRe.MatchString(msg):
		return true
	case strings.HasPrefix(msg, "no matching function for call to"),
		strings.HasPrefix(msg, "no matching member function for call to"),
		strings.HasPrefix(msg, "no matching constructor for initialization of"):
		if len(res.Notes) == 0 {
			return false
		}
		for _, n := range res.Notes {
			switch {
			case arityNoteRe.MatchString(n.Msg):
			case typed:
				return false
			case strings.HasPrefix(n.Msg, "passing argument to parameter"):
			case strings.Contains(n.Msg, "no known conversion from '"+standName+"'") && !strings.Contains(n.Msg, "template"):
			default:
				return false
			}
		}
		return true
	}
	return false
}

// noMemberRe matches clang's "no member named 'm' in 'T'".
var noMemberRe = regexp.MustCompile(`^no member named '([^']+)' in '([^']+)'(?:;.*)?$`)

// arrowMemberError reports whether the compiler error msg is the missing member of a class that
// has an operator-> to a class that has the member: typed code generation writes x->m there (the
// lowering, ftype.Mapper.ArrowMember), so it is no error of the program.
func (c *checker) arrowMemberError(msg string) bool {
	m := noMemberRe.FindStringSubmatch(msg)
	if m == nil {
		return false
	}
	mp, ok := c.mapType(m[2])
	return ok && mp.Kind == ftype.Record && c.rep.Mapper.ArrowMember(mp.Type, m[1])
}
