package ctype

import (
	"fmt"
	"strconv"
	"strings"
)

var sprintf = fmt.Sprintf

// Parse reads a C++ type in the spelling that clang prints (qualType,
// desugaredQualType, the text of a diagnostic).
func Parse(s string) (*Type, error) {
	p := &parser{s: s}
	t, err := p.typeName()
	if err != nil {
		return nil, err
	}
	p.skip()
	if p.i < len(p.s) {
		return nil, p.errf("unexpected %q after the type", p.s[p.i:min(p.i+12, len(p.s))])
	}
	return t, nil
}

// MustParse is Parse for tests and constants: it panics on an error.
func MustParse(s string) *Type {
	t, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return t
}

// maxDepth bounds the nesting of a type (templates, parentheses): a text
// from a header is not trusted to be shallow.
const maxDepth = 900

type parser struct {
	s      string
	i      int
	depth  int
	noPack bool              // inside a parameter list: a lone ... is C variadic
	fatal  error             // set when the nesting limit is hit: no fallback may hide it
	memo   map[int]memoEntry // typeName by start position
}

func (p *parser) errf(format string, a ...any) error {
	return &ParseError{Text: p.s, Pos: p.i, Msg: sprintf(format, a...)}
}

func (p *parser) tooDeep() error {
	p.fatal = p.errf("the type is nested too deeply")
	return p.fatal
}

