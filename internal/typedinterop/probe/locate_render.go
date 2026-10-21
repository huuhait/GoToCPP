package probe

import (
	"cmp"
	"slices"
	"strings"
)

// From the dump of a lookup to declarations (step 12.13c2): where each is, what its declaration
// is in C++, and its doc comment. The text is the compiler's spelling of the declaration, not
// the file's: macros (RLAPI), attributes and bodies are gone, typedef names are kept as the
// compiler prints them (size_type, FILE *), and the name is qualified like the Go source says.

// A Declaration is one declaration that a name stands for.
type Declaration struct {
	// Kind is function, method, constructor, destructor, type, template, variable, constant,
	// field or macro.
	Kind string `json:"kind"`

	// File is the absolute, normalized path of the header. Line is 0-based and Col is the
	// UTF-16 column of the name, as the language server protocol counts; Length is the
	// length of the name in code units.
	File   string `json:"file"`
	Line   int    `json:"line"`
	Col    int    `json:"col"`
	Length int    `json:"length"`

	// Signature is the declaration in C++; Doc the comment of the header, or "".
	Signature string `json:"signature"`
	Doc       string `json:"doc,omitempty"`
}

const (
	maxRenderedMembers = 24
	maxRenderedEnums   = 16
	maxOverloads       = 64 // a larger set is not shown: it is not a hover
	maxSignatureBytes  = 2 << 10
)

// A usingTarget is a name that a using declaration brought in: the declaration is the
// target's. Only a name that is written from the global scope can be followed.
type usingTarget struct {
	Scope string `json:"scope"`
	Name  string `json:"name"`
}

// renderer turns the groups of one run into declarations.
type renderer struct {
	set *sourceSet
}

type tree struct {
	nodes []dumpNode
	kids  [][]int
	tops  []int
}

func newTree(nodes []dumpNode) *tree {
	t := &tree{nodes: nodes, kids: make([][]int, len(nodes))}
	var stack []int
	for i, n := range nodes {
		for len(stack) > n.depth {
			stack = stack[:len(stack)-1]
		}
		if n.depth == 0 {
			t.tops = append(t.tops, i)
		} else if len(stack) == n.depth {
			parent := stack[n.depth-1]
			t.kids[parent] = append(t.kids[parent], i)
		}
		stack = append(stack, i)
	}
	return t
}

// children returns the direct children of node i with the given kinds (all when none).
func (t *tree) children(i int, kinds ...string) []int {
	var out []int
	for _, k := range t.kids[i] {
		if len(kinds) == 0 || slices.Contains(kinds, t.nodes[k].hdr.kind) {
			out = append(out, k)
		}
	}
	return out
}

// declarations renders the answer of a lookup for name. qual qualifies the names ("std",
// "geo::Widget"). ok is false when any declaration of the answer cannot be shown exactly: a
// set of overloads is answered completely or not at all.
func (r *renderer) declarations(group dumpGroup, qual string, members bool,
) (decls []Declaration, follow []usingTarget, ok bool) {
	t := newTree(group.nodes)
	if group.droppedTop || len(t.tops) > maxOverloads {
		return nil, nil, false // the set is not complete, or too large to show
	}
	for _, top := range t.tops {
		h := t.nodes[top].hdr
		if h.kind == "UsingShadowDecl" {
			continue // the using declaration before it names the target
		}
		if slices.Contains(h.pre, "implicit") && members {
			continue // the injected class name and the special members of a class
		}
		if h.kind == "UsingDecl" {
			if target, found := usingTargetOf(h); found && !members {
				follow = append(follow, target)
				continue
			}
			return nil, nil, false
		}
		if h.name != group.name || h.invalid {
			return nil, nil, false
		}
		decl, rendered := r.render(t, top, qual, group.partial)
		if !rendered {
			return nil, nil, false
		}
		decls = append(decls, decl)
	}
	slices.SortFunc(decls, func(a, b Declaration) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
	})
	decls = slices.CompactFunc(decls, func(a, b Declaration) bool {
		return a.File == b.File && a.Line == b.Line && a.Col == b.Col
	})
	return decls, follow, true
}

