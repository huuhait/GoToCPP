package icheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The messages. clang's diagnostic for a failed probe is mapped to Go's words
// (the wording of go/types where Go has the same error: "cannot use x (variable
// of type T) as U value in argument to f", "too many arguments in call to f",
// "x.y undefined (type T has no field or method y)"), with the C++ types written
// as the foreign view writes them (C.int, *C.char, std.vector[C.long]). A
// diagnostic that has no rule is the fallback, which keeps clang's message:
//
//	<expression> is not valid C++: <clang's message>
//
// Every pattern of these messages has an entry in `gocpp explain`
// (internal/explain/messages.go, TestInteropCheckerMessages).

// A failCtx is what a rule knows about one failed use.
type failCtx struct {
	ref  *collect.Ref
	res  *probe.Result
	st   collect.Site
	info *astInfo
	pos  token.Position

	msg    string // the compiler's message, without a suggestion
	hint   string // the compiler's suggestion ("did you mean 'size'?"), if any
	text   string // the use as the user wrote it
	callee string // the function of a call, as the user wrote it
	args   []string
	call   *collect.Call // the root of the reference, when it is a call
	argIdx int           // the argument the error is about, -1 when unknown

	related []diag.Related

	// skip, when set by a rule, says that the error is not reported: it repeats
	// another one (a void argument is reported at the argument).
	skip string
}

// flagTag is the name of the warning option that clang appends to a warning that is an
// error by default ("[-Waddress-of-temporary]").
var flagTag = regexp.MustCompile(` \[-W[^\]]+\]$`)

var didYouMean = regexp.MustCompile(`;\s*did you mean '([^']*)'\?\s*$`)

func (c *checker) newFailCtx(r *collect.Ref, res *probe.Result, st collect.Site, info *astInfo) *failCtx {
	fc := &failCtx{ref: r, res: res, st: st, info: info, pos: c.pos(st), argIdx: -1}
	fc.msg = flagTag.ReplaceAllString(akaPair(res.Error), "")
	if m := didYouMean.FindStringSubmatch(fc.msg); m != nil {
		fc.hint = m[1]
		fc.msg = didYouMean.ReplaceAllString(fc.msg, "")
	}
	fc.text = userText(st.Text, c.hdrs)
	fc.call, _ = r.Expr.(*collect.Call)
	if fc.call != nil {
		if callee, args, ok := splitCall(fc.text); ok {
			fc.callee, fc.args = callee, args
			if len(args) != len(fc.call.Args) {
				fc.args = nil
			}
		}
		fc.argIdx = c.failingArg(fc)
	}
	return fc
}

// argPos returns the position of the argument idx of the call that the use is,
// else def: go/types points at the argument, and so does the checker.
func (c *checker) argPos(fc *failCtx, def token.Position) token.Position {
	if fc.info == nil || fc.argIdx < 0 {
		return def
	}
	if call, ok := fc.info.node.(*ast.CallExpr); ok && fc.argIdx < len(call.Args) {
		if p := c.fset.Position(call.Args[fc.argIdx].Pos()); p.IsValid() {
			return p
		}
	}
	return def
}

// selPos returns the position of the selector name of the member that the use
// takes (p.nosuch is at nosuch), else def.
func (c *checker) selPos(fc *failCtx, def token.Position) token.Position {
	if fc.info == nil {
		return def
	}
	n := fc.info.node
	if call, ok := n.(*ast.CallExpr); ok {
		n = call.Fun
	}
	if s, ok := n.(*ast.SelectorExpr); ok {
		if p := c.fset.Position(s.Sel.Pos()); p.IsValid() {
			return p
		}
	}
	return def
}

// failingArg finds which argument of the call the compiler's error is about,
// from the column of the error in the probe's line: the argument that starts
// there. -1 when it cannot tell.
func (c *checker) failingArg(fc *failCtx) int {
	at := fc.res.ErrorAt
	src := c.rep.Table.Source
	if at == nil || src == nil || at.Line < 1 {
		return -1
	}
	lines := c.probeLines()
	if at.Line > len(lines) {
		return -1
	}
	line := lines[at.Line-1]
	i := strings.Index(line, "decltype((")
	j := strings.LastIndex(line, ">;") - 2 // the expression ends before "))" of the decltype
	if i < 0 || j < i+len("decltype((") {
		return -1
	}
	base := i + len("decltype((")
	inner := line[base:j]
	// The expression may be wrapped in parentheses of its own.
	for strings.HasPrefix(inner, "(") {
		open, _, ok := callSpans(inner)
		if ok && open == 0 {
			break
		}
		if !strings.HasSuffix(inner, ")") || !balanced(inner[1:len(inner)-1]) {
			break
		}
		inner = inner[1 : len(inner)-1]
		base++
	}
	_, spans, ok := callSpans(inner)
	if !ok {
		return -1
	}
	col := at.Col - 1 - base
	for k, sp := range spans {
		if col >= sp[0] && col < sp[1] {
			return k
		}
	}
	return -1
}