func (p *parser) skip() {
	for p.i < len(p.s) && isSpace(p.s[p.i]) {
		p.i++
	}
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func isIdentChar(c byte) bool { return isIdentStart(c) || c >= '0' && c <= '9' }

// word returns the identifier at the current position, without consuming it.
func (p *parser) peekWord() string {
	p.skip()
	j := p.i
	if j >= len(p.s) || !isIdentStart(p.s[j]) {
		return ""
	}
	for j < len(p.s) && isIdentChar(p.s[j]) {
		j++
	}
	return p.s[p.i:j]
}

func (p *parser) eat(lit string) bool {
	p.skip()
	if strings.HasPrefix(p.s[p.i:], lit) {
		p.i += len(lit)
		return true
	}
	return false
}

func (p *parser) peek() byte {
	p.skip()
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

var builtinWords = map[string]bool{
	"void": true, "bool": true, "_Bool": true, "char": true, "short": true, "int": true, "long": true,
	"float": true, "double": true, "signed": true, "unsigned": true, "wchar_t": true,
	"char8_t": true, "char16_t": true, "char32_t": true, "__int128": true, "_Complex": true,
	"_Float16": true, "__fp16": true, "__bf16": true, "__float128": true, "_Float128": true,
	"__ibm128": true, "_Float32": true, "_Float64": true, "_Float32x": true, "_Float64x": true,
}

var cvWords = map[string]string{
	"const": "const", "volatile": "volatile", "__restrict": "restrict", "restrict": "restrict", "__restrict__": "restrict",
}

// typeName reads a type: specifiers, then a declarator.
//
// The result is remembered by position: a declarator that may be a group or a parameter list
// is read once to find out and once for good (isGroup, templateArg), and without the memory
// that doubles at every level of "int (S<int (S<...>)>)" (exponential time).
func (p *parser) typeName() (*Type, error) {
	if p.fatal != nil {
		return nil, p.fatal // the nesting limit was hit somewhere: nothing is read again
	}
	start := p.i
	if e, ok := p.memo[start]; ok {
		p.i = e.end
		return e.t, e.err
	}
	t, err := p.typeName0()
	if p.fatal == nil {
		if p.memo == nil {
			p.memo = make(map[int]memoEntry)
		}
		p.memo[start] = memoEntry{t, err, p.i}
	}
	return t, err
}

type memoEntry struct {
	t   *Type
	err error
	end int
}

func (p *parser) typeName0() (*Type, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxDepth {
		return nil, p.tooDeep()
	}
	base, err := p.specifiers()
	if err != nil {
		return nil, err
	}
	wrap, err := p.declarator()
	if err != nil {
		return nil, err
	}
	t := wrap(base)
	if err := p.check(t); err != nil {
		return nil, err
	}
	if p.peek() == '.' && strings.HasPrefix(p.s[p.i:], "...") && !p.noPack {
		p.i += 3
		t.Pack = true
	}
	return t, nil
}

// check rejects declarators that C++ does not have (a pointer to a reference,
// an array of functions, a function that returns an array), which Parse could
// not print back.
func (p *parser) check(t *Type) error {
	for ; t != nil && t.IsDeclarator(); t = t.Elem {
		e := t.Elem
		if e == nil {
			return p.errf("bad declarator")
		}
		switch {
		case e.IsRef() && t.Kind != Func:
			return p.errf("a reference cannot be the target of a pointer, reference, array or function")
		case t.Kind == Array && (e.Kind == Func):
			return p.errf("an array of functions")
		case t.Kind == Func && (e.Kind == Func || e.Kind == Array):
			return p.errf("a function that returns a function or an array")
		}
	}
	return nil
}

// specifiers reads the cv-qualifiers and the leaf of a type.
func (p *parser) specifiers() (*Type, error) {
	var cv struct{ c, v, r bool }
	var leaf *Type
	var bwords []string
	setCV := func(w string) {
		switch cvWords[w] {
		case "const":
			cv.c = true
		case "volatile":
			cv.v = true
		case "restrict":
			cv.r = true
		}
	}
	for {
		p.skip()
		if p.i >= len(p.s) {
			break
		}
		w := p.peekWord()
		if w != "" && cvWords[w] != "" {
			p.i += len(w)
			setCV(w)
			continue
		}
		if leaf == nil && w != "" && builtinWords[w] {
			p.i += len(w)
			bwords = append(bwords, w)
			continue
		}
		if leaf == nil && len(bwords) == 0 {
			if c := p.s[p.i]; w != "" || c == '(' || c == '<' || c == ':' {
				var err error
				leaf, err = p.leaf()
				if err != nil {
					return nil, err
				}
				continue
			}
		}
		break
	}
	if len(bwords) > 0 {
		if leaf != nil {
			return nil, p.errf("a builtin type next to a name")
		}
		name, ok := canonBuiltin(bwords)
		if !ok {
			return nil, p.errf("not a type: %q", strings.Join(bwords, " "))
		}
		leaf = &Type{Kind: Builtin, Name: name}
	}
	if leaf == nil {
		return nil, p.errf("expected a type")
	}
	leaf.Const, leaf.Volatile, leaf.Restrict = cv.c, cv.v, cv.r
	return leaf, nil
}

// canonBuiltin returns the canonical name of a builtin type written with the
// words ws in any order ("long unsigned int" is "unsigned long").
func canonBuiltin(ws []string) (string, bool) {
	var unsigned, signed, short, char, integer, double, float, cplx bool
	long := 0
	var other []string
	for _, w := range ws {
		switch w {
		case "unsigned":
			unsigned = true
		case "signed":
			signed = true
		case "short":
			short = true
		case "long":
			long++
		case "char":
			char = true
		case "int":
			integer = true
		case "double":
			double = true
		case "float":
			float = true
		case "_Complex":
			cplx = true
		default:
			other = append(other, w)
		}
	}
	prefix := ""
	if cplx {
		prefix = "_Complex "
	}
	if len(other) > 0 {
		if len(other) > 1 || unsigned && signed || short || long > 0 || char || integer || double || float {
			return "", false
		}
		if unsigned || signed {
			if other[0] == "__int128" {
				if unsigned {
					return prefix + "unsigned __int128", true
				}
				return prefix + "__int128", true
			}
			return "", false
		}
		if other[0] == "_Bool" {
			return "bool", true
		}
		return prefix + other[0], true
	}
	switch {
	case double:
		if long == 1 && !unsigned && !signed && !short && !char && !integer && !float {
			return prefix + "long double", true
		}
		if long == 0 && !unsigned && !signed && !short && !char && !integer && !float {
			return prefix + "double", true
		}
		return "", false
	case float:
		if long == 0 && !unsigned && !signed && !short && !char && !integer {
			return prefix + "float", true
		}
		return "", false
	case char:
		if long > 0 || short || integer {
			return "", false
		}
		switch {
		case unsigned && signed:
			return "", false
		case unsigned:
			return "unsigned char", true
		case signed:
			return "signed char", true
		}
		return "char", true
	}
	if unsigned && signed || short && long > 0 || long > 2 {
		return "", false
	}
	name := "int"
	switch {
	case short:
		name = "short"
	case long == 1:
		name = "long"
	case long == 2:
		name = "long long"
	}
	if unsigned {
		if name == "int" {
			return "unsigned int", true
		}
		return "unsigned " + name, true
	}
	if !integer && !signed && !short && long == 0 {
		return "", false
	}
	return name, true
}

// leaf reads a name, a type without a name, or a special word.
func (p *parser) leaf() (*Type, error) {
	p.skip()
	switch c := p.s[p.i]; {
	case c == '(':
		end := strings.IndexByte(p.s[p.i:], ')')
		if end < 0 {
			return nil, p.errf("unbalanced parenthesis")
		}
		inner := p.s[p.i+1 : p.i+end]
		switch {
		case inner == "anonymous namespace":
			return p.name("")
		case isUnnamedText(inner):
			p.i += end + 1
			if k := strings.Index(inner, " at "); k >= 0 {
				inner = inner[:k]
			}
			return &Type{Kind: Unnamed, Name: inner}, nil
		}
		return nil, p.errf("unexpected parenthesis")
	case c == '<':
		end := strings.IndexByte(p.s[p.i:], '>')
		if end < 0 {
			return nil, p.errf("unbalanced <")
		}
		t := &Type{Kind: Special, Name: p.s[p.i : p.i+end+1]}
		p.i += end + 1
		return t, nil
	}
	w := p.peekWord()
	switch w {
	case "struct", "class", "union", "enum", "typename":
		p.i += len(w)
		p.skip()
		if w == "enum" {
			if k := p.peekWord(); k == "class" || k == "struct" {
				p.i += len(k)
			}
		}
		if p.peek() == '(' {
			t, err := p.leaf()
			if err != nil {
				return nil, err
			}
			if t.Kind == Unnamed {
				return t, nil
			}
			return nil, p.errf("expected a name after %s", w)
		}
		return p.name(w)
	case "decltype", "__typeof__", "typeof", "__typeof", "_Atomic", "__underlying_type", "__remove_cvref", "__decay":
		return p.rawBuiltinCall(w)
	}
	if strings.HasPrefix(w, "__") && p.i+len(w) < len(p.s) && p.s[p.i+len(w)] == '(' {
		return p.rawBuiltinCall(w) // __remove_cv(char): a builtin type transformation
	}
	return p.name("")
}

// rawBuiltinCall reads word(...) as a special type with the text kept.
func (p *parser) rawBuiltinCall(w string) (*Type, error) {
	start := p.i
	p.i += len(w)
	if p.i >= len(p.s) || p.s[p.i] != '(' {
		// "auto"-like word, or one with a space before the parenthesis (clang prints
		// __remove_cv(char) without one; "X (A)" is a function type): a name
		p.i = start
		return p.name("")
	}
	depth := 0
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				p.i++
				return &Type{Kind: Special, Name: p.s[start:p.i]}, nil
			}
		}
		p.i++
	}
	return nil, p.errf("unbalanced parenthesis")
}