// usingTargetOf reads the name of a using declaration ("::std::string"). Only a name from the
// global scope is followed: a name relative to the namespace of the declaration cannot be
// looked up without that namespace's scope rules.
func usingTargetOf(h nodeHeader) (usingTarget, bool) {
	rooted, ok := strings.CutPrefix(h.name, "::")
	if !ok {
		return usingTarget{}, false
	}
	scope, name, found := lastCut(rooted, "::")
	if !found {
		return usingTarget{Name: rooted}, identRx.MatchString(rooted)
	}
	for _, part := range strings.Split(scope, "::") {
		if !identRx.MatchString(part) {
			return usingTarget{}, false
		}
	}
	return usingTarget{Scope: scope, Name: name}, identRx.MatchString(name)
}

// lastCut splits s around the last sep.
func lastCut(s, sep string) (before, after string, found bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}

func (r *renderer) render(t *tree, i int, qual string, partial bool) (Declaration, bool) {
	h := t.nodes[i].hdr
	var decl Declaration
	var ok bool
	switch h.kind {
	case "FunctionDecl", "CXXMethodDecl", "CXXConstructorDecl", "CXXDestructorDecl",
		"CXXConversionDecl":
		decl.Signature, ok = functionSignature(t, i, "", qual)
		decl.Kind = functionKind(h.kind)
	case "FunctionTemplateDecl":
		decl.Signature, decl.Kind, ok = templateFunction(t, i, qual)
	case "TypedefDecl":
		decl.Signature, ok = typedefSignature(h, qual), true
		decl.Kind = "type"
	case "TypeAliasDecl":
		decl.Signature, ok = aliasSignature(h, qual), true
		decl.Kind = "type"
	case "TypeAliasTemplateDecl":
		decl.Signature, ok = templateAlias(t, i, qual)
		decl.Kind = "template"
	case "CXXRecordDecl":
		decl.Signature, ok = classSignature(t, i, qual, partial)
		decl.Kind = "type"
	case "ClassTemplateDecl":
		decl.Signature, ok = templateRecord(t, i, qual)
		decl.Kind = "template"
	case "EnumDecl":
		decl.Signature, ok = enumDeclaration(t, i, qual)
		decl.Kind = "type"
	case "VarDecl":
		decl.Signature, ok = variableSignature(t, i, qual), true
		decl.Kind = "variable"
	case "EnumConstantDecl":
		decl.Signature, ok = enumeratorSignature(t, i, qual), true
		decl.Kind = "constant"
	case "FieldDecl":
		decl.Signature, ok = fieldSignature(h, qual), true
		decl.Kind = "field"
	}
	if !ok || decl.Signature == "" || len(decl.Signature) > maxSignatureBytes {
		return Declaration{}, false
	}
	line, col, length, placed := r.set.place(h.at, h.name)
	if !placed {
		return Declaration{}, false
	}
	decl.File, decl.Line, decl.Col, decl.Length = h.at.file, line, col, length
	decl.Doc = r.set.docOf(h.end, h.at, commentOf(t, i))
	return decl, true
}

// commentOf returns the comment that the compiler attached to the declaration at i or, for a
// template, to its pattern.
func commentOf(t *tree, i int) *dumpNode {
	if c := t.children(i, "FullComment"); len(c) > 0 {
		return &t.nodes[c[0]]
	}
	for _, k := range t.kids[i] {
		switch t.nodes[k].hdr.kind {
		case "FunctionDecl", "CXXRecordDecl", "TypeAliasDecl":
			if c := t.children(k, "FullComment"); len(c) > 0 {
				return &t.nodes[c[0]]
			}
		}
	}
	return nil
}

