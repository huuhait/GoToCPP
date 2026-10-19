package bindgen

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// A constant is a macro or a const variable with a compile-time value.
type constant struct {
	Name  string // the C++ name (qualified for a variable in a namespace)
	Go    string // the name in the view
	Lit   string // the Go literal
	Type  string // the Go type of a typed constant (a const variable, an enum macro), else ""
	Macro bool
	Seq   int
	Decl  *gdecl // the const variable, whose name the constant takes
}

// A structMacro is a macro whose value is a struct or an array: Go constants
// must be basic types, so the view has a var.
type structMacro struct {
	Name string
	Expr string // the Go composite literal
	Type string
	Seq  int
}

// A pendingStruct is a struct macro and the initializer that the compiler gave it.
type pendingStruct struct {
	name string
	seq  int
	node *jnode
	typ  string
}

// A skippedMacro is a macro with no translation.
type skippedMacro struct {
	Name   string
	Params string
	Why    string
}

// constants holds what the probe says about the macros and const variables.
type constants struct {
	consts  []*constant
	structs []*structMacro
	pending []*pendingStruct // struct macros whose literals are written after the names are known
	skipped []skippedMacro
}

// macroCandidates returns the macros to ask the compiler about, and puts the
// others (function-like, empty, include guards, implementation names) in skipped.
func macroCandidates(pp *ppResult, allows func(string) bool, skipped *[]skippedMacro) []*macro {
	var out []*macro
	for _, m := range pp.Macros {
		switch {
		case strings.HasPrefix(m.Name, "__"):
			continue
		case hostileNames[m.Name]:
			*skipped = append(*skipped, skippedMacro{m.Name, "", "its name is a C++ keyword or a name the probe uses"})
		case implementationMacro(m):
			continue
		case !allows(m.Name):
			continue
		case m.FuncLike:
			*skipped = append(*skipped, skippedMacro{m.Name, m.Params, "function-like"})
		case m.Body == m.Name:
			continue // #define SOCK_STREAM SOCK_STREAM: the enumerator is the name already
		case m.Body == "" || isGuard(m):
			continue
		case dependsOnUse(m.Body):
			*skipped = append(*skipped, skippedMacro{m.Name, "", "its value depends on where it is used"})
		case !balanced(m.Body):
			*skipped = append(*skipped, skippedMacro{m.Name, "", "not an expression the compiler accepts"})
		default:
			out = append(out, m)
		}
	}
	return out
}

// implementationMacro says that a macro belongs to the C or C++ library's
// implementation: _GLIBCXX_X and the like, or a name that starts with an
// underscore and a capital letter and comes from a part file (bits/...). The
// standard's own _IOFBF is defined by the header itself and stays.
func implementationMacro(m *macro) bool {
	if len(m.Name) < 2 || m.Name[0] != '_' {
		return false
	}
	for _, p := range []string{"_GLIBCXX", "_GLIBC_", "_LIBCPP", "_STL", "_MSVC", "_CRT", "_LIBCXX"} {
		if strings.HasPrefix(m.Name, p) {
			return true
		}
	}
	return m.Name[1] >= 'A' && m.Name[1] <= 'Z' && isBits(m.File)
}

// dependsOnUse says that a macro body names one of the predefined macros that
// change with the place of use (__FILE__, __LINE__, __COUNTER__).
func dependsOnUse(body string) bool {
	for _, w := range []string{"__FILE__", "__LINE__", "__DATE__", "__TIME__", "__TIMESTAMP__", "__COUNTER__", "__BASE_FILE__", "__FILE_NAME__", "__INCLUDE_LEVEL__"} {
		if strings.Contains(body, w) {
			return true
		}
	}
	return false
}

// hostileNames are the names that a macro of a header must not keep when the probe's helper
// code is compiled after the header: the keywords of C++ and the names the helpers use.
var hostileNames = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`alignas alignof and and_eq asm auto bitand bitor bool break case catch char char8_t char16_t
		char32_t class compl concept const consteval constexpr constinit const_cast continue co_await co_return co_yield decltype
		default delete do double dynamic_cast else enum explicit export extern false float for friend goto if inline int long
		mutable namespace new noexcept not not_eq nullptr operator or or_eq private protected public register reinterpret_cast
		requires return short signed sizeof static static_assert static_cast struct switch template this thread_local throw true
		try typedef typeid typename union unsigned using virtual void volatile wchar_t while xor xor_eq
		std size_t uint64_t int64_t uint8_t string map array slice key val key_type mapped_type value_type enc size deref range_t arith str_of not_exact`) {
		m[w] = true
	}
	return m
}()

