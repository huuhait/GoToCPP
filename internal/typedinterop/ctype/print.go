package ctype

import (
	"strconv"
	"strings"
)

// String writes the type the way clang prints it ("const char *const",
// "int (*)(int, long)", "int (&)[4]", "std::vector<std::vector<int>>").
func (t *Type) String() string {
	if t == nil {
		return ""
	}
	if t.Pack {
		c := *t
		c.Pack = false
		return c.String() + "..."
	}
	var d string    // the declarator text so far, built from the outermost part inwards
	ptrish := false // d starts with a pointer, reference or pointer to member
	cur := t
	for cur.IsDeclarator() {
		switch cur.Kind {
		case Pointer:
			d = "*" + cvSuffix(cur) + spaceIf(cur, d) + d
			ptrish = true
		case LRef:
			d = "&" + d
			ptrish = true
		case RRef:
			d = "&&" + d
			ptrish = true
		case MemberPtr:
			d = cur.Class.String() + "::*" + cvSuffix(cur) + spaceIf(cur, d) + d
			ptrish = true
		case Array:
			d = group(d, ptrish) + "[" + arrayLen(cur) + "]"
			ptrish = false
		case Func:
			d = group(d, ptrish) + "(" + params(cur) + ")" + funcQuals(cur)
			ptrish = false
			if cur.Trailing {
				d += " -> " + cur.Elem.String()
				cur = &Type{Kind: Named, Path: []Seg{{Name: "auto"}}}
				continue
			}
		}
		cur = cur.Elem
		if cur == nil {
			return "<invalid>"
		}
	}
	leaf := leafString(cur)
	switch {
	case d == "":
		return leaf
	case d[0] == '[':
		return leaf + d
	}
	return leaf + " " + d
}

// spaceIf is the space that clang puts between a qualified pointer and the
// declarator after it ("char *const *").
func spaceIf(p *Type, d string) string {
	if d == "" || (!p.Const && !p.Volatile && !p.Restrict) {
		return ""
	}
	if d[0] == '[' {
		return ""
	}
	return " "
}

func cvSuffix(t *Type) string {
	s := ""
	if t.Const {
		s += "const"
	}
	if t.Volatile {
		if s != "" {
			s += " "
		}
		s += "volatile"
	}
	if t.Restrict {
		if s != "" {
			s += " "
		}
		s += "__restrict"
	}
	return s
}

// group parenthesizes a declarator that starts with a pointer, a reference or
// a pointer to member when an array or function part follows it.
func group(d string, ptrish bool) string {
	if ptrish {
		return "(" + d + ")"
	}
	return d
}

func arrayLen(t *Type) string {
	if t.Len >= 0 {
		return strconv.FormatInt(t.Len, 10)
	}
	return t.LenExpr
}

func params(f *Type) string {
	var ps []string
	for _, p := range f.Params {
		ps = append(ps, p.String())
	}
	if f.Variadic {
		ps = append(ps, "...")
	}
	return strings.Join(ps, ", ")
}

func funcQuals(f *Type) string {
	s := ""
	if f.ConstQ {
		s += " const"
	}
	if f.VolatileQ {
		s += " volatile"
	}
	if f.RefQual != "" {
		s += " " + f.RefQual
	}
	if f.Attr != "" {
		s += " " + f.Attr
	}
	if f.Noexcept != "" {
		s += " " + f.Noexcept
	}
	return s
}

// leafString writes a leaf with its qualifiers.
func leafString(t *Type) string {
	var b strings.Builder
	if t.Const {
		b.WriteString("const ")
	}
	if t.Volatile {
		b.WriteString("volatile ")
	}
	if t.Restrict {
		b.WriteString("__restrict ")
	}
	switch t.Kind {
	case Builtin, Special:
		b.WriteString(t.Name)
	case Unnamed:
		b.WriteString("(" + t.Name + ")")
	case Named:
		if t.Tag != "" {
			b.WriteString(t.Tag + " ")
		}
		if t.Global {
			b.WriteString("::")
		}
		for i, s := range t.Path {
			if i > 0 {
				b.WriteString("::")
			}
			b.WriteString(s.Name)
			if s.HasArgs {
				b.WriteByte('<')
				for j, a := range s.Args {
					if j > 0 {
						b.WriteString(", ")
					}
					if a.Type != nil {
						b.WriteString(a.Type.String())
					} else {
						b.WriteString(a.Expr)
					}
					if a.Pack {
						b.WriteString("...")
					}
				}
				b.WriteByte('>')
			}
		}
	default:
		b.WriteString("<invalid>")
	}
	return b.String()
}
