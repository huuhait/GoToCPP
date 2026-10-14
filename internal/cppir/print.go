package cppir

import (
	"fmt"
	"strings"
)

// Print returns the C++ of the translation unit f. out is the absolute path of the file that
// the C++ goes to, or "" if it isn't known: the lines with the zero Pos then get #line
// directives with their own lines in that file (see the package doc).
func Print(f *File, out string) []byte {
	p := &printer{}
	p.w.out = out
	p.decls(f.Decls)
	return p.w.buf.Bytes()
}

// A printer writes a tree to its writer.
type printer struct {
	w writer
}

// line writes one line of text mapped to pos (a plain line for the zero Pos).
func (p *printer) line(pos Pos, text string) {
	p.w.line(pos, text)
}

// ---- Declarations ----

func (p *printer) decls(list []Decl) {
	for _, d := range list {
		p.decl(d)
	}
}

func (p *printer) decl(d Decl) {
	switch d := d.(type) {
	case *Include:
		if d.System {
			p.w.plain("#include <" + d.Path + ">")
		} else {
			p.w.plain("#include " + CString(d.Path))
		}
	case *Namespace:
		name := ""
		if d.Name != "" {
			name = " " + d.Name // an anonymous namespace has none
		}
		p.line(d.Pos, "namespace"+name+" {")
		p.decls(d.Decls)
		p.line(d.End, "} // namespace"+name)
	case *Func:
		p.template(d.Pos, d.Template)
		head := words(d.Specs)
		if d.Result != nil {
			head += TypeString(d.Result) + " "
		}
		head += d.Name + "(" + p.params(d.Params) + ")"
		if len(d.Quals) > 0 {
			head += " " + strings.Join(d.Quals, " ")
		}
		if d.Body == nil {
			p.line(d.Pos, head+";")
			return
		}
		p.line(d.Pos, head+" {")
		p.indented(d.Body.Stmts)
		p.line(d.Body.End, "}")
	case *Struct:
		p.template(d.Pos, d.Template)
		head := "struct " + d.Name
		if d.Forward {
			p.line(d.Pos, head+";")
			return
		}
		if len(d.Bases) > 0 {
			bases := make([]string, len(d.Bases))
			for i, b := range d.Bases {
				bases[i] = TypeString(b)
			}
			head += " : " + strings.Join(bases, ", ")
		}
		p.line(d.Pos, head+" {")
		p.w.indent()
		p.decls(d.Members)
		p.w.dedent()
		p.line(d.End, "};")
	case *Access:
		p.w.dedent()
		p.line(d.Pos, d.Name+":")
		p.w.indent()
	case *Using:
		if d.Type != nil {
			p.line(d.Pos, "using "+d.Name+" = "+TypeString(d.Type)+";")
		} else {
			p.line(d.Pos, "using "+d.Name+";")
		}
	case *VarDecl:
		p.line(d.Pos, p.inline(d))
	case *Comment:
		p.line(d.Pos, "// "+d.Text)
	case *Blank:
		p.w.blank()
	default:
		panic(fmt.Sprintf("cppir: unknown declaration %T", d))
	}
}

// template writes the template head line, if t is not nil.
func (p *printer) template(pos Pos, t *Template) {
	if t != nil {
		p.line(pos, "template <"+p.params(t.Params)+">")
	}
}

// params returns the parameters, joined with ", ".
func (p *printer) params(list []Param) string {
	parts := make([]string, len(list))
	for i, pa := range list {
		parts[i] = p.param(pa)
	}
	return strings.Join(parts, ", ")
}

func (p *printer) param(pa Param) string {
	s := TypeString(pa.Type)
	if pa.Pack {
		s += "..."
	}
	if pa.Name != "" {
		s += " " + pa.Name
	}
	return s
}

// words returns the words, each followed by a space.
func words(list []string) string {
	var b strings.Builder
	for _, w := range list {
		b.WriteString(w)
		b.WriteByte(' ')
	}
	return b.String()
}

// ---- Statements ----

// indented writes the statements one level deeper.
func (p *printer) indented(list []Stmt) {
	p.w.indent()
	p.stmts(list)
	p.w.dedent()
}

func (p *printer) stmts(list []Stmt) {
	for _, s := range list {
		p.stmt(s, "", s.position())
	}
}

// multiline reports whether the statement s goes on over more than one line.
func multiline(s Stmt) bool {
	switch s := s.(type) {
	case *Block:
		return !s.Inline
	case *Scope, *Switch, *Try:
		return true
	case *If:
		b, ok := s.Then.(*Block)
		return ok && !b.Inline || s.Else != nil && multiline(s.Else)
	case *For:
		return !s.Body.Inline
	case *While:
		return !s.Body.Inline
	case *Line:
		return len(s.Stmts) > 0 && multiline(s.Stmts[len(s.Stmts)-1])
	}
	return false
}

