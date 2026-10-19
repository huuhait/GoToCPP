package typedinterop

import (
	"context"
	"maps"
	"math/big"
	"runtime"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// Step 12.8 (gap E1, ADR-097): the values of the C/C++ names that a program uses where Go needs a
// constant. Constants is the function that check.Config.Constants takes: internal/check finds the
// names (before go/types, which cannot evaluate them), this package asks clang.
//
// The query is a set of references that are bare paths with WantValue, which is the 12.2
// constant probe: an integer through ConstantExpr, a double through bit_cast (the exact bits), a
// string literal read from the AST. The probe goes through the Runner, so the answers are cached on
// disk per reference under the contents of the headers (12.7), and the references that are not
// cached run in the batches of 12.7.

// Constants returns the function that gives check.Config.Constants the values of C/C++ names. The
// compiler is clang (opts.Clang or probe.FindClang); without clang, with headers that do not
// include, or for another target than this machine (the value of a macro is the host's, not the
// target's) it answers nothing and the names stay opaque.
func Constants(opts Options) check.ConstFunc {
	return func(ctx context.Context, q check.ConstQuery) (map[string]check.ConstAnswer, error) {
		return constValues(ctx, q, opts)
	}
}

func constValues(ctx context.Context, q check.ConstQuery, opts Options) (map[string]check.ConstAnswer, error) {
	if q.GOOS != "" && q.GOOS != runtime.GOOS || q.GOARCH != "" && q.GOARCH != runtime.GOARCH {
		return nil, nil
	}
	cxx := opts.Clang
	if cxx.Path == "" {
		var err error
		if cxx, err = probe.FindClang(ctx); err != nil {
			return nil, nil // no clang: opaque
		}
	}
	rn := opts.Runner
	if rn == nil {
		rn = new(probe.Runner)
	}
	// The collector's reference merges paths with the same C++ text. Two header packages of the same name
	// as a namespace (oop.scale and other.scale, where oop is a namespace) are not the same name in C++:
	// they are asked in groups in which no text is twice.
	var groups [][]check.CppPath
next:
	for _, p := range q.Paths {
		for gi, g := range groups {
			if !slices.ContainsFunc(g, func(o check.CppPath) bool { return pathOf(o).String() == pathOf(p).String() }) {
				groups[gi] = append(g, p)
				continue next
			}
		}
		groups = append(groups, []check.CppPath{p})
	}
	out := make(map[string]check.ConstAnswer, len(q.Paths))
	for _, g := range groups {
		part, err := constGroup(ctx, q, g, cxx, rn, opts)
		if err != nil {
			return nil, err
		}
		if part == nil {
			return nil, nil // the headers do not include: nothing can be asked
		}
		maps.Copy(out, part)
	}
	return out, nil
}

func pathOf(p check.CppPath) *collect.Path {
	return &collect.Path{Ns: p.Ns, Hdr: p.Hdr, Parts: p.Parts}
}

// constGroup asks the paths of one group (no two of the same C++ text). It returns nil, nil when the
// headers do not include (the namespace probe has the error).
func constGroup(ctx context.Context, q check.ConstQuery, paths []check.CppPath, cxx build.CXX, rn *probe.Runner, opts Options) (map[string]check.ConstAnswer, error) {
	refs := make([]*collect.Ref, 0, len(paths))
	for _, p := range paths {
		refs = append(refs, &collect.Ref{Expr: pathOf(p), WantValue: true})
	}
	set := collect.NewSet(refs...)
	kinds, includeErrs, err := probeKinds(ctx, cxx, q.Dir, q.Flags, q.Includes, set)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	if len(includeErrs) > 0 {
		return nil, nil
	}
	cfg := probe.Config{CXX: cxx, Dir: q.Dir, Flags: q.Flags, Timeout: opts.Timeout, MaxDecl: opts.MaxDecl, MaxJSON: opts.MaxJSON}
	tab, err := rn.Probe(ctx, cfg, set, probe.GenConfig{Includes: q.Includes, Kinds: kinds})
	if err != nil {
		return nil, err
	}
	// Macros whose value is the probe file's own (__LINE__, __FILE__, __COUNTER__, __DATE__, ...) are
	// not asked for in the program: only when a name is a macro.
	var volatile map[string]string
	for _, p := range paths {
		if kinds[pathOf(p).KindPath()] == interop.Macro {
			if volatile, err = rn.VolatileMacros(ctx, cfg, tab.Source); err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return nil, err
			}
			break
		}
	}
	out := make(map[string]check.ConstAnswer, len(paths))
	var enums []string // canonical names of the types that need to be looked up: an enumerator may be of a scoped enum
	type pending struct {
		key string
		ans check.ConstAnswer
		typ string
	}
	var later []pending
	for _, p := range paths {
		pp := pathOf(p)
		ref := set.Ref(pp.String())
		if ref == nil {
			continue
		}
		res := tab.Result(ref.ID)
		if res == nil {
			continue
		}
		ans, ok := answerOf(res, kinds[pp.KindPath()])
		if !ok {
			continue
		}
		if kinds[pp.KindPath()] == interop.Macro && len(p.Parts) == 1 {
			if why := volatile[p.Parts[0]]; why != "" {
				ans = check.ConstAnswer{NotConst: true, Why: "its value depends on where or when it is used (the macro expands to " + why + "), so the probe's value would not be the program's"}
			}
		}
		ans.Spelling = tab.Source.Spelling(ref.ID)
		if name, named := namedType(res.Base); ans.Kind != check.ConstNone && named {
			later = append(later, pending{p.Key(), ans, name})
			enums = append(enums, name)
			continue
		}
		out[p.Key()] = ans
	}
	if len(later) > 0 {
		scoped, err := scopedEnums(ctx, rn, cfg, q.Includes, enums)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		}
		for _, l := range later {
			if unscoped, known := scoped[l.typ]; known && unscoped {
				out[l.key] = l.ans
			}
			// A scoped enum is not an integer to C++ and its enumerator is no number to Go; a type that is
			// not an enum is not known: both stay as the user wrote them.
		}
	}
	return out, nil
}