func functionKind(kind string) string {
	switch kind {
	case "CXXMethodDecl", "CXXConversionDecl":
		return "method"
	case "CXXConstructorDecl":
		return "constructor"
	case "CXXDestructorDecl":
		return "destructor"
	}
	return "function"
}

// qualified joins the qualification and the name.
func qualified(qual, name string) string {
	if qual == "" {
		return name
	}
	return qual + "::" + name
}

// join writes a type and a declared name: "const char *" and "title" are "const char *title".
func join(typ, name string) string {
	if name == "" {
		return typ
	}
	if strings.HasSuffix(typ, "*") || strings.HasSuffix(typ, "&") {
		return typ + name
	}
	return typ + " " + name
}

// declare writes a declaration of name with the type typ, with the name inside the type where
// C++ puts it: "void (*)(int)" is "void (*name)(int)", "char[4]" is "char name[4]".
func declare(typ, name string) string {
	if name == "" {
		return typ
	}
	for _, open := range []string{"(*)", "(&)", "(&&)"} {
		if i := strings.Index(typ, open); i >= 0 {
			return typ[:i+len(open)-1] + name + typ[i+len(open)-1:]
		}
	}
	if strings.HasSuffix(typ, "]") {
		if i := strings.IndexByte(typ, '['); i > 0 {
			return strings.TrimSpace(typ[:i]) + " " + name + typ[i:]
		}
	}
	return join(typ, name)
}

// splitFunctionType splits "R (P1, P2) const noexcept" into the result, the parameters and
// what follows them. A result that is a function pointer or an array reference, and a type with
// more than one group of parentheses outside template arguments, is not split.
func splitFunctionType(typ string) (result, params, tail string, ok bool) {
	depth, angle := 0, 0
	start := -1
	for i := 0; i < len(typ); i++ {
		switch typ[i] {
		case '<':
			if depth == 0 {
				angle++
			}
		case '>':
			if depth == 0 && angle > 0 {
				angle--
			}
		case '(':
			if depth == 0 && angle == 0 && start < 0 {
				start = i
			}
			depth++
		case ')':
			depth--
			if depth == 0 && angle == 0 && start >= 0 {
				params = typ[start+1 : i]
				tail = strings.TrimSpace(typ[i+1:])
				result = strings.TrimSpace(typ[:start])
				if strings.HasPrefix(params, "*") || strings.HasPrefix(params, "&") ||
					strings.HasPrefix(params, "^") ||
					strings.Contains(tail, "(") && !strings.HasPrefix(tail, "noexcept") {
					return "", "", "", false
				}
				return result, params, tail, true
			}
			if depth < 0 {
				return "", "", "", false
			}
		}
	}
	return "", "", "", false
}

// countParams counts the parameters of a parameter list as the compiler prints a function type.
func countParams(params string) (count int, variadic bool) {
	params = strings.TrimSpace(params)
	if params == "" || params == "void" {
		return 0, false
	}
	depth := 0
	count = 1
	for i := 0; i < len(params); i++ {
		switch params[i] {
		case '(', '<', '[':
			depth++
		case ')', '>', ']':
			depth--
		case ',':
			if depth == 0 {
				count++
			}
		}
	}
	if strings.HasSuffix(params, "...") {
		variadic = true
		if strings.TrimSpace(strings.TrimSuffix(params, "...")) == "" {
			return 0, true
		}
		if strings.HasSuffix(strings.TrimSpace(strings.TrimSuffix(params, "...")), ",") {
			count--
		}
	}
	return count, variadic
}

// literalText reads the value of a literal node ("'int' 2", "'const char[2]' lvalue \"x\"").
func literalText(n dumpNode) (string, bool) {
	rest := n.hdr.rest
	switch n.hdr.kind {
	case "IntegerLiteral", "FloatingLiteral", "CXXBoolLiteralExpr", "CharacterLiteral":
		_, value, ok := readQuoted(rest)
		return strings.TrimSpace(value), ok && strings.TrimSpace(value) != ""
	case "StringLiteral":
		_, value, ok := readQuoted(rest)
		value = strings.TrimPrefix(strings.TrimSpace(value), "lvalue ")
		return value, ok && strings.HasPrefix(value, `"`)
	case "CXXNullPtrLiteralExpr":
		return "nullptr", true
	}
	return "", false
}