// balanced reports whether the parentheses of s are balanced (quotes aside).
func balanced(s string) bool {
	d := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\'':
			if j := skipQuoted(s, i); j >= 0 {
				i = j
			}
		case '(':
			d++
		case ')':
			d--
			if d < 0 {
				return false
			}
		}
	}
	return d == 0
}

func (c *checker) probeLines() []string {
	if c.pl == nil {
		c.pl = strings.Split(c.rep.Table.Source.Text, "\n")
	}
	return c.pl
}

// ---------------------------------------------------------------- types

// mapType maps a type of the compiler's message.
func (c *checker) mapType(s string) (ftype.Mapped, bool) {
	s = strings.TrimSpace(identityRe.ReplaceAllString(akaRe.ReplaceAllString(s, ""), "$1"))
	t, err := ctype.Parse(s)
	if err != nil {
		return ftype.Mapped{}, false
	}
	m := c.rep.Mapper.Map(t, nil)
	if m.Kind == ftype.Opaque {
		return m, false
	}
	return m, true
}

var akaTail = regexp.MustCompile(akaSep + `[^']*`)

var akaPairRe = regexp.MustCompile(`'([^']*)' \(aka '([^']*)'\)`)

// akaSep joins the two spellings of a type that the compiler gives as 'X' (aka
// 'Y'): X as the declaration wrote it (FILE *, value_type) and Y the canonical
// one (_IO_FILE *, long). ty chooses the one that reads better.
const akaSep = "\x00"

// akaPair puts the pair of spellings of a type of the message in one quoted word.
func akaPair(s string) string { return akaPairRe.ReplaceAllString(s, "'$1"+akaSep+"$2'") }

// disp writes a mapped type the way the program names it, with the header
// package in front of the names it declares (shapes.Point).
func (c *checker) disp(m ftype.Mapped) string { return c.qualify(m.Display()) }

// qualify puts the header package in front of the bare names of types that the
// program names through one (Point is shapes.Point). The package of a name is known
// when the program writes the name (shapes.Point), or one of the enumerators of an
// enum (shapes.Blue makes Color shapes.Color). Other names stay as the foreign
// view has them: nothing says which header declares them (step 12.13).
func (c *checker) qualify(s string) string {
	if len(c.hdrs) == 0 {
		return s
	}
	if c.typeHdr == nil {
		c.typeHdr = map[string]string{}
		for n, h := range c.hdrs {
			c.typeHdr[n] = h
		}
		if d := c.rep.Decls; d != nil {
			for name, e := range d.Enums {
				if strings.Contains(name, "::") {
					continue
				}
				for _, v := range e.Values {
					if h := c.hdrs[v.Name]; h != "" {
						c.typeHdr[name] = h
						break
					}
				}
			}
		}
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		ch := s[i]
		if !(ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z') {
			b.WriteByte(ch)
			i++
			continue
		}
		j := i
		for j < len(s) && (s[j] == '_' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= '0' && s[j] <= '9') {
			j++
		}
		word := s[i:j]
		after := byte(0)
		if j < len(s) {
			after = s[j]
		}
		before := byte(0)
		if i > 0 {
			before = s[i-1]
		}
		switch {
		case before == '.' || after == '.' || word == "const" || word == "void" || word == "std" || word == "C" || goWords[word]:
			b.WriteString(word)
		case c.typeHdr[word] != "":
			b.WriteString(c.typeHdr[word] + "." + word)
		default:
			b.WriteString(word)
		}
		i = j
	}
	return b.String()
}

// goWords are the words of a type in Go's spelling.
var goWords = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true, "uint": true, "uint8": true, "uint16": true,
	"uint32": true, "uint64": true, "uintptr": true, "float32": true, "float64": true, "string": true, "bool": true,
	"byte": true, "rune": true, "error": true, "any": true, "unsafe": true, "func": true, "map": true, "opaque": true,
	"variable": true, "value": true, "type": true,
}