// stmt writes the statement s. Its first line starts with lead and is mapped to at (the place
// of the Line that s ends, or s's own).
func (p *printer) stmt(s Stmt, lead string, at Pos) {
	if _, ok := s.(*Blank); ok {
		p.w.blank()
		return
	}
	if c, ok := s.(*Comment); ok {
		p.line(at, lead+"// "+c.Text) // nothing follows it on its line
		return
	}
	if !multiline(s) {
		p.line(at, lead+p.inline(s))
		return
	}
	switch s := s.(type) {
	case *Line:
		last := len(s.Stmts) - 1
		p.stmt(s.Stmts[last], lead+p.joined(s.Stmts[:last], true), at)
	case *Block:
		p.line(at, lead+"{")
		p.indented(s.Stmts)
		p.line(s.End, "}")
	case *Scope:
		p.scope(s, lead, at, false)
	case *If:
		p.ifStmt(s, lead, at)
	case *For:
		p.line(at, lead+p.forHead(s)+" {")
		p.indented(s.Body.Stmts)
		p.line(s.Body.End, "}")
	case *While:
		p.line(at, lead+"while ("+p.expr(s.Cond)+") {")
		p.indented(s.Body.Stmts)
		p.line(s.Body.End, "}")
	case *Switch:
		p.line(at, lead+"switch ("+p.expr(s.Tag)+") {")
		for _, c := range s.Cases {
			p.line(c.Pos, p.caseLabel(c)+" {")
			p.indented(c.Body)
			if c.Break {
				p.line(c.End, "} break;")
			} else {
				p.line(c.End, "}")
			}
		}
		p.line(s.End, "}")
	case *Try:
		p.line(at, lead+"try {")
		p.indented(s.Body.Stmts)
		end := s.Body.End
		for _, c := range s.Catches {
			p.line(c.Pos, "} catch ("+p.catchParam(c)+") {")
			p.indented(c.Body.Stmts)
			end = c.Body.End
		}
		p.line(end, "}")
	default:
		panic(fmt.Sprintf("cppir: unknown statement %T", s))
	}
}

// joined returns the statements inline, joined with spaces, and with a space at the end if
// trailing and there are any.
func (p *printer) joined(list []Stmt, trailing bool) string {
	parts := make([]string, len(list))
	for i, s := range list {
		parts[i] = p.inline(s)
	}
	s := strings.Join(parts, " ")
	if trailing && s != "" {
		s += " "
	}
	return s
}

// scope writes a Scope. With owe, it doesn't write its closing brace: it returns the number of
// braces that its caller, the Scope around it, closes on its own closing line (see Scope).
func (p *printer) scope(s *Scope, lead string, at Pos, owe bool) int {
	open := "{"
	if len(s.Head) > 0 {
		open += " " + p.joined(s.Head, false)
	}
	p.line(at, lead+open)
	p.w.indent()
	braces := 1
	for i, st := range s.Stmts {
		if i == len(s.Stmts)-1 {
			if inner, innerLead, innerAt := trailingScope(st); inner != nil {
				braces += p.scope(inner, p.joined(innerLead, true), innerAt, true)
				break
			}
		}
		p.stmt(st, "", st.position())
	}
	p.w.dedent()
	if owe {
		return braces
	}
	p.line(s.End, strings.TrimSuffix(strings.Repeat("} ", braces), " "))
	return 0
}

// trailingScope returns the Scope that the statement s is or ends with (a Line), the
// statements of the Line in front of it, and the place of its first line; or nil.
func trailingScope(s Stmt) (*Scope, []Stmt, Pos) {
	switch s := s.(type) {
	case *Scope:
		return s, nil, s.Pos
	case *Line:
		if n := len(s.Stmts); n > 0 {
			if sc, ok := s.Stmts[n-1].(*Scope); ok {
				return sc, s.Stmts[:n-1], s.Pos
			}
		}
	}
	return nil, nil, Pos{}
}

// ifStmt writes an If that goes on over more lines (multiline): its Then is a block that is
// not inline, or its Else goes on over more lines. Then a Then that is inline, or not a
// block, is written as a block over several lines too.
func (p *printer) ifStmt(s *If, lead string, at Pos) {
	p.line(at, lead+"if ("+p.expr(s.Cond)+") {")
	then := thenBlock(s)
	p.indented(then.Stmts)
	end := then.End
	for e := s.Else; ; {
		switch x := e.(type) {
		case nil:
			p.line(end, "}")
			return
		case *If:
			if multiline(x) {
				b := thenBlock(x)
				p.line(x.Pos, "} else if ("+p.expr(x.Cond)+") {")
				p.indented(b.Stmts)
				end, e = b.End, x.Else
				continue
			}
		case *Block:
			if !x.Inline {
				p.line(x.Pos, "} else {")
				p.indented(x.Stmts)
				p.line(x.End, "}")
				return
			}
		}
		p.line(end, "} else "+p.inline(e))
		return
	}
}