// valueBelow returns the value that the compiler computed below node i (a ConstantExpr has
// "value: Int 65") or the first literal below it.
func valueBelow(t *tree, i int) (string, bool) {
	for j := i + 1; j < len(t.nodes) && t.nodes[j].depth > t.nodes[i].depth; j++ {
		if text, ok := strings.CutPrefix(t.nodes[j].text, "value: "); ok {
			kind, value, _ := strings.Cut(text, " ")
			if kind == "Int" || kind == "Float" {
				return value, value != ""
			}
		}
		if value, ok := literalText(t.nodes[j]); ok {
			return value, true
		}
	}
	return "", false
}

// parameterList writes the parameters of the function at i. The ParmVarDecl children have the
// names and the default arguments; the type of the function says how many there must be.
func parameterList(t *tree, i int, params string) string {
	want, variadic := countParams(params)
	parms := t.children(i, "ParmVarDecl")
	if len(parms) != want {
		return strings.TrimSpace(params) // the names are not known: the types are
	}
	var parts []string
	for _, p := range parms {
		h := t.nodes[p].hdr
		text := declare(h.typ, h.name)
		if slices.Contains(h.post, "cinit") || slices.Contains(h.post, "callinit") {
			if value, ok := valueBelow(t, p); ok {
				text += " = " + value
			} else {
				text += " = ..."
			}
		}
		parts = append(parts, text)
	}
	if variadic {
		parts = append(parts, "...")
	}
	return strings.Join(parts, ", ")
}

// functionSignature writes the declaration of the function at i, which is the function itself
// or the pattern of a template: "int Widget::size(int x) const".
func functionSignature(t *tree, i int, templateHeader, qual string) (string, bool) {
	h := t.nodes[i].hdr
	result, params, tail, ok := splitFunctionType(h.typ)
	if !ok {
		return "", false
	}
	list := parameterList(t, i, params)
	var b strings.Builder
	if templateHeader != "" {
		b.WriteString(templateHeader + "\n")
	}
	for _, word := range []string{"static", "virtual", "explicit"} {
		if slices.Contains(h.post, word) || slices.Contains(h.pre, word) {
			b.WriteString(word + " ")
		}
	}
	if slices.Contains(h.pre, "constexpr") || slices.Contains(h.post, "constexpr") {
		b.WriteString("constexpr ")
	}
	name := qualified(qual, h.name)
	switch h.kind {
	case "CXXConstructorDecl", "CXXDestructorDecl", "CXXConversionDecl":
		b.WriteString(name)
	default:
		b.WriteString(join(result, name))
	}
	b.WriteString("(" + list + ")")
	if tail != "" {
		b.WriteString(" " + tail)
	}
	if slices.Contains(h.post, "pure") {
		b.WriteString(" = 0")
	}
	return b.String(), true
}

// templateParams writes "template <typename T, int N>" from the parameter children of the
// template at i. Default arguments are not written.
func templateParams(t *tree, i int) string {
	var parts []string
	for _, k := range t.kids[i] {
		h := t.nodes[k].hdr
		switch h.kind {
		case "TemplateTypeParmDecl":
			keyword := "typename"
			if slices.Contains(h.pre, "class") {
				keyword = "class"
			}
			if slices.Contains(h.pre, "...") {
				keyword += "..."
			}
			parts = append(parts, join(keyword, h.name))
		case "NonTypeTemplateParmDecl":
			name := h.name
			if len(h.post) > 0 {
				name = h.post[len(h.post)-1] // the dump prints "'int' depth 0 index 0 N"
				if _, _, numeric := leadingInt(name); numeric {
					name = ""
				}
			}
			parts = append(parts, declare(h.typ, name))
		case "TemplateTemplateParmDecl":
			parts = append(parts, join("template <...> class", h.name))
		}
	}
	return "template <" + strings.Join(parts, ", ") + ">"
}

