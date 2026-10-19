package bindgen

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
)

// structMacros reads the values of the macros that are structs (#define
// RAYWHITE CLITERAL(Color){ 245, 245, 245, 255 }) from the compiler's AST of
// "auto x = (RAYWHITE);" and writes them as Go composite literals.
func (g *gen) structMacros(ctx context.Context, res *constants, names map[string]*macro) error {
	order := sortedKeys(names)
	var src strings.Builder
	src.WriteString(g.cfg.includeLine())
	for i, name := range order {
		fmt.Fprintf(&src, "auto gpbg_x_%d = (%s);\n", i, name)
	}
	t := g.cfg.tool()
	path, err := t.source("structs.cpp", src.String())
	if err != nil {
		return err
	}
	vars := map[int]*jnode{}
	mode := []string{"-fsyntax-only", "-Xclang", "-ast-dump=json", "-Xclang", "-ast-dump-filter=gpbg_x_"}
	_, _, err = t.run(ctx, mode, path, func(r io.Reader) error {
		dec := json.NewDecoder(r)
		for {
			n := new(jnode)
			if err := dec.Decode(n); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			if n.Kind != "VarDecl" || !strings.HasPrefix(n.Name, "gpbg_x_") {
				continue
			}
			if i, err := strconv.Atoi(strings.TrimPrefix(n.Name, "gpbg_x_")); err == nil {
				vars[i] = n
			}
		}
	})
	if err != nil {
		return fmt.Errorf("reading the struct macros: %w", err)
	}
	for i, name := range order {
		m := names[name]
		v := vars[i]
		if v == nil || len(v.Inner) == 0 {
			res.skipped = append(res.skipped, skippedMacro{name, "", "value is not an expression"})
			continue
		}
		res.pending = append(res.pending, &pendingStruct{name: name, seq: m.Seq, node: &v.Inner[len(v.Inner)-1], typ: v.qual()})
	}
	return nil
}

// renderStructMacros writes the literals of the struct macros. It runs after the names
// are given, because a literal names enumerators and types.
func (g *gen) renderStructMacros(res *constants) {
	var traits []*jnode
	for _, p := range res.pending {
		collectTraits(p.node, &traits)
	}
	g.askTraits(traits)
	for _, p := range res.pending {
		lit, err := g.initExpr(p.node, nil)
		if err != nil {
			res.skipped = append(res.skipped, skippedMacro{p.name, "", scrub(err.Error())})
			continue
		}
		res.structs = append(res.structs, &structMacro{Name: p.name, Expr: lit, Type: p.typ, Seq: p.seq})
	}
}

// initExpr writes the value of an initializer expression as a Go expression.
// It reads literals, braces, casts, enumerators and the arithmetic that fold
// evaluates; anything else is an error, and the macro is listed as not translated.
func (g *gen) initExpr(n *jnode, scope []string) (string, error) {
	switch n.Kind {
	case "ImplicitCastExpr", "CStyleCastExpr", "CXXFunctionalCastExpr", "CXXStaticCastExpr":
		if n.CastKind == "NullToPointer" || n.CastKind == "NullToMemberPointer" {
			return "nil", nil
		}
		if len(n.Inner) == 1 && n.Inner[0].Kind != "InitListExpr" {
			if v, ok := fold(n); ok {
				return numLiteral(v)
			}
			if x := peelExpr(&n.Inner[0]); x.Kind == "DeclRefExpr" && x.ReferencedDecl != nil && x.ReferencedDecl.Kind == "EnumConstantDecl" {
				// an enumerator converted to a number (int, long, ...): its value, converted
				if val, ok := g.enumConst(x.ReferencedDecl.Name); ok {
					if v, ok := convert(cval{i: val}, typeNum(n.Type)); ok {
						return numLiteral(v)
					}
				}
			}
			if un, ok := g.enumUnder(n.qual()); ok && peelExpr(&n.Inner[0]).Kind != "DeclRefExpr" {
				// a number converted to an enum: the bits of its underlying type
				if v, ok := fold(&n.Inner[0]); ok {
					if c, ok := convert(v, un); ok {
						return numLiteral(c)
					}
				}
				return "", fmt.Errorf("a conversion that the view cannot fold")
			}
			if typeNum(n.Type).kind != notNum {
				// a conversion to a number that C leaves undefined, or that cannot be folded: the
				// value inside must not be written as if the conversion had not happened
				return "", fmt.Errorf("a conversion that the view cannot fold")
			}
		}
		fallthrough
	case "ExprWithCleanups", "ParenExpr", "MaterializeTemporaryExpr", "CXXBindTemporaryExpr", "CompoundLiteralExpr",
		"ConstantExpr", "SubstNonTypeTemplateParmExpr":
		if n.Kind == "ConstantExpr" && len(n.Inner) == 0 {
			if v, ok := n.valueString(); ok {
				return v, nil
			}
		}
		if len(n.Inner) != 1 {
			return "", fmt.Errorf("unsupported %s", n.Kind)
		}
		return g.initExpr(&n.Inner[0], scope)
	case "IntegerLiteral", "CharacterLiteral", "FloatingLiteral", "UnaryOperator", "BinaryOperator", "UnaryExprOrTypeTraitExpr":
		if v, ok := fold(n); ok {
			return numLiteral(v)
		}
	case "CXXBoolLiteralExpr":
		v, _ := n.valueString()
		return v, nil
	case "StringLiteral":
		v, _ := n.valueString()
		s, ok := decodeCString(v)
		if !ok {
			return "", fmt.Errorf("string %s", v)
		}
		return strconv.Quote(s), nil
	case "CXXNullPtrLiteralExpr", "GNUNullExpr":
		return "nil", nil
	case "ImplicitValueInitExpr":
		return g.zeroValue(n.qual(), scope)
	case "DeclRefExpr":
		if r := n.ReferencedDecl; r != nil && r.Kind == "EnumConstantDecl" {
			if v, ok := g.enumShort[r.Name]; ok {
				return v, nil
			}
		}
	case "InitListExpr":
		return g.listExpr(n, scope)
	}
	return "", fmt.Errorf("not a constant that the view can write (%s)", n.Kind)
}