// thenBlock returns the Then of s as a block (a statement that is not one is the only
// statement of a block without an End: it is followed by an else).
func thenBlock(s *If) *Block {
	if b, ok := s.Then.(*Block); ok {
		return b
	}
	return &Block{Stmts: []Stmt{s.Then}}
}

func (p *printer) forHead(s *For) string {
	head := "for ("
	if s.Init != nil {
		head += p.inline(s.Init)
	} else {
		head += ";"
	}
	if s.Cond != nil {
		head += " " + p.expr(s.Cond)
	}
	head += ";"
	if s.Post != nil {
		head += " " + p.expr(s.Post)
	}
	return head + ")"
}

func (p *printer) caseLabel(c *Case) string {
	if c.Values == nil {
		return "default:"
	}
	labels := make([]string, len(c.Values))
	for i, v := range c.Values {
		labels[i] = "case " + p.expr(v) + ":"
	}
	return strings.Join(labels, " ")
}

func (p *printer) catchParam(c *Catch) string {
	if c.Param == nil {
		return "..."
	}
	return p.param(*c.Param)
}

// inline returns the statement s on one line.
func (p *printer) inline(s Stmt) string {
	switch s := s.(type) {
	case *ExprStmt:
		return p.expr(s.X) + ";"
	case *VarDecl:
		return p.varDecl(s) + ";"
	case *Return:
		if s.X == nil {
			return "return;"
		}
		return "return " + p.expr(s.X) + ";"
	case *Break:
		return "break;"
	case *Continue:
		return "continue;"
	case *Goto:
		return "goto " + s.Label + ";"
	case *Label:
		return s.Name + ":;"
	case *Attr:
		return "[[" + s.Name + "]];"
	case *Comment:
		if strings.Contains(s.Text, "*/") {
			panic("cppir: */ in a comment that shares its line: " + s.Text)
		}
		return "/* " + s.Text + " */"
	case *Blank:
		return ""
	case *Block:
		return p.braces(s.Stmts)
	case *Scope:
		return p.braces(append(append([]Stmt(nil), s.Head...), s.Stmts...))
	case *Line:
		return p.joined(s.Stmts, false)
	case *If:
		text := "if (" + p.expr(s.Cond) + ") " + p.inline(s.Then)
		if s.Else != nil {
			text += " else " + p.inline(s.Else)
		}
		return text
	case *For:
		return p.forHead(s) + " " + p.braces(s.Body.Stmts)
	case *While:
		return "while (" + p.expr(s.Cond) + ") " + p.braces(s.Body.Stmts)
	case *Switch:
		parts := []string{"switch (" + p.expr(s.Tag) + ") {"}
		for _, c := range s.Cases {
			part := p.caseLabel(c) + " " + p.braces(c.Body)
			if c.Break {
				part += " break;"
			}
			parts = append(parts, part)
		}
		return strings.Join(append(parts, "}"), " ")
	case *Try:
		text := "try " + p.braces(s.Body.Stmts)
		for _, c := range s.Catches {
			text += " catch (" + p.catchParam(c) + ") " + p.braces(c.Body.Stmts)
		}
		return text
	}
	panic(fmt.Sprintf("cppir: unknown statement %T", s))
}

// braces returns the statements as an inline block: { a; b; }, or {} if there are none.
func (p *printer) braces(list []Stmt) string {
	if len(list) == 0 {
		return "{}"
	}
	return "{ " + p.joined(list, false) + " }"
}

// varDecl returns the declaration without its ;.
func (p *printer) varDecl(d *VarDecl) string {
	vars := make([]string, len(d.Vars))
	for i, v := range d.Vars {
		vars[i] = v.Name
		switch init := v.Init.(type) {
		case nil:
		case *InitList:
			if init.Type == nil {
				vars[i] += p.expr(init) // T x{a, b}
				break
			}
			vars[i] += " = " + p.expr(init)
		default:
			vars[i] += " = " + p.operand(init, PrecAssign)
		}
	}
	return words(d.Specs) + TypeString(d.Type) + " " + strings.Join(vars, ", ")
}

// ---- Expressions ----

// ExprString returns the C++ of the expression e, as Print writes it.
func ExprString(e Expr) string {
	var p printer
	return p.expr(e)
}

// TypeString returns the C++ of the type t, as Print writes it.
func TypeString(t Type) string {
	var p printer
	return p.typ(t)
}

// operand returns the C++ of e in a place that needs at least the precedence prec: in
// parentheses if e binds less tightly.
func (p *printer) operand(e Expr, prec int) string {
	s := p.expr(e)
	if Prec(e) < prec {
		return "(" + s + ")"
	}
	return s
}