// ty writes a type of the compiler's message in the foreign view's words:
// C.int, *C.char, Point, std.vector[C.long], string for the probe's string. A
// type that the mapper cannot read is kept as the compiler wrote it, with "."
// for "::".
func (c *checker) ty(s string) string {
	s = strings.TrimSpace(s)
	if x, y, ok := strings.Cut(s, akaSep); ok {
		// The written spelling when the foreign view knows it, else the canonical one.
		if _, ok := c.mapType(x); ok && !memberTypedef(x) {
			return c.ty(x)
		}
		return c.ty(y)
	}
	m, ok := c.mapType(s)
	if !ok {
		return plainType(s)
	}
	d := c.disp(m)
	if m.Const && m.Kind != ftype.Pointer && m.Kind != ftype.FuncPtr {
		d = "const " + d
	}
	return d
}

var memberTypedefRe = regexp.MustCompile(`^(?:const )?(?:\w+_type|type|iterator|const_iterator|reference|const_reference|pointer|const_pointer)\b`)

// memberTypedef reports whether the written spelling of a type is a member
// typedef of a library class (value_type, iterator): the canonical spelling
// says more.
func memberTypedef(x string) bool { return memberTypedefRe.MatchString(strings.TrimSpace(x)) }

// goTypes name the Go types that C++ gets as the fixed-size types of <cstdint>.
var goTypes = map[string]string{
	"int64_t": "int", "int32_t": "int32", "int16_t": "int16", "int8_t": "int8",
	"uint64_t": "uint64", "uint32_t": "uint32", "uint16_t": "uint16", "uint8_t": "uint8",
	"uintptr_t": "uintptr", "double": "float64", "float": "float32", "bool": "bool",
	"void*": "unsafe.Pointer",
}

// typeOfExpr writes the type that the collector's expression t names, in Go's
// words, or "".
func (c *checker) typeOfExpr(t collect.Expr) string {
	switch t := t.(type) {
	case *collect.GoType:
		if g, ok := goTypes[t.Cxx]; ok {
			return g
		}
		return plainType(t.Cxx)
	case *collect.Stand:
		switch t.Name {
		case "string":
			return "string"
		case "slice":
			if len(t.Args) == 1 {
				return "[]" + c.typeOfExpr(t.Args[0])
			}
		case "array":
			if len(t.Args) == 2 {
				return "[" + strings.TrimPrefix(t.Args[1].String(), "lit(") + "]" + c.typeOfExpr(t.Args[0])
			}
		case "map":
			if len(t.Args) == 2 {
				return "map[" + c.typeOfExpr(t.Args[0]) + "]" + c.typeOfExpr(t.Args[1])
			}
		}
	case *collect.Decay:
		if v := c.valueOf(t.String()); v != nil {
			return c.disp(v.Mapped)
		}
	case *collect.Path:
		if t.Ns != "" && t.Ns != "C" {
			return strings.ReplaceAll(strings.TrimPrefix(t.Ns, "::"), "::", ".") + "." + strings.Join(t.Parts, ".")
		}
		if t.Hdr != "" && t.Ns == "" {
			return t.Hdr + "." + strings.Join(t.Parts, ".")
		}
		return c.qualify(strings.Join(t.Parts, "."))
	case *collect.Star:
		if in := c.typeOfExpr(t.X); in != "" {
			return "*" + in
		}
	case *collect.Inst:
		var as []string
		for _, a := range t.Args {
			as = append(as, c.typeOfExpr(a))
		}
		if x := c.typeOfExpr(t.X); x != "" && !slicesHas(as, "") {
			return x + "[" + strings.Join(as, ", ") + "]"
		}
	case *collect.IndexOrInst:
		var as []string
		for _, a := range t.Args {
			as = append(as, c.typeOfExpr(a))
		}
		if x := c.typeOfExpr(t.X); x != "" && !slicesHas(as, "") {
			return x + "[" + strings.Join(as, ", ") + "]"
		}
	}
	return ""
}