func numLiteral(v cval) (string, error) {
	s, ok := v.literal()
	if !ok {
		return "", fmt.Errorf("a value that has no Go constant (negative zero, infinity or NaN)")
	}
	return s, nil
}

// A slot is a member that a braced initializer fills, in order.
type slot struct {
	name string // the Go name, "" for an anonymous union or struct
	typ  string // the Go type
	anon *jnode // the declaration of the anonymous member
	bf   numT   // for a bit field: its width and the signedness of its type
}

// slots lists the members that the braces of a record fill: the named fields (bit
// fields too), and the anonymous members; an unnamed bit field is padding.
func (g *gen) slots(rec *jnode, scope []string) []slot {
	var out []slot
	var lastAnon *jnode
	for i := range rec.Inner {
		c := &rec.Inner[i]
		switch {
		case (c.Kind == "CXXRecordDecl" || c.Kind == "RecordDecl") && c.Name == "" && !c.IsImplicit:
			lastAnon = c
		case c.Kind == "FieldDecl" && c.Name != "":
			ty, _ := g.goType(c.qual(), scope)
			if isUnnamedSpelling(c.qual()) && lastAnon != nil {
				out = append(out, slot{name: goIdent(c.Name), typ: "struct", anon: lastAnon})
			} else {
				sl := slot{name: goIdent(c.Name), typ: ty}
				if c.IsBitfield {
					if t := typeNum(c.Type); t.kind == intNum {
						if w, ok := constantValue(c); ok {
							if n, err := strconv.Atoi(w); err == nil && n > 0 && n < t.bits {
								sl.bf = numT{kind: intNum, bits: n, signed: t.signed}
							}
						}
					}
				}
				out = append(out, sl)
			}
		case c.Kind == "FieldDecl" && !c.IsBitfield && lastAnon != nil:
			out = append(out, slot{anon: lastAnon})
		}
	}
	return out
}

// listExpr writes a braced initializer of a struct, a union or an array.
func (g *gen) listExpr(n *jnode, scope []string) (string, error) {
	spelling := n.qual()
	ct, err := g.goType(spelling, scope)
	if err != nil {
		return "", err
	}
	list := n.Inner
	if len(n.ArrayFill) > 0 { // the filler, then the elements
		list = n.ArrayFill[1:]
	}
	if strings.HasPrefix(ct, "[") { // an array: Go allows fewer elements
		if unwritable(ct) {
			return "", fmt.Errorf("a 128-bit or long double element")
		}
		var elems []string
		for i := range list {
			s, err := g.initExpr(&list[i], scope)
			if err != nil {
				return "", err
			}
			elems = append(elems, s)
		}
		return ct + "{" + strings.Join(elems, ", ") + "}", nil
	}
	d := g.recordNamed(spelling)
	if d == nil || d.kind != "record" {
		return "", fmt.Errorf("braces for %s", scrub(spelling))
	}
	return g.recordLit(ct, d.node, d.tag, list, n.Field, declScope(d))
}

