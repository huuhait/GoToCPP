package lower

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
)

// The guard of the constants of typed interop (Phase 12 step 12.8, ADR-097 §7).
//
// check put the value that clang gave into the program wherever a C/C++ name sat in an array length, a
// const declaration, a case or a shift count (check.Package.Consts). The program is built with the
// compiler of the build, which can be g++: a header that branches on __clang__ (or on a compiler version)
// gives another value there, and the program would silently use clang's. So the translation unit that
// holds the headers asserts, for each name that was replaced, that the compiler sees the value that was
// used:
//
//	#line 12 "main.gp"
//	static_assert(static_cast<long long>(VV) == 1LL, "gocpp: h.VV differs between the probe compiler (clang++) and this compiler (main.gp:12:10)");
//
// The #line puts a failure at the use in the .gp file. The asserts cost nothing at run time and one
// constant comparison each at compile time; a name is asserted once however often it is used.

// constAsserts returns the static_asserts of the constants of the package, or nil.
func (g *gen) constAsserts() []cppir.Decl {
	var out []cppir.Decl
	seen := make(map[string]bool)
	for _, u := range g.pkg.Consts {
		if u.Spelling == "" || seen[u.Spelling] {
			continue
		}
		seen[u.Spelling] = true
		cond, why := constCond(u, g.cName(u.Spelling, u.At)) // a C/C++ name for the macro protection (cnames.go)
		pos := g.pos(u.At)
		if cond == nil {
			out = append(out, &cppir.Comment{Pos: pos, Text: fmt.Sprintf("gocpp: %s is not checked against this compiler (%s)", u.Written, why)})
			continue
		}
		out = append(out, &cppir.StaticAssert{Pos: pos, Cond: cond, Msg: fmt.Sprintf("gocpp: %s differs between the probe compiler (clang++) and this compiler (%s:%d:%d)",
			plain(u.Written), plain(filepath.Base(u.Pos.Filename)), u.Pos.Line, u.Pos.Column)})
	}
	if len(out) > 0 {
		out = append(out, &cppir.Blank{})
	}
	return out
}

// plain keeps the characters of a name or a file name that need no escape in a message.
func plain(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '.', r == '-', r == '/':
			return r
		}
		return '?'
	}, s)
}

// constCond returns the condition that says that the name has the value that check used, or nil and
// the reason when it cannot be written.
func constCond(u check.ConstUse, name cppir.Expr) (cppir.Expr, string) {
	eq := func(x, y cppir.Expr) cppir.Expr { return &cppir.Binary{Op: "==", X: x, Y: y} }
	a := u.Answer
	switch a.Kind {
	case check.ConstInt, check.ConstChar, check.ConstBool:
		switch v := a.Int; {
		case v == nil:
		case v.IsInt64():
			text := strconv.FormatInt(v.Int64(), 10) + "LL"
			if v.Int64() == math.MinInt64 {
				text = "(-9223372036854775807LL - 1)"
			}
			return eq(call("static_cast<long long>", name), lit(text)), ""
		case v.IsUint64():
			return eq(call("static_cast<unsigned long long>", name), lit(v.String()+"ULL")), ""
		default:
			return nil, "the value is wider than 64 bits"
		}
	case check.ConstFloat:
		return eq(call("static_cast<double>", name), lit(u.Value)), ""
	case check.ConstString:
		n := len(a.Str)
		return eq(call("std::string_view", name, &cppir.Binary{Op: "-", X: call("sizeof", name), Y: lit("1")}),
			call("std::string_view", lit(cString(a.Str)), lit(strconv.Itoa(n)))), ""
	}
	return nil, "no value"
}

// cString returns the C++ string literal of the bytes s: printable ASCII as it is, everything else as a
// three-digit octal escape (which, unlike \x, ends after three digits).
func cString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := range len(s) {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c >= 0x20 && c < 0x7f && c != '?': // ? is left out: ??x is a trigraph in old modes
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "\\%03o", c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// takeConstAsserts returns the asserts that no translation unit has taken yet (all of them for the
// first one) and forgets them.
func (g *gen) takeConstAsserts() []cppir.Decl {
	out := g.pendingAsserts
	g.pendingAsserts = nil
	return out
}