// name reads a qualified name with template arguments.
func (p *parser) name(tag string) (*Type, error) {
	t := &Type{Kind: Named, Tag: tag}
	if p.eat("::") {
		t.Global = true
	}
	for {
		p.skip()
		if p.i < len(p.s) && p.s[p.i] == '(' && len(t.Path) > 0 {
			// std::basic_string::(unnamed enum at /file.h:1:2)
			end := strings.IndexByte(p.s[p.i:], ')')
			if end > 0 {
				inner := p.s[p.i+1 : p.i+end]
				if isUnnamedText(inner) {
					if k := strings.Index(inner, " at "); k >= 0 {
						inner = inner[:k]
					}
					p.i += end + 1
					t.Path = append(t.Path, Seg{Name: "(" + inner + ")"})
					if !p.nameContinues() {
						break
					}
					continue
				}
			}
		}
		if p.i < len(p.s) && p.s[p.i] == '(' {
			// (anonymous namespace)
			if strings.HasPrefix(p.s[p.i:], "(anonymous namespace)") {
				p.i += len("(anonymous namespace)")
				t.Path = append(t.Path, Seg{Name: "(anonymous namespace)"})
				if !p.nameContinues() {
					break
				}
				continue
			}
			return nil, p.errf("expected a name")
		}
		w := p.peekWord()
		if strings.HasPrefix(w, "type") && strings.HasPrefix(p.s[p.i+skipLen(p.s, p.i):], "type-parameter-") {
			p.skip()
			j := p.i + len("type-parameter-")
			for j < len(p.s) && (p.s[j] >= '0' && p.s[j] <= '9' || p.s[j] == '-') {
				j++
			}
			t.Path = append(t.Path, Seg{Name: p.s[p.i:j]})
			p.i = j
			if !p.nameContinues() {
				break
			}
			continue
		}
		if w == "template" && len(t.Path) > 0 {
			p.i += len(w)
			continue
		}
		if w == "" {
			return nil, p.errf("expected a name")
		}
		p.i += len(w)
		seg := Seg{Name: w}
		if p.i < len(p.s) && p.s[p.i] == '<' {
			args, err := p.templateArgs()
			if err != nil {
				return nil, err
			}
			seg.HasArgs, seg.Args = true, args
		}
		t.Path = append(t.Path, seg)
		if !p.nameContinues() {
			break
		}
	}
	return t, nil
}