// recordLit writes the literal of a struct or union of type ct, declared by rec, from the
// elements of its braces.
func (g *gen) recordLit(ct string, rec *jnode, tag string, list []jnode, field *jref, scope []string) (string, error) {
	slots := g.slots(rec, scope)
	elem := func(s slot, e *jnode) (string, error) {
		if s.anon != nil {
			x := e
			for x.Kind != "InitListExpr" && len(x.Inner) == 1 {
				x = &x.Inner[0]
			}
			if x.Kind != "InitListExpr" {
				return "", fmt.Errorf("an anonymous member that is not braced")
			}
			body, _, _ := g.structBody(s.anon, scope, false, "")
			l := x.Inner
			if len(x.ArrayFill) > 0 {
				l = x.ArrayFill[1:]
			}
			return g.recordLit(body, s.anon, s.anon.TagUsed, l, x.Field, scope)
		}
		x := e
		for (x.Kind == "ImplicitCastExpr" || x.Kind == "ConstantExpr" || x.Kind == "ParenExpr") && len(x.Inner) == 1 {
			x = &x.Inner[0]
		}
		if x.Kind == "StringLiteral" && strings.HasPrefix(s.typ, "[") {
			lit, _ := x.valueString()
			str, ok := decodeCString(lit)
			if !ok {
				return "", fmt.Errorf("string %s", lit)
			}
			var bs []string
			for _, c := range []byte(str) {
				bs = append(bs, strconv.Itoa(int(int8(c))))
			}
			return s.typ + "{" + strings.Join(bs, ", ") + "}", nil
		}
		if unwritable(s.typ) {
			return "", fmt.Errorf("a 128-bit or long double field")
		}
		v, err := g.initExpr(e, scope)
		if err != nil {
			return "", err
		}
		if s.bf.kind == intNum {
			// a bit field keeps the low bits of the value
			if x, ok := new(big.Int).SetString(v, 0); ok {
				v = s.bf.wrap(x).String()
			}
		}
		if v == "0" && (strings.HasPrefix(s.typ, "*") || strings.HasPrefix(s.typ, "func(") || s.typ == "unsafe.Pointer") {
			v = "nil"
		}
		return v, nil
	}
	if tag == "union" {
		if len(list) != 1 || field == nil {
			return "", fmt.Errorf("union value")
		}
		for _, s := range slots {
			if s.name == goIdent(field.Name) && s.anon == nil {
				v, err := elem(s, &list[0])
				if err != nil {
					return "", err
				}
				return ct + "{" + s.name + ": " + v + "}", nil
			}
		}
		return "", fmt.Errorf("a union member that is not a field")
	}
	if len(list) > len(slots) {
		return "", fmt.Errorf("more values than fields")
	}
	var parts []string
	keyed := len(list) < len(slots)
	for i := range list {
		v, err := elem(slots[i], &list[i])
		if err != nil {
			return "", err
		}
		if keyed {
			if slots[i].name == "" {
				return "", fmt.Errorf("an anonymous member among fewer values")
			}
			v = slots[i].name + ": " + v
		}
		parts = append(parts, v)
	}
	return ct + "{" + strings.Join(parts, ", ") + "}", nil
}

// zeroValue writes the zero value of a type.
func (g *gen) zeroValue(spelling string, scope []string) (string, error) {
	ty, err := g.goType(spelling, scope)
	if err != nil {
		return "", err
	}
	switch {
	case strings.HasPrefix(ty, "*"), strings.HasPrefix(ty, "func("), ty == "unsafe.Pointer":
		return "nil", nil
	case ty == "C.bool":
		return "false", nil
	case strings.HasPrefix(ty, "C.") && !strings.HasPrefix(ty, "C.va_list"), strings.HasPrefix(ty, "int"), strings.HasPrefix(ty, "uint"):
		return "0", nil
	case strings.HasPrefix(ty, "["), g.recordNamed(spelling) != nil:
		if d := g.recordNamed(spelling); d != nil && d.kind == "enum" {
			return "0", nil
		}
		return ty + "{}", nil
	}
	return "", fmt.Errorf("zero value of %s", spelling)
}