func slicesHas(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// describe says what an argument is, in the words of go/types: "untyped int
// constant", "variable of type int", "value of type C.ulong".
func (c *checker) describe(e collect.Expr) string {
	switch e := e.(type) {
	case *collect.Lit:
		return describeLit(e.Text)
	case *collect.Arg:
		if t := c.typeOfExpr(e.T); t != "" {
			if e.LValue {
				return "variable of type " + t
			}
			return "value of type " + t
		}
		return "value"
	}
	if v := c.valueOf(e.String()); v != nil {
		if v.Lvalue {
			return "variable of type " + c.disp(v.Mapped)
		}
		return "value of type " + c.disp(v.Mapped)
	}
	return "value"
}

// ---------------------------------------------------------------- rules

// A rule maps one family of the compiler's messages.
type rule struct {
	re *regexp.Regexp
	f  func(c *checker, fc *failCtx, m []string) string
}

var rules []rule

func init() {
	add := func(re string, f func(c *checker, fc *failCtx, m []string) string) {
		rules = append(rules, rule{regexp.MustCompile(re), f})
	}
	add(`^no member named '([^']+)' in (the global namespace|namespace '([^']+)')$`, (*checker).ruleUndefinedMember)
	add(`^(?:use of undeclared identifier|unknown type name|no template named) '([^']+)'$`, (*checker).ruleUndeclared)
	add(`^(?:no type named|no template named) '([^']+)' in namespace '([^']+)'`, (*checker).ruleUndefinedType)
	add(`^no member named '([^']+)' in '(.+)'$`, (*checker).ruleNoMember)
	add(`^member reference type '(.+)' is a pointer`, (*checker).rulePointerMember)
	add(`^'([^']+)' is a (private|protected) member of '(.+)'$`, (*checker).rulePrivate)
	add(`^no matching (?:member )?(?:function|constructor) for (?:call to|initialization of) '(.+)'$`, (*checker).ruleNoMatch)
	add(`^call to '(.+)' is ambiguous$`, (*checker).ruleAmbiguous)
	add(`^cannot initialize a parameter of type '(.+)' with an? (?:lvalue|rvalue) of type '(.+)'$`, (*checker).ruleArgType)
	add(`^no viable conversion from '(.+)' to '(.+)'$`, (*checker).ruleConversionFrom)
	add(`^(?:non-const lvalue reference to type '(.+)' cannot bind to (?:a temporary of type|a value of unrelated type|an rvalue of type|a bit-field of type|a vector element of type|a value of type) '(.+)'|binding reference of type '(.+)' to value of type '(.+)' drops '(?:const|volatile|const volatile)' qualifier|rvalue reference to type '(.+)' cannot bind to lvalue of type '(.+)'|reference to type '(.+)' could not bind to an? (?:lvalue|rvalue) of type '(.+)')`, (*checker).ruleReferenceBind)
	add(`^too (many|few) arguments to (?:function call|call to .*), expected (.+), have (\d+)`, (*checker).ruleArity)
	add(`^too few arguments to function call, single argument '[^']*' was not specified$`, (*checker).ruleArityOne)
	add(`^called object type '(.+)' is not a function or function pointer$`, (*checker).ruleNotFunction)
	add(`^invalid operands to binary expression \('(.+)' and '(.+)'\)$`, (*checker).ruleBinary)
	add(`^invalid argument type '(.+)' to unary expression$`, (*checker).ruleUnary)
	add(`^'this' argument to member function '([^']+)' has type '(.+)', but function is not marked (.+)$`, (*checker).ruleConstMethod)
	add(`^reference to non-static member function must be called`, (*checker).ruleMethodValue)
	add(`^call to non-static member function without an object argument$`, (*checker).ruleNonStatic)
	add(`^(?:cannot take the address of an rvalue of type|taking the address of a temporary object of type) '(.+)'$`, (*checker).ruleAddressOf)
	add(`^expression is not assignable$|^cannot assign to .*$`, (*checker).ruleNotAssignable)
	add(`^(?:incomplete type '(.+)' named in nested name specifier|member access into incomplete type '(.+)'|invalid use of incomplete type '(.+)'|implicit instantiation of undefined template '(.+)'|allocation of incomplete type '(.+)'|variable has incomplete type '(.+)')`, (*checker).ruleIncomplete)
	add(`^(?:call to (?:implicitly-deleted )?[a-z ]+ of '(.+)' is deleted|call to deleted .*|call to implicitly-deleted .*|attempt to use a deleted function)`, (*checker).ruleDeleted)
	add(`^(?:cannot convert '(.+)' to '(.+)' without a conversion operator|no matching conversion for (?:functional-style cast|static_cast|C-style cast) from '(.+)' to '(.+)'|static_cast from '(.+)' to '(.+)' is not allowed)`, (*checker).ruleCast)
}

// explainFailure maps the failure of a probe to the message of the diagnostic,
// its related lines, and its position.
func (c *checker) explainFailure(fc *failCtx) (msg string, related []diag.Related, pos token.Position) {
	fc.pos = c.pos(fc.st)
	for _, r := range rules {
		if m := r.re.FindStringSubmatch(fc.msg); m != nil {
			if out := r.f(c, fc, m); out != "" {
				return out, fc.related, fc.pos
			}
		}
	}
	return fmt.Sprintf(fmtNotValidCXX, fc.text, plainMsg(fc.msg)), fc.related, fc.pos
}

// plainMsg writes the types in a compiler message the way plainType does.
func plainMsg(s string) string {
	s = akaRe.ReplaceAllString(s, "")
	s = akaTail.ReplaceAllString(s, "")
	s = standRe.Replace(s)
	return s
}

// selText returns the text of the selector that ends with .name in the use.
func selText(text, name string) string {
	if i := strings.Index(text, "."+name); i >= 0 {
		return text[:i+1+len(name)]
	}
	return text
}

func (c *checker) ruleUndefinedMember(fc *failCtx, m []string) string {
	name := m[1]
	cxx := "::" + name
	if m[3] != "" {
		cxx = "::" + m[3] + "::" + name
	}
	text := fc.callee
	if text == "" {
		text = fc.text
	}
	written := selText(text, name)
	if !strings.HasSuffix(written, name) {
		written = text
	}
	// The error is at the name, as the lowering reports an unknown name.
	if i := strings.LastIndex(written, name); i >= 0 && strings.HasSuffix(written, name) {
		fc.pos.Column = c.colOfName(fc.pos, written, i)
	}
	return fmt.Sprintf(fmtUndefined, written, cxx)
}

// colOfName returns the column of the name that starts at offset i of the
// user's text written, which starts at pos, when the source has it there.
func (c *checker) colOfName(pos token.Position, written string, i int) int {
	line := c.srcLine(pos.Filename, pos.Line)
	if pos.Column >= 1 && pos.Column-1+len(written) <= len(line) && line[pos.Column-1:pos.Column-1+len(written)] == written {
		return pos.Column + i
	}
	return pos.Column
}

func (c *checker) ruleUndeclared(fc *failCtx, m []string) string {
	return fmt.Sprintf(fmtUndefined, m[1], "::"+m[1])
}

func (c *checker) ruleUndefinedType(fc *failCtx, m []string) string {
	text := fc.text
	if fc.callee != "" {
		text = fc.callee
	}
	written := selText(text, m[1])
	return fmt.Sprintf(fmtUndefined, written, "::"+m[2]+"::"+m[1])
}

func (c *checker) ruleNoMember(fc *failCtx, m []string) string {
	name, recv := m[1], m[2]
	sel := selText(fc.calleeOrText(), name)
	fc.hintMembers(c, recv, name)
	fc.pos = c.selPos(fc, fc.pos)
	return fmt.Sprintf(fmtNoMember, sel, c.receiverType(fc, recv), name)
}

// receiverType writes the type of the value whose member is wanted as the
// program names it (z_stream, not the struct z_stream_s that the compiler
// prints), else as the compiler's message has it.
func (c *checker) receiverType(fc *failCtx, clangType string) string {
	var x collect.Expr
	switch e := fc.ref.Expr.(type) {
	case *collect.Select:
		x = e.X
	case *collect.Call:
		if s, ok := e.Fun.(*collect.Select); ok {
			x = s.X
		}
	}
	if a, ok := x.(*collect.Arg); ok {
		if t := c.typeOfExpr(a.T); t != "" {
			return t
		}
	}
	return c.ty(clangType)
}

func (fc *failCtx) calleeOrText() string {
	if fc.callee != "" {
		return fc.callee
	}
	return fc.text
}

// hintMembers adds "did you mean" from the members of the record, or from the
// compiler's own suggestion.
func (fc *failCtx) hintMembers(c *checker, recv, name string) {
	hint := ""
	if mp, ok := c.mapType(recv); ok {
		if ms := c.rep.Mapper.MembersOf(mp.Type); ms != nil {
			hint = closest(ms.Names(), name)
		}
	}
	if hint == "" {
		hint = fc.hint
	}
	if hint != "" {
		fc.related = append(fc.related, diag.Related{Msg: "did you mean " + hint + "?"})
	}
}

// closest returns the name that is at most two edits away from name, the nearest
// one first, or "" (a case difference counts as one edit).
func closest(names []string, name string) string {
	if len(name) < 3 {
		return "" // one letter is close to every one-letter name
	}
	limit := 1
	if len(name) >= 6 {
		limit = 2
	}
	best, bestD := "", limit+1
	for _, n := range names {
		if n == name || strings.HasPrefix(n, "~") || strings.HasPrefix(n, "operator") {
			continue
		}
		d := editDistance(n, name)
		if strings.EqualFold(n, name) {
			d = 1
		}
		if d < bestD || d == bestD && n < best {
			best, bestD = n, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

func (c *checker) rulePointerMember(fc *failCtx, m []string) string {
	name := ""
	if fc.call != nil {
		if s, ok := fc.call.Fun.(*collect.Select); ok {
			name = s.Name
		}
	} else if s, ok := fc.ref.Expr.(*collect.Select); ok {
		name = s.Name
	}
	if name == "" {
		return ""
	}
	// The compiler names the type after the automatic dereference of one level; the
	// value is a pointer to that.
	fc.pos = c.selPos(fc, fc.pos)
	return fmt.Sprintf(fmtNoMember, selText(fc.calleeOrText(), name), "*"+c.ty(m[1]), name)
}

func (c *checker) rulePrivate(fc *failCtx, m []string) string {
	fc.pos = c.selPos(fc, fc.pos)
	return fmt.Sprintf(fmtPrivate, selText(fc.calleeOrText(), m[1]), m[1], m[2], c.ty(m[3]))
}

// argTypes writes the types of the arguments of the call for a message.
func (c *checker) argTypes(fc *failCtx) (string, bool) {
	if fc.call == nil {
		return "", false
	}
	var ts []string
	for _, a := range fc.call.Args {
		t := c.argType(a)
		if t == "" {
			return "", false
		}
		ts = append(ts, t)
	}
	return strings.Join(ts, ", "), true
}

// argType is the type of an argument in Go's words, "untyped int" for a
// constant ("" when unknown).
func (c *checker) argType(e collect.Expr) string {
	if l, ok := e.(*collect.Lit); ok {
		return strings.TrimSuffix(describeLit(l.Text), " constant")
	}
	if a, ok := e.(*collect.Arg); ok {
		return c.typeOfExpr(a.T)
	}
	if v := c.valueOf(e.String()); v != nil {
		return c.disp(v.Mapped)
	}
	return ""
}

var (
	noteArity   = regexp.MustCompile(`^candidate .*not viable: requires (?:(single argument '[^']*')|(\d+) arguments?|at least (\d+) arguments?|(?:from|between) (\d+) (?:and|to) (\d+) arguments?), but (\d+) (?:was|were|arguments? (?:was|were)) provided`)
	noteConv    = regexp.MustCompile(`^candidate .*not viable: no known conversion from '(.+)' to '(.+)' for (\d+)(?:st|nd|rd|th) argument`)
	noteIgnored = regexp.MustCompile(`^(candidate .* ignored): (.*)$`)
	noteThis    = regexp.MustCompile(`^candidate .*not viable: '(this)' argument has type '(.+)', but method is not marked (.+)$`)
)

// addCandidates turns the notes of an overload failure into related lines.
func (c *checker) addCandidates(fc *failCtx) {
	const limit = 6
	n := 0
	for _, note := range fc.res.Notes {
		text := ""
		note.Msg = akaPair(note.Msg)
		switch {
		case noteArity.MatchString(note.Msg):
			m := noteArity.FindStringSubmatch(note.Msg)
			want := ""
			switch {
			case m[1] != "":
				want = "1 argument"
			case m[2] != "":
				want = m[2] + " arguments"
			case m[3] != "":
				want = "at least " + m[3] + " arguments"
			default:
				want = m[4] + " to " + m[5] + " arguments"
			}
			text = fmt.Sprintf("candidate needs %s, got %s", want, m[6])
		case noteConv.MatchString(note.Msg):
			m := noteConv.FindStringSubmatch(note.Msg)
			text = fmt.Sprintf("candidate: argument %s: cannot use %s as %s", m[3], c.ty(m[1]), c.ty(m[2]))
		case noteThis.MatchString(note.Msg):
			m := noteThis.FindStringSubmatch(note.Msg)
			text = fmt.Sprintf("candidate: the value (type %s) is not %s", c.ty(m[2]), m[3])
		case noteIgnored.MatchString(note.Msg):
			m := noteIgnored.FindStringSubmatch(note.Msg)
			text = m[1] + ": " + c.noteText(m[2])
		case strings.HasPrefix(note.Msg, "candidate"):
			text = "candidate declared here"
		default:
			continue
		}
		if n == limit {
			fc.related = append(fc.related, diag.Related{Msg: "and more candidates"})
			break
		}
		n++
		fc.related = append(fc.related, diag.Related{Pos: c.diagPos(note), Msg: text})
	}
}

// noteText writes the types of a compiler's note.
func (c *checker) noteText(s string) string { return plainMsg(s) }

// diagPos returns the position of a compiler diagnostic when it is in a header
// (the probe file has no place of the user's), else the zero position.
func (c *checker) diagPos(d probe.Diag) token.Position {
	if d.File == "" || d.Line == 0 || strings.HasSuffix(d.File, "gp_probe.cpp") {
		return token.Position{}
	}
	return token.Position{Filename: d.File, Line: d.Line, Column: d.Col}
}

func (c *checker) ruleNoMatch(fc *failCtx, m []string) string {
	c.addCandidates(fc)
	callee := fc.callee
	if callee == "" {
		callee = fc.text
	}
	if ts, ok := c.argTypes(fc); ok {
		return fmt.Sprintf(fmtNoOverload, callee, ts)
	}
	return fmt.Sprintf(fmtNoOverloadAt, callee)
}

func (c *checker) ruleAmbiguous(fc *failCtx, m []string) string {
	c.addCandidates(fc)
	callee := fc.callee
	if callee == "" {
		callee = fc.text
	}
	return fmt.Sprintf(fmtAmbiguous, callee)
}

// argText returns the text of the failing argument and what it is, or
// "an argument" and "value" when they are not known.
func (c *checker) argText(fc *failCtx) (text, desc string) {
	if fc.call != nil && fc.argIdx >= 0 && fc.argIdx < len(fc.call.Args) {
		desc = c.describe(fc.call.Args[fc.argIdx])
		if fc.args != nil {
			return fc.args[fc.argIdx], desc
		}
		return "an argument", desc
	}
	return "an argument", "value"
}

func (c *checker) calleeName(fc *failCtx) string {
	if fc.callee != "" {
		return fc.callee
	}
	return fc.text
}

func (c *checker) ruleArgType(fc *failCtx, m []string) string {
	if strings.TrimSpace(m[2]) == "void" {
		fc.skip = "the argument is a void call: reported at the argument"
		return "-"
	}
	text, desc := c.argText(fc)
	if fc.argIdx < 0 || desc == "value" {
		desc = "value of type " + c.ty(m[2])
	}
	fc.cppReason(c, m[1], m[2])
	fc.pos = c.argPos(fc, fc.pos)
	return fmt.Sprintf(fmtCannotUse, text, desc, c.ty(m[1]), "argument to "+c.calleeName(fc))
}

func (c *checker) ruleConversionFrom(fc *failCtx, m []string) string {
	if strings.TrimSpace(m[1]) == "void" {
		fc.skip = "the argument is a void call: reported at the argument"
		return "-"
	}
	text, desc := c.argText(fc)
	if fc.argIdx < 0 || desc == "value" {
		desc = "value of type " + c.ty(m[1])
	}
	fc.cppReason(c, m[2], m[1])
	fc.pos = c.argPos(fc, fc.pos)
	return fmt.Sprintf(fmtCannotUse, text, desc, c.ty(m[2]), "argument to "+c.calleeName(fc))
}

// cppReason adds the line that says why C++ refuses the conversion from the
// type from to the type to, when it is one of the usual mistakes.
func (fc *failCtx) cppReason(c *checker, to, from string) {
	tm, tok := c.mapType(to)
	fm, fok := c.mapType(from)
	reason := ""
	switch {
	case strings.Contains(from, standName) && tok && tm.Kind == ftype.Pointer:
		reason = "a Go string has no conversion to a C pointer: write C.cstr(s) (or C.CString(s))"
	case strings.Contains(from, standName):
		reason = "a Go string converts only to std::string_view in C++ (not to std::string or const char*): write std.string(s) or C.cstr(s)"
	case tok && fok && tm.Kind == ftype.Enum && fm.Kind == ftype.Scalar:
		reason = "C++ has no implicit conversion from an integer to an enum: write the enumerator, or an explicit conversion"
	case tok && fok && tm.Kind == ftype.Enum && fm.Kind == ftype.Enum:
		reason = "the two are different enumeration types"
	case tok && fok && tm.Kind == ftype.Pointer && fm.Kind == ftype.Scalar:
		reason = "C++ has no implicit conversion from an integer to a pointer: only the constant 0 (NULL) is a null pointer"
	case tok && fok && tm.Kind == ftype.Pointer && fm.Kind == ftype.Record && tm.Elem != nil && tm.Elem.Type == fm.Type:
		reason = "the parameter is a pointer: pass the address of the value (&x)"
	}
	if reason != "" {
		fc.related = append(fc.related, diag.Related{Msg: reason})
	}
}

func (c *checker) ruleReferenceBind(fc *failCtx, m []string) string {
	to, from := m[1], m[2]
	if to == "" && m[3] != "" {
		to, from = m[3], m[4]
	}
	if to == "" {
		to, from = m[5], m[6]
	}
	if to == "" {
		to, from = m[7], m[8]
	}
	text, desc := c.argText(fc)
	if fc.argIdx < 0 || desc == "value" {
		desc = "value of type " + c.ty(from)
	}
	fc.related = append(fc.related, diag.Related{Msg: "the C++ parameter is a reference that needs a variable (an addressable value), not a temporary value or a const one"})
	fc.pos = c.argPos(fc, fc.pos)
	return fmt.Sprintf(fmtCannotUse, text, desc, c.ty(to), "argument to "+c.calleeName(fc))
}

func (c *checker) ruleArity(fc *failCtx, m []string) string {
	want := m[2]
	if strings.HasPrefix(want, "single argument") {
		want = "1"
	}
	want = strings.TrimPrefix(want, "at least ")
	if strings.HasPrefix(m[2], "at least ") {
		want = "at least " + want
	}
	if m[1] == "many" {
		return fmt.Sprintf(fmtTooMany, c.calleeName(fc), m[3], want)
	}
	return fmt.Sprintf(fmtNotEnough, c.calleeName(fc), m[3], want)
}

func (c *checker) ruleArityOne(fc *failCtx, m []string) string {
	return fmt.Sprintf(fmtNotEnough, c.calleeName(fc), "0", "1")
}

func (c *checker) ruleNotFunction(fc *failCtx, m []string) string {
	return fmt.Sprintf(fmtNotFunction, c.calleeName(fc), c.ty(m[1]))
}

func (c *checker) ruleBinary(fc *failCtx, m []string) string {
	op := "this operator"
	if b, ok := fc.ref.Expr.(*collect.Binary); ok {
		op = b.Op
	}
	return fmt.Sprintf(fmtBinary, fc.text, op, c.ty(m[1]), c.ty(m[2]))
}

func (c *checker) ruleUnary(fc *failCtx, m []string) string {
	op := "this operator"
	if u, ok := fc.ref.Expr.(*collect.Unary); ok {
		op = u.Op
	}
	return fmt.Sprintf(fmtUnary, op, fc.text, c.ty(m[1]))
}

func (c *checker) ruleConstMethod(fc *failCtx, m []string) string {
	recv := fc.callee
	if i := strings.LastIndex(recv, "."+m[1]); i >= 0 {
		recv = recv[:i]
	}
	fc.pos = c.selPos(fc, fc.pos)
	return fmt.Sprintf(fmtConstMethod, m[1], recv, c.ty(m[2]), m[3])
}

func (c *checker) ruleMethodValue(fc *failCtx, m []string) string {
	fc.pos = c.selPos(fc, fc.pos)
	return fmt.Sprintf(fmtMethodValue, fc.text)
}

func (c *checker) ruleNonStatic(fc *failCtx, m []string) string {
	return fmt.Sprintf(fmtNonStatic, c.calleeName(fc))
}

func (c *checker) ruleAddressOf(fc *failCtx, m []string) string {
	return fmt.Sprintf(fmtAddress, strings.TrimPrefix(fc.text, "&"), c.ty(m[1]))
}

func (c *checker) ruleNotAssignable(fc *failCtx, m []string) string {
	return fmt.Sprintf(fmtNotAssign, fc.text)
}

func (c *checker) ruleIncomplete(fc *failCtx, m []string) string {
	t := ""
	for _, s := range m[1:] {
		if s != "" {
			t = s
		}
	}
	return fmt.Sprintf(fmtIncomplete, c.ty(t))
}

func (c *checker) ruleDeleted(fc *failCtx, m []string) string {
	return fmt.Sprintf(fmtDeleted, fc.text, plainMsg(fc.msg))
}

func (c *checker) ruleCast(fc *failCtx, m []string) string {
	from, to := "", ""
	for i := 1; i+1 < len(m); i += 2 {
		if m[i] != "" {
			from, to = m[i], m[i+1]
		}
	}
	// The conversion T(x): x is the one argument of the call.
	text, desc := c.argText(&failCtx{call: fc.call, args: fc.args, argIdx: 0})
	if fc.call == nil || len(fc.call.Args) != 1 || fc.args == nil {
		text, desc = fc.text, "value of type "+c.ty(from)
	}
	target := c.ty(to)
	if fc.call != nil {
		if t := c.typeOfExpr(fc.call.Fun); t != "" {
			target = t
		}
	}
	return fmt.Sprintf(fmtConvert, text, desc, target)
}