func templateFunction(t *tree, i int, qual string) (string, string, bool) {
	patterns := t.children(i, "FunctionDecl")
	if len(patterns) == 0 {
		return "", "", false
	}
	sig, ok := functionSignature(t, patterns[0], templateParams(t, i), qual)
	return sig, "template", ok
}

func templateAlias(t *tree, i int, qual string) (string, bool) {
	patterns := t.children(i, "TypeAliasDecl")
	if len(patterns) == 0 {
		return "", false
	}
	return templateParams(t, i) + "\n" + aliasSignature(t.nodes[patterns[0]].hdr, qual), true
}

func templateRecord(t *tree, i int, qual string) (string, bool) {
	patterns := t.children(i, "CXXRecordDecl")
	for _, p := range patterns {
		pattern := t.nodes[p].hdr
		if slices.Contains(pattern.pre, "implicit") {
			continue
		}
		// The lookup answers with the latest declaration of the template. After the
		// definition that is a redeclaration without it, whose place is not the
		// definition's, and the dump does not say where the definition is.
		if !pattern.definition && (pattern.prev || t.nodes[i].hdr.prev) {
			return "", false
		}
		head := recordHead(pattern, qual)
		return templateParams(t, i) + "\n" + head + ";", true
	}
	return "", false
}

// classSignature is the declaration of a class: with its members when the dump is of the
// definition, and "struct Name;" for a class that is only declared. A declaration that comes
// after others and is not the definition may have the definition before it somewhere else: no
// answer then, rather than a forward declaration shown as an empty class.
func classSignature(t *tree, i int, qual string, partial bool) (string, bool) {
	h := t.nodes[i].hdr
	if h.definition {
		return recordSignature(t, i, qual, partial), true
	}
	if h.prev {
		return "", false
	}
	return recordHead(h, qual) + ";", true
}

// enumDeclaration is enumSignature for an enum that the dump has the enumerators of. An enum
// without any is an opaque declaration (enum E : int;) or one that follows its definition.
func enumDeclaration(t *tree, i int, qual string) (string, bool) {
	h := t.nodes[i].hdr
	if len(t.children(i, "EnumConstantDecl")) == 0 && h.prev {
		return "", false
	}
	return enumSignature(t, i, qual), true
}

// aliasSignature writes "using name = type;". The type is the canonical one: the written one is
// relative to the scope of the declaration.
func aliasSignature(h nodeHeader, qual string) string {
	target := h.typ
	if h.canon != "" {
		target = h.canon
	}
	return "using " + qualified(qual, h.name) + " = " + target + ";"
}

// typedefSignature writes "typedef type name;". A typedef of a tag keeps its elaborated spelling
// (struct Vector2), which names the same type anywhere.
func typedefSignature(h nodeHeader, qual string) string {
	target := h.typ
	if h.canon != "" && !startsWithTag(h.typ) {
		target = h.canon
	}
	return "typedef " + declare(target, qualified(qual, h.name)) + ";"
}

func startsWithTag(typ string) bool {
	for _, tag := range []string{"struct ", "union ", "enum ", "class "} {
		if strings.HasPrefix(typ, tag) {
			return true
		}
	}
	return false
}

// recordHead writes "struct Name" or "class ns::Name" and the base classes.
func recordHead(h nodeHeader, qual string) string {
	tag := "class"
	for _, word := range h.pre {
		if word == "struct" || word == "class" || word == "union" {
			tag = word
		}
	}
	return tag + " " + qualified(qual, h.name)
}