// isGuard says that a macro is an include guard (_STDIO_H, FOO_H_INCLUDED).
func isGuard(m *macro) bool {
	if m.Body != "" && m.Body != "1" {
		return false
	}
	n := strings.TrimRight(m.Name, "_")
	for _, suf := range []string{"_H", "_HPP", "_HXX", "_HH", "_INCLUDED", "_INCLUDE", "_H_INCLUDED"} {
		if strings.HasSuffix(n, suf) {
			return true
		}
	}
	return false
}

// balanced says that the brackets of a macro body match and that it has no
// statement separator: the probe puts the body inside parentheses, and a body
// such as "{" would confuse the compiler's recovery for the lines after it.
func balanced(body string) bool {
	var stack []byte
	inStr := false
	var quote byte
	for i := 0; i < len(body); i++ {
		c := body[i]
		if inStr {
			if c == '\\' {
				i++
			} else if c == quote {
				inStr = false
			}
			continue
		}
		switch c {
		case '"', '\'':
			inStr, quote = true, c
		case '(', '[', '{':
			stack = append(stack, c)
		case ')', ']', '}':
			if len(stack) == 0 {
				return false
			}
			open := stack[len(stack)-1]
			if !(open == '(' && c == ')' || open == '[' && c == ']' || open == '{' && c == '}') {
				return false
			}
			stack = stack[:len(stack)-1]
		case ';', '#':
			return false
		}
	}
	return !inStr && len(stack) == 0
}