// namedType returns the qualified name of the class or enum that the type text names, and whether
// it names one (not a builtin type, an array of characters or a pointer).
func namedType(typ string) (string, bool) {
	t, err := ctype.Parse(typ)
	if err != nil {
		return typ, true // unknown: look it up, and do not substitute when the lookup has no answer
	}
	if t.Kind != ctype.Named {
		return "", false
	}
	return t.QualName(), true
}

// scopedEnums asks the declarations of the types: for each, whether it is an enum that is not
// scoped (an enumerator of it converts to an integer).
func scopedEnums(ctx context.Context, rn *probe.Runner, cfg probe.Config, includes, types []string) (map[string]bool, error) {
	var qs []probe.DeclQuery
	seen := make(map[string]bool)
	for _, t := range types {
		if !seen[t] {
			seen[t] = true
			qs = append(qs, probe.DeclQuery{Kind: probe.DeclType, Name: t})
		}
	}
	tab, err := rn.Decls(ctx, cfg, probe.DeclConfig{Includes: includes}, qs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool)
	for _, q := range qs {
		canon := q.Name
		if c, ok := tab.Canon[q.Name]; ok {
			canon = c
		}
		if e := tab.Enums[canon]; e != nil {
			out[q.Name] = !e.Scoped
		}
	}
	return out, nil
}

// charTypes are the C++ types of the character literals ('a', L'a', u8'a', u'a', U'a').
var charTypes = map[string]bool{"char": true, "wchar_t": true, "char8_t": true, "char16_t": true, "char32_t": true}

// answerOf converts the probe's answer for a name. ok is false when the probe has no answer to give
// (the name is left as it is).
func answerOf(res *probe.Result, kind interop.Kind) (check.ConstAnswer, bool) {
	switch {
	case res.Skipped != "":
		return check.ConstAnswer{}, false
	case res.Value != nil:
		v := res.Value
		switch v.Kind {
		case "int":
			k := check.ConstInt
			if charTypes[unqualified(res.Base)] {
				k = check.ConstChar
			}
			return check.ConstAnswer{Kind: k, Int: new(big.Int).Set(v.Int)}, true
		case "bool":
			return check.ConstAnswer{Kind: check.ConstBool, Int: new(big.Int).Set(v.Int)}, true
		case "float":
			if f := v.Float; f != f || f > 1.7976931348623157e308 || f < -1.7976931348623157e308 {
				return check.ConstAnswer{NotConst: true, Why: "its value " + v.String() + " is not a number that a Go constant can have (Go constants are finite)"}, true
			}
			return check.ConstAnswer{Kind: check.ConstFloat, Float: v.Float}, true
		case "string":
			// The dump gives the text as JSON, where a byte that is not UTF-8 became U+FFFD: the length of the
			// literal (the array bound of its type) tells that the text is not the string.
			if n, ok := charArrayLen(res.Base); !ok || n-1 != len(v.Str) {
				return check.ConstAnswer{NotConst: true, Why: "the string has bytes that are not valid UTF-8, which the compiler's dump cannot give back exactly"}, true
			}
			return check.ConstAnswer{Kind: check.ConstString, Str: v.Str}, true
		}
		return check.ConstAnswer{}, false
	case res.OK && res.Form == probe.FormType:
		return check.ConstAnswer{NotConst: true, Why: "it names a type, not a value"}, true
	case res.OK && strings.Contains(res.ValueError, "'enc'"):
		return check.ConstAnswer{NotConst: true, Why: "its type " + res.Base + " is not a number, a character, a boolean or a string literal, the only things that Go constants can be"}, true
	case res.OK && res.ValueError != "":
		return check.ConstAnswer{NotConst: true, Why: "the C++ compiler cannot evaluate it at compile time (" + firstLine(res.ValueError) + ")"}, true
	case !res.OK && kind == interop.Macro:
		// The macro is there (the preprocessor lists it) and (NAME) does not compile: a function-like macro
		// is no name without its call, and a macro with a broken body is the compiler's to report.
		return check.ConstAnswer{NotConst: true, Why: "it is a macro that does not expand to an expression (it is empty, function-like or a statement): " + firstLine(res.Error)}, true
	}
	return check.ConstAnswer{}, false
}

// charArrayLen returns the bound of the type text "const char [10]" of a narrow string literal.
func charArrayLen(typ string) (int, bool) {
	t, err := ctype.Parse(typ)
	if err != nil || t.Kind != ctype.Array || t.Len < 1 || t.Elem == nil || t.Elem.Kind != ctype.Builtin || t.Elem.Name != "char" {
		return 0, false
	}
	return int(t.Len), true
}

// unqualified returns the type text without a leading const or volatile.
func unqualified(typ string) string {
	for {
		if rest, ok := strings.CutPrefix(typ, "const "); ok {
			typ = rest
		} else if rest, ok := strings.CutPrefix(typ, "volatile "); ok {
			typ = rest
		} else {
			return typ
		}
	}
}

func firstLine(s string) string {
	for i := range len(s) {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