// recordSignature writes the declaration of a class: its public data members and member
// functions, as far as the dump has them.
func recordSignature(t *tree, i int, qual string, partial bool) string {
	h := t.nodes[i].hdr
	head := recordHead(h, qual)
	var bases []string
	for _, k := range t.kids[i] {
		access, typ, ok := baseOf(t.nodes[k])
		if ok {
			bases = append(bases, access+typ)
		}
	}
	if len(bases) > 0 {
		head += " : " + strings.Join(bases, ", ")
	}
	public := !slices.Contains(h.pre, "class")
	var lines []string
	omitted := partial
	for _, k := range t.kids[i] {
		m := t.nodes[k].hdr
		if m.kind == "AccessSpecDecl" {
			public = m.name == "public"
			continue
		}
		if slices.Contains(m.pre, "implicit") {
			continue
		}
		if !public {
			continue
		}
		var line string
		switch m.kind {
		case "FieldDecl":
			line = fieldLine(m)
		case "CXXMethodDecl", "CXXConstructorDecl", "CXXDestructorDecl", "CXXConversionDecl":
			sig, ok := functionSignature(t, k, "", "")
			if !ok {
				omitted = true
				continue
			}
			line = sig + ";"
		default:
			continue
		}
		if len(lines) >= maxRenderedMembers {
			omitted = true
			break
		}
		lines = append(lines, "    "+line)
	}
	if len(lines) == 0 && !omitted {
		return head + " {}"
	}
	if omitted {
		lines = append(lines, "    // ...")
	}
	return head + " {\n" + strings.Join(lines, "\n") + "\n}"
}

// baseOf reads a base class line of a dump: "public 'geo::Base'".
func baseOf(n dumpNode) (access, typ string, ok bool) {
	switch n.hdr.kind {
	case "public", "protected", "private", "virtual":
	default:
		return "", "", false
	}
	i := strings.IndexByte(n.text, '\'')
	if i < 0 {
		return "", "", false
	}
	typ, _, ok = readQuoted(n.text[i:])
	return n.text[:i], typ, ok
}

func fieldLine(h nodeHeader) string {
	return declare(h.typ, h.name) + ";"
}

func fieldSignature(h nodeHeader, qual string) string {
	return declare(h.typ, qualified(qual, h.name))
}

// enumSignature writes "enum Name { A, B = 5 }" with the enumerators of the dump.
func enumSignature(t *tree, i int, qual string) string {
	h := t.nodes[i].hdr
	head := "enum "
	for _, word := range h.pre {
		if word == "class" || word == "struct" {
			head += "class "
		}
	}
	head += qualified(qual, h.name)
	if h.typ != "" {
		head += " : " + h.typ
	}
	var items []string
	omitted := false
	for _, k := range t.children(i, "EnumConstantDecl") {
		if len(items) >= maxRenderedEnums {
			omitted = true
			break
		}
		item := t.nodes[k].hdr.name
		if value, ok := valueBelow(t, k); ok {
			item += " = " + value
		}
		items = append(items, "    "+item+",")
	}
	if omitted {
		items = append(items, "    // ...")
	}
	if len(items) == 0 {
		return head + ";" // an opaque declaration: enum class Kind : int;
	}
	return head + " {\n" + strings.Join(items, "\n") + "\n}"
}

func variableSignature(t *tree, i int, qual string) string {
	h := t.nodes[i].hdr
	var b strings.Builder
	for _, word := range []string{"extern", "static", "inline", "constexpr"} {
		if slices.Contains(h.post, word) || slices.Contains(h.pre, word) {
			b.WriteString(word + " ")
		}
	}
	b.WriteString(declare(h.typ, qualified(qual, h.name)))
	if value, ok := valueBelow(t, i); ok {
		b.WriteString(" = " + value)
	}
	return b.String()
}

func enumeratorSignature(t *tree, i int, qual string) string {
	h := t.nodes[i].hdr
	text := join(h.typ, qualified(qual, h.name))
	if value, ok := valueBelow(t, i); ok {
		text += " = " + value
	}
	return text
}