// nameContinues consumes the "::" that continues a qualified name, unless it
// is the start of a pointer to member ("S::*").
func (p *parser) nameContinues() bool {
	if !strings.HasPrefix(p.s[p.i:], "::") {
		return false
	}
	j := p.i + 2
	for j < len(p.s) && isSpace(p.s[j]) {
		j++
	}
	if j < len(p.s) && p.s[j] == '*' {
		return false
	}
	p.i += 2
	return true
}

// templateArgs reads <...>, at the '<'.
func (p *parser) templateArgs() ([]Arg, error) {
	p.skip()
	p.i++ // <
	var args []Arg
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxDepth {
		return nil, p.tooDeep()
	}
	for {
		p.skip()
		if p.i >= len(p.s) {
			return nil, p.errf("unbalanced <")
		}
		if p.s[p.i] == '>' {
			p.i++
			return args, nil
		}
		if len(args) > 0 {
			if p.s[p.i] != ',' {
				return nil, p.errf("expected , or > in the template arguments")
			}
			p.i++
			p.skip()
		}
		a, err := p.templateArg()
		if err != nil {
			return nil, err
		}
		args = append(args, a)
	}
}

func (p *parser) templateArg() (Arg, error) {
	p.skip()
	start := p.i
	// A type first, if what follows it ends the argument.
	w := p.peekWord()
	startsExpr := false
	switch {
	case w == "true" || w == "false" || w == "nullptr" || w == "sizeof" || w == "alignof" || w == "noexcept":
		startsExpr = true
	case p.i < len(p.s) && (p.s[p.i] >= '0' && p.s[p.i] <= '9' || p.s[p.i] == '-' || p.s[p.i] == '\'' || p.s[p.i] == '"'):
		startsExpr = true
	}
	if !startsExpr {
		save := p.i
		t, err := p.typeName()
		if err != nil && (p.fatal != nil || save < len(p.s) && isIdentStart(p.s[save])) {
			// A name or type that does not parse is an error, not an expression.
			return Arg{}, err
		}
		if err == nil {
			p.skip()
			if p.i < len(p.s) && (p.s[p.i] == ',' || p.s[p.i] == '>') {
				return Arg{Type: t}, nil
			}
		}
		p.i = save
	}
	// An expression: up to the ',' or '>' at depth 0.
	depth := 0
	j := p.i
	for ; j < len(p.s); j++ {
		c := p.s[j]
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case '<':
			if depth >= 0 && j > start && isIdentChar(p.s[j-1]) {
				depth++
			}
		case '>':
			if j > 0 && p.s[j-1] == '-' {
				continue
			}
			if depth == 0 {
				goto done
			}
			depth--
		case ',':
			if depth == 0 {
				goto done
			}
		case '\'', '"':
			q := c
			j++
			for j < len(p.s) && p.s[j] != q {
				if p.s[j] == '\\' {
					j++
				}
				j++
			}
		}
	}
done:
	if j >= len(p.s) || j == start {
		p.i = j
		return Arg{}, p.errf("bad template argument")
	}
	text := collapse(p.s[start:j])
	if strings.HasSuffix(text, "-") {
		return Arg{}, p.errf("bad template argument")
	}
	p.i = j
	pack := false
	if strings.HasSuffix(text, "...") {
		pack = true
		text = strings.TrimSpace(strings.TrimSuffix(text, "..."))
	}
	return Arg{Expr: text, Pack: pack}, nil
}

// A wrapper applies a declarator part to a type.
type wrapper func(*Type) *Type