// list returns the expressions, each as an operand of a list (a comma would split it), joined
// with ", ".
func (p *printer) list(list []Expr) string {
	parts := make([]string, len(list))
	for i, e := range list {
		parts[i] = p.operand(e, PrecAssign)
	}
	return strings.Join(parts, ", ")
}

func (p *printer) expr(e Expr) string {
	switch e := e.(type) {
	case *Name:
		return e.Name + p.targs(e.Args)
	case *Lit:
		return e.Text
	case *Call:
		return p.operand(e.Fun, PrecPostfix) + "(" + p.list(e.Args) + ")"
	case *Binary:
		prec := Prec(e)
		// Every C++ binary operator groups from left to right: a right operand of the same
		// precedence needs parentheses, a - (b - c).
		op := " " + e.Op + " "
		if e.Op == "," {
			op = ", "
		}
		return p.operand(e.X, prec) + op + p.operand(e.Y, prec+1)
	case *Unary:
		x := p.operand(e.X, PrecUnary)
		if (e.Op == "-" || e.Op == "+" || e.Op == "&") && strings.HasPrefix(x, e.Op) {
			x = "(" + x + ")" // - -x, not the decrement --x
		}
		return e.Op + x
	case *Postfix:
		return p.operand(e.X, PrecPostfix) + e.Op
	case *Index:
		return p.operand(e.X, PrecPostfix) + "[" + p.expr(e.Index) + "]"
	case *Member:
		op := "."
		if e.Arrow {
			op = "->"
		}
		return p.operand(e.X, PrecPostfix) + op + e.Name
	case *Paren:
		return "(" + p.expr(e.X) + ")"
	case *Assign:
		return p.operand(e.X, PrecAssign+1) + " " + e.Op + " " + p.operand(e.Y, PrecAssign)
	case *Cond:
		return p.operand(e.Cond, PrecLogOr) + " ? " + p.expr(e.X) + " : " + p.operand(e.Y, PrecAssign)
	case *Cast:
		return e.Kind + "<" + p.typ(e.Type) + ">(" + p.expr(e.X) + ")"
	case *InitList:
		s := "{" + p.list(e.Elems) + "}"
		if e.Type != nil {
			s = p.typ(e.Type) + s
		}
		return s
	case *Designated:
		return "." + e.Name + " = " + p.operand(e.X, PrecAssign)
	case *Lambda:
		var caps []string
		if e.Default != "" {
			caps = append(caps, e.Default)
		}
		for _, c := range e.Captures {
			cs := c.Name
			if c.ByRef {
				cs = "&" + cs
			}
			if c.Init != nil {
				cs += " = " + p.operand(c.Init, PrecAssign)
			}
			caps = append(caps, cs)
		}
		s := "[" + strings.Join(caps, ", ") + "](" + p.params(e.Params) + ")"
		if len(e.Specs) > 0 {
			s += " " + strings.Join(e.Specs, " ")
		}
		if e.Result != nil {
			s += " -> " + p.typ(e.Result)
		}
		return s + " " + p.braces(e.Body.Stmts)
	case *Expand:
		return p.operand(e.X, PrecPostfix) + "..."
	}
	panic(fmt.Sprintf("cppir: unknown expression %T", e))
}

// targs returns the template arguments <a, b>, or "" for nil.
func (p *printer) targs(args []Node) string {
	if args == nil {
		return ""
	}
	parts := make([]string, len(args))
	for i, a := range args {
		switch a := a.(type) {
		case Type:
			parts[i] = p.typ(a)
		case Expr:
			// A > would end the argument list: (a > b).
			parts[i] = p.operand(a, PrecShift+1)
		default:
			panic(fmt.Sprintf("cppir: a template argument %T", a))
		}
	}
	return "<" + strings.Join(parts, ", ") + ">"
}

func (p *printer) typ(t Type) string {
	switch t := t.(type) {
	case *TypeName:
		return t.Name + p.targs(t.Args)
	case *Ref:
		if t.RValue {
			return p.typ(t.Elem) + "&&"
		}
		return p.typ(t.Elem) + "&"
	case *Ptr:
		return p.typ(t.Elem) + "*"
	case *Const:
		if _, ok := t.Elem.(*Ptr); ok {
			return p.typ(t.Elem) + " const"
		}
		return "const " + p.typ(t.Elem)
	case *FuncType:
		parts := make([]string, len(t.Params))
		for i, pt := range t.Params {
			parts[i] = p.typ(pt)
		}
		return p.typ(t.Result) + "(" + strings.Join(parts, ", ") + ")"
	}
	panic(fmt.Sprintf("cppir: unknown type %T", t))
}