// probeConstants asks the compiler for the type and value of each macro and
// of the const variables in cands, with the probe of step 12.2. includes are
// the #include texts of the header.
func (g *gen) probeConstants(ctx context.Context, macros []*macro, vars []*gdecl) (*constants, error) {
	res := &constants{}
	res.skipped = append(res.skipped, g.skippedMacros...)
	if len(macros)+len(vars) == 0 {
		return res, nil
	}
	includes := []string{g.cfg.includeText()}
	var undef []string
	for _, m := range g.pp.Macros {
		if hostileNames[m.Name] {
			undef = append(undef, m.Name)
		}
	}
	if len(undef) > 0 {
		// the probe compiles its helpers after the header: take the macros that would break them away
		var w strings.Builder
		w.WriteString("#include " + g.cfg.includeText() + "\n")
		for _, n := range undef {
			w.WriteString("#undef " + n + "\n")
		}
		path, err := g.cfg.tool().source("probe_wrap.h", w.String())
		if err != nil {
			return nil, err
		}
		includes = []string{`"` + path + `"`}
	}
	kinds := map[string]interop.Kind{}
	var refs []*collect.Ref
	byKey := map[string]int{} // key -> index into entries
	type entry struct {
		m *macro
		v *gdecl
	}
	var entries []entry
	for _, m := range macros {
		p := &collect.Path{Parts: []string{m.Name}}
		kinds[p.KindPath()] = interop.Macro
		refs = append(refs, &collect.Ref{Expr: p, WantValue: true})
		byKey[p.String()] = len(entries)
		entries = append(entries, entry{m: m})
	}
	for _, d := range vars {
		p := &collect.Path{Ns: nsOf(d.scope), Parts: []string{d.name}}
		if len(d.scope) == 0 {
			p.Ns = ""
		}
		kinds[p.KindPath()] = interop.Value
		refs = append(refs, &collect.Ref{Expr: p, WantValue: true})
		byKey[p.String()] = len(entries)
		entries = append(entries, entry{v: d})
	}
	set := collect.NewSet(refs...)
	src := probe.Generate(set, probe.GenConfig{Includes: includes, Kinds: kinds})
	var runner probe.Runner
	tab, err := runner.Run(ctx, probe.Config{CXX: g.cfg.cxx, Dir: g.cfg.Dir, Flags: g.cfg.clangFlags(), Timeout: g.cfg.Timeout, CacheDir: g.cfg.CacheDir}, src)
	if err != nil {
		return nil, fmt.Errorf("probing the macros: %w", err)
	}
	g.probeCached = tab.Cached
	structNames := map[string]*macro{}
	for _, ref := range set.Refs {
		r := tab.Results[ref.ID]
		idx, ok := byKey[ref.Key]
		if !ok {
			continue
		}
		e := entries[idx]
		if e.m != nil {
			m := e.m
			if !r.OK {
				res.skipped = append(res.skipped, skippedMacro{m.Name, "", "not an expression the compiler accepts"})
				continue
			}
			if r.Value != nil {
				lit, ok := constLiteral(r.Value)
				if !ok {
					res.skipped = append(res.skipped, skippedMacro{m.Name, "", "value has no Go constant"})
					continue
				}
				if r.Value.Kind == "float" && r.Type == "float" {
					lit = strconv.FormatFloat(float64(float32(r.Value.Float)), 'g', -1, 32)
					if !strings.ContainsAny(lit, ".eEn") {
						lit += ".0"
					}
				}
				if r.Value.Kind == "string" && strings.Contains(r.Value.Str, "gp_probe.cpp") {
					res.skipped = append(res.skipped, skippedMacro{m.Name, "", "its value depends on where it is used"})
					continue
				}
				c := &constant{Name: m.Name, Lit: lit, Macro: true, Seq: m.Seq}
				if r.Value.Kind == "int" {
					c.Type = g.enumTypeName(r)
				}
				res.consts = append(res.consts, c)
				continue
			}
			if d := g.recordNamed(r.Type); d != nil {
				structNames[m.Name] = m
				continue
			}
			why := "not a constant"
			if strings.Contains(r.Type, "*") {
				why = "a pointer, not a Go constant"
			}
			res.skipped = append(res.skipped, skippedMacro{m.Name, "", why})
			continue
		}
		// a const variable
		if !r.OK || r.Value == nil {
			continue
		}
		lit, ok := constLiteral(r.Value)
		if !ok {
			continue
		}
		ty, err := g.goType(strings.TrimPrefix(e.v.node.qual(), "const "), e.v.scope)
		if err != nil {
			continue
		}
		res.consts = append(res.consts, &constant{Name: e.v.qname(), Lit: lit, Type: ty, Seq: e.v.seq, Decl: e.v})
		e.v.done = true // written as a constant
	}
	if len(structNames) > 0 {
		if err := g.structMacros(ctx, res, structNames); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// nsOf returns the namespace of a scope as collect.Path wants it.
func nsOf(scope []string) string {
	if len(scope) == 0 {
		return ""
	}
	return "::" + strings.Join(scope, "::")
}

// constLiteral writes a constant value as a Go literal.
func constLiteral(c *probe.Const) (string, bool) {
	switch c.Kind {
	case "int":
		return c.Int.String(), true
	case "bool":
		if c.Int.Sign() == 0 {
			return "false", true
		}
		return "true", true
	case "float":
		if math.IsInf(c.Float, 0) || math.IsNaN(c.Float) || c.Float == 0 && math.Signbit(c.Float) {
			return "", false
		}
		return floatLiteral(c.Float), true
	case "string":
		return strconv.Quote(c.Str), true
	}
	return "", false
}

// floatLiteral32 writes a float32 so that Go reads it back as the same float32.
func floatLiteral32(f float32) string {
	s := strconv.FormatFloat(float64(f), 'g', -1, 32)
	if !strings.ContainsAny(s, ".eEn") {
		s += ".0"
	}
	return s
}

// floatLiteral writes a float so that Go reads it back as a float constant.
func floatLiteral(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eEn") {
		s += ".0"
	}
	return s
}

// enumTypeName returns the Go name of the enum that a macro's value has the
// type of, or "".
func (g *gen) enumTypeName(r probe.Result) string {
	d := g.recordNamed(r.Type)
	if d == nil || d.kind != "enum" {
		return ""
	}
	return g.ref(d)
}

// recordNamed returns the record or enum that the canonical type text names.
func (g *gen) recordNamed(typ string) *gdecl {
	t := strings.TrimSpace(typ)
	for _, kw := range []string{"const ", "struct ", "union ", "class ", "enum "} {
		t = strings.TrimPrefix(t, kw)
	}
	d := g.tab.types[t]
	if d == nil {
		return nil
	}
	if d.into != nil {
		d = d.into
	}
	if d.kind == "record" || d.kind == "enum" {
		return d
	}
	return nil
}