// declarator reads the abstract declarator after the specifiers.
func (p *parser) declarator() (wrapper, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxDepth {
		return nil, p.tooDeep()
	}
	var ops []wrapper
loop:
	for {
		p.skip()
		if p.i >= len(p.s) {
			break
		}
		switch c := p.s[p.i]; {
		case c == '*':
			p.i++
			c, v, r := p.cvAfter()
			ops = append(ops, func(t *Type) *Type { return &Type{Kind: Pointer, Elem: t, Const: c, Volatile: v, Restrict: r} })
		case c == '&':
			p.i++
			k := LRef
			if p.i < len(p.s) && p.s[p.i] == '&' {
				p.i++
				k = RRef
			}
			p.cvAfter()
			ops = append(ops, func(t *Type) *Type { return &Type{Kind: k, Elem: t} })
		case isIdentStart(c) || c == ':':
			save := p.i
			cls, ok := p.memberPtrClass()
			if !ok {
				p.i = save
				break loop
			}
			c, v, r := p.cvAfter()
			ops = append(ops, func(t *Type) *Type {
				return &Type{Kind: MemberPtr, Elem: t, Class: cls, Const: c, Volatile: v, Restrict: r}
			})
		default:
			break loop
		}
	}
	var inner wrapper
	if p.peek() == '(' && p.isGroup() {
		p.i++
		var err error
		inner, err = p.declarator()
		if err != nil {
			return nil, err
		}
		if !p.eat(")") {
			return nil, p.errf("expected )")
		}
	}
	var sufs []wrapper
	for {
		p.skip()
		if p.i >= len(p.s) {
			break
		}
		switch p.s[p.i] {
		case '[':
			end := p.matchBracket()
			if end < 0 {
				return nil, p.errf("unbalanced [")
			}
			text := strings.TrimSpace(p.s[p.i+1 : end])
			p.i = end + 1
			n, expr := int64(-1), ""
			if text != "" {
				if v, err := strconv.ParseInt(text, 10, 64); err == nil && v >= 0 {
					n = v
				} else {
					expr = text
				}
			}
			sufs = append(sufs, func(t *Type) *Type { return &Type{Kind: Array, Elem: t, Len: n, LenExpr: expr} })
			continue
		case '(':
			f, err := p.funcSuffix()
			if err != nil {
				return nil, err
			}
			sufs = append(sufs, f)
			continue
		}
		break
	}
	return func(t *Type) *Type {
		for _, op := range ops {
			t = op(t)
		}
		for i := len(sufs) - 1; i >= 0; i-- {
			t = sufs[i](t)
		}
		if inner != nil {
			t = inner(t)
		}
		return t
	}, nil
}