// decodeCString reads a C string literal as clang prints it in the AST
// ("a\n\x41\101"), without the escapes' meaning for wide strings.
func decodeCString(lit string) (string, bool) {
	lit = strings.TrimSpace(lit)
	if len(lit) < 2 || lit[0] != '"' || lit[len(lit)-1] != '"' {
		return "", false
	}
	s := lit[1 : len(lit)-1]
	var out []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' {
			out = append(out, c)
			continue
		}
		i++
		if i >= len(s) {
			return "", false
		}
		switch e := s[i]; e {
		case 'n':
			out = append(out, '\n')
		case 't':
			out = append(out, '\t')
		case 'r':
			out = append(out, '\r')
		case 'a':
			out = append(out, 7)
		case 'b':
			out = append(out, 8)
		case 'f':
			out = append(out, 12)
		case 'v':
			out = append(out, 11)
		case 'e':
			out = append(out, 27)
		case '\\', '\'', '"', '?':
			out = append(out, e)
		case 'x':
			j := i + 1
			for j < len(s) && isHex(s[j]) {
				j++
			}
			v, err := strconv.ParseUint(s[i+1:j], 16, 32)
			if err != nil {
				return "", false
			}
			out = append(out, byte(v))
			i = j - 1
		case '0', '1', '2', '3', '4', '5', '6', '7':
			j := i
			for j < len(s) && j < i+3 && s[j] >= '0' && s[j] <= '7' {
				j++
			}
			v, _ := strconv.ParseUint(s[i:j], 8, 32)
			out = append(out, byte(v))
			i = j - 1
		default:
			return "", false
		}
	}
	return string(out), true
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// peelExpr strips parentheses and implicit casts that keep the value.
func peelExpr(n *jnode) *jnode {
	for (n.Kind == "ParenExpr" || n.Kind == "ImplicitCastExpr" && (n.CastKind == "NoOp" || n.CastKind == "LValueToRValue")) && len(n.Inner) == 1 {
		n = &n.Inner[0]
	}
	return n
}

// enumConst returns the value of the enumerator with the name.
func (g *gen) enumConst(name string) (*big.Int, bool) {
	for _, d := range g.tab.all {
		if d.kind != "enum" {
			continue
		}
		for _, e := range enumValues(d.node) {
			if e.Name == name {
				return e.Value, true
			}
		}
	}
	return nil, false
}

// unwritable says that a field type is one whose constants the view cannot write: a 128-bit
// integer (a [16]byte there) and a long double (a Go constant has double precision).
func unwritable(typ string) bool {
	return strings.HasSuffix(typ, "[16]byte") || strings.HasSuffix(typ, "C.longdouble")
}

// enumUnder returns the integer type under the enum that the spelling names: the declared one,
// else the one the view gives it (unsigned int unless a value is negative or too large).
func (g *gen) enumUnder(spelling string) (numT, bool) {
	d := g.recordNamed(spelling)
	if d == nil || d.kind != "enum" {
		return numT{}, false
	}
	if f := d.node.FixedUnderlying; f != nil {
		if t := typeNum(f); t.kind == intNum {
			return t, true
		}
		return numT{}, false
	}
	switch g.enumUnderlying(d, enumValues(d.node)) {
	case "C.uint":
		return numT{kind: intNum, bits: 32}, true
	case "C.int":
		return numT{kind: intNum, bits: 32, signed: true}, true
	case "C.ulong":
		return numT{kind: intNum, bits: 64}, true
	case "C.long":
		return numT{kind: intNum, bits: 64, signed: true}, true
	}
	return numT{}, false
}

// collectTraits lists the sizeof(type) and alignof(type) nodes under n.
func collectTraits(n *jnode, out *[]*jnode) {
	if n.Kind == "UnaryExprOrTypeTraitExpr" && (n.Name == "sizeof" || n.Name == "alignof") && n.ArgType != nil && len(n.Inner) == 0 {
		*out = append(*out, n)
	}
	for i := range n.Inner {
		collectTraits(&n.Inner[i], out)
	}
	for i := range n.ArrayFill {
		collectTraits(&n.ArrayFill[i], out)
	}
}

// askTraits asks the compiler for the values of the sizeof and alignof nodes, in one run, and
// stores each in its node (a node the compiler does not answer for keeps no value: the macro is listed).
func (g *gen) askTraits(ns []*jnode) {
	if len(ns) == 0 {
		return
	}
	var src strings.Builder
	src.WriteString(g.cfg.includeLine())
	src.WriteString("template <unsigned long N> struct gpbg_n {};\n")
	for i, n := range ns {
		fmt.Fprintf(&src, "using gpbg_t_%d = gpbg_n<%s(%s)>;\n", i, n.Name, n.ArgType.QualType)
	}
	t := g.cfg.tool()
	path, err := t.source("traits.cpp", src.String())
	if err != nil {
		return
	}
	got := map[string]string{}
	mode := []string{"-fsyntax-only", "-Xclang", "-ast-dump=json", "-Xclang", "-ast-dump-filter=gpbg_t_"}
	t.run(g.ctx, mode, path, func(r io.Reader) error {
		dec := json.NewDecoder(r)
		for {
			n := new(jnode)
			if err := dec.Decode(n); err != nil {
				return nil
			}
			if n.Kind == "TypeAliasDecl" {
				if v, ok := constantValue(n); ok {
					got[n.Name] = v
				}
			}
		}
	})
	for i, n := range ns {
		if v, ok := got[fmt.Sprintf("gpbg_t_%d", i)]; ok {
			n.Value = []byte(strconv.Quote(v))
		}
	}
}