// matchBracket returns the index of the ']' that closes the '[' at p.i.
func (p *parser) matchBracket() int {
	depth := 0
	for j := p.i; j < len(p.s); j++ {
		switch p.s[j] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// cvAfter reads the cv-qualifiers that follow a '*'.
func (p *parser) cvAfter() (c, v, r bool) {
	for {
		w := p.peekWord()
		switch cvWords[w] {
		case "const":
			c = true
		case "volatile":
			v = true
		case "restrict":
			r = true
		default:
			return
		}
		p.i += len(w)
	}
}

// memberPtrClass reads "S::*" (the class, then the "::" and the "*").
func (p *parser) memberPtrClass() (*Type, bool) {
	cls, err := p.name("")
	if err != nil {
		return nil, false
	}
	p.skip()
	if !strings.HasPrefix(p.s[p.i:], "::") {
		return nil, false
	}
	j := p.i + 2
	for j < len(p.s) && isSpace(p.s[j]) {
		j++
	}
	if j >= len(p.s) || p.s[j] != '*' {
		return nil, false
	}
	p.i = j + 1
	return cls, true
}

// isGroup reports whether the '(' at p.i groups a declarator ("(*)", "(&)",
// "(S::*)") instead of starting a parameter list.
func (p *parser) isGroup() bool {
	j := p.i + 1
	for j < len(p.s) && isSpace(p.s[j]) {
		j++
	}
	if j >= len(p.s) {
		return false
	}
	switch p.s[j] {
	case '*', '&', '^':
		return true
	}
	if isIdentStart(p.s[j]) || p.s[j] == ':' {
		q := &parser{s: p.s, i: j, memo: p.memo, depth: p.depth}
		defer func() {
			p.memo = q.memo
			if q.fatal != nil && p.fatal == nil {
				p.fatal = q.fatal // the speculative read hit the limit: so did this one
			}
		}()
		if _, ok := q.memberPtrClass(); ok {
			return true
		}
	}
	return false
}

// funcSuffix reads "(params) quals" at the '('.
func (p *parser) funcSuffix() (wrapper, error) {
	p.i++ // (
	var params []*Type
	variadic := false
	for {
		p.skip()
		if p.i >= len(p.s) {
			return nil, p.errf("unbalanced (")
		}
		if p.s[p.i] == ')' {
			p.i++
			break
		}
		if len(params) > 0 || variadic {
			if p.s[p.i] != ',' {
				return nil, p.errf("expected , or ) in the parameters")
			}
			p.i++
			p.skip()
		}
		if strings.HasPrefix(p.s[p.i:], "...") {
			p.i += 3
			variadic = true
			continue
		}
		t, err := p.typeName()
		if err != nil {
			return nil, err
		}
		params = append(params, t)
	}
	// (void) is no parameters in C.
	if len(params) == 1 && params[0].Kind == Builtin && params[0].Name == "void" && !params[0].Const && !params[0].Volatile {
		params = nil
	}
	f := &Type{Kind: Func, Params: params, Variadic: variadic}
	var trailing *Type
	for {
		p.skip()
		w := p.peekWord()
		switch {
		case w == "const":
			p.i += len(w)
			f.ConstQ = true
			continue
		case w == "volatile":
			p.i += len(w)
			f.VolatileQ = true
			continue
		case w == "noexcept":
			p.i += len(w)
			f.Noexcept = "noexcept"
			if p.peek() == '(' {
				end := p.matchParen()
				if end < 0 {
					return nil, p.errf("unbalanced (")
				}
				f.Noexcept = "noexcept" + p.s[p.i:end+1]
				p.i = end + 1
			}
			continue
		case w == "throw":
			p.i += len(w)
			if p.peek() != '(' {
				return nil, p.errf("expected ( after throw")
			}
			end := p.matchParen()
			if end < 0 {
				return nil, p.errf("unbalanced (")
			}
			f.Noexcept = "throw" + p.s[p.i:end+1]
			p.i = end + 1
			continue
		case w == "__attribute__":
			p.i += len(w)
			if p.peek() != '(' {
				return nil, p.errf("expected ( after __attribute__")
			}
			end := p.matchParen()
			if end < 0 {
				return nil, p.errf("unbalanced (")
			}
			if f.Attr != "" {
				f.Attr += " "
			}
			f.Attr += "__attribute__" + p.s[p.i:end+1]
			p.i = end + 1
			continue
		case strings.HasPrefix(p.s[p.i:], "&&"):
			p.i += 2
			f.RefQual = "&&"
			continue
		case strings.HasPrefix(p.s[p.i:], "&"):
			p.i++
			f.RefQual = "&"
			continue
		case strings.HasPrefix(p.s[p.i:], "->"):
			p.i += 2
			t, err := p.typeName()
			if err != nil {
				return nil, err
			}
			trailing = t
			continue
		}
		break
	}
	return func(t *Type) *Type {
		c := *f
		c.Elem = t
		if trailing != nil {
			c.Elem, c.Trailing = trailing, true
		}
		return &c
	}, nil
}

// matchParen returns the index of the ')' that closes the '(' at p.i.
func (p *parser) matchParen() int {
	depth := 0
	for j := p.i; j < len(p.s); j++ {
		switch p.s[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// collapse trims s and replaces every run of ASCII white space by one space.
func collapse(s string) string {
	var b strings.Builder
	space := false
	for i := 0; i < len(s); i++ {
		if isSpace(s[i]) {
			space = b.Len() > 0
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// skipLen is the number of white space bytes at s[i:].
func skipLen(s string, i int) int {
	n := 0
	for i+n < len(s) && isSpace(s[i+n]) {
		n++
	}
	return n
}

// isUnnamedText reports whether the text inside parentheses names a type with no name:
// "unnamed struct", "anonymous union", "lambda", "unnamed struct at /f.h:1:2", and the bare
// words that Parse(String(t)) of such a text gives.
func isUnnamedText(inner string) bool {
	for _, w := range []string{"unnamed", "anonymous", "lambda"} {
		if inner == w || strings.HasPrefix(inner, w+" ") {
			return true
		}
	}
	return false
}
