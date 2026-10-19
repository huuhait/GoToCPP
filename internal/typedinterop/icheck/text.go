package icheck

import (
	"regexp"
	"strings"
)

// Helpers for the text of expressions as the collector keeps it (Site.Text:
// the source after the loader's rewrite, with "C." in front of every name of
// a header package or a namespace) and for the type spellings in the
// compiler's messages.

// callSpans finds the call at the end of text ("f(a, b)"): where the callee
// ends (the offset of the opening parenthesis) and the byte span [start, end)
// of each argument, without the white space around it. ok is false when the
// text does not end with a call.
func callSpans(text string) (open int, spans [][2]int, ok bool) {
	end := len(strings.TrimRight(text, " \t"))
	if end == 0 || text[end-1] != ')' {
		return 0, nil, false
	}
	var stack []int
	match := -1
	for i := 0; i < end; i++ {
		switch c := text[i]; c {
		case '"', '\'', '`':
			j := skipQuoted(text, i)
			if j < 0 {
				return 0, nil, false
			}
			i = j
		case '(', '[', '{':
			stack = append(stack, i)
		case ')', ']', '}':
			if len(stack) == 0 {
				return 0, nil, false
			}
			o := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if i == end-1 && c == ')' {
				match = o
			}
		}
	}
	if match < 0 || len(stack) != 0 {
		return 0, nil, false
	}
	return match, argSpans(text, match+1, end-1), true
}

// splitCall splits the text of a call "f(a, b)" into the callee "f" and the
// argument texts. ok is false when the text does not end with a call.
func splitCall(text string) (callee string, args []string, ok bool) {
	open, spans, ok := callSpans(text)
	if !ok {
		return "", nil, false
	}
	for _, sp := range spans {
		args = append(args, text[sp[0]:sp[1]])
	}
	return text[:open], args, true
}

// skipQuoted returns the index of the closing quote of the literal that
// starts at i, or -1.
func skipQuoted(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch {
		case s[j] == '\\' && q != '`':
			j++
		case s[j] == q:
			return j
		}
	}
	return -1
}

// argSpans splits s[from:to] at the commas that are not inside brackets or
// quotes: the spans of the arguments, trimmed. An empty list has none.
func argSpans(s string, from, to int) [][2]int {
	if strings.TrimSpace(s[from:to]) == "" {
		return nil
	}
	var out [][2]int
	trim := func(a, b int) [2]int {
		for a < b && (s[a] == ' ' || s[a] == '\t') {
			a++
		}
		for b > a && (s[b-1] == ' ' || s[b-1] == '\t') {
			b--
		}
		return [2]int{a, b}
	}
	depth, start := 0, from
	for i := from; i < to; i++ {
		switch c := s[i]; c {
		case '"', '\'', '`':
			if j := skipQuoted(s, i); j >= 0 && j < to {
				i = j
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, trim(start, i))
				start = i + 1
			}
		}
	}
	return append(out, trim(start, to))
}

var (
	cPrefix = regexp.MustCompile(`\bC\.([A-Za-z_][A-Za-z_0-9]*)`)
	asConv  = regexp.MustCompile(`\bC\.As\[([^\[\]]*)\]\(`)
)

// cgoNames are the names of cgo's own C package: the C types and the helpers.
// They keep their "C." when the text of a use is turned back into what the user
// wrote (the loader writes C.name for the names of header packages too).
var cgoNames = map[string]bool{
	"char": true, "schar": true, "uchar": true, "short": true, "ushort": true, "int": true, "uint": true,
	"long": true, "ulong": true, "longlong": true, "ulonglong": true, "float": true, "double": true,
	"size_t": true, "ssize_t": true, "intptr_t": true, "uintptr_t": true, "int8_t": true, "uint8_t": true,
	"int16_t": true, "uint16_t": true, "int32_t": true, "uint32_t": true, "int64_t": true, "uint64_t": true,
	"wchar_t": true, "bool": true, "_Bool": true, "ptrdiff_t": true,
	"cstr": true, "CString": true, "GoString": true, "GoStringN": true, "GoBytes": true, "free": true,
}

// userText turns the text of a site back into what the user wrote: the loader
// wrote C.name for a name of a header package and C.std for a namespace, and
// check wrote C.As[T](x) for the conversion T(x) of a C/C++ value. hdrs maps the
// first name of such a path to its header package (shapes for C.area); a name
// that is a namespace or has no (single) header loses its "C.", except the names
// of cgo's own C package (C.int, C.cstr), which keep it.
func userText(s string, hdrs map[string]string) string {
	// C.As[*C.char](x) is (*C.char)(x); C.As[int](x) is int(x).
	s = asConv.ReplaceAllStringFunc(s, func(m string) string {
		t := asConv.FindStringSubmatch(m)[1]
		if strings.HasPrefix(t, "*") {
			return "(" + t + ")("
		}
		return t + "("
	})
	return cPrefix.ReplaceAllStringFunc(s, func(m string) string {
		name := m[2:]
		if h := hdrs[name]; h != "" {
			return h + "." + name
		}
		if cgoNames[name] || strings.HasPrefix(name, "struct_") || strings.HasPrefix(name, "union_") || strings.HasPrefix(name, "enum_") {
			return m
		}
		return name
	})
}

// A literal is how an untyped constant that the collector wrote as C++ reads
// in Go's words.
func describeLit(text string) string {
	t := strings.TrimSpace(text)
	for strings.HasPrefix(t, "(") && strings.HasSuffix(t, ")") {
		t = strings.TrimSpace(t[1 : len(t)-1])
	}
	switch {
	case t == "true" || t == "false":
		return "untyped bool constant"
	case t == "nullptr" || t == "NULL":
		return "untyped nil"
	case strings.HasPrefix(t, `"`):
		return "untyped string constant"
	case strings.HasPrefix(t, "'"):
		return "untyped rune constant"
	}
	body := strings.TrimLeft(t, "+-")
	if strings.HasPrefix(body, "0x") || strings.HasPrefix(body, "0X") {
		return "untyped int constant"
	}
	if strings.ContainsAny(body, ".eE") {
		return "untyped float constant"
	}
	return "untyped int constant"
}

var (
	akaRe   = regexp.MustCompile(` \(aka '[^']*'\)`)
	standRe = strings.NewReplacer(
		"gocpp_probe_internal_pr::string", "string",
	)
	identityRe = regexp.MustCompile(`std::type_identity_t<([^<>]*)>`)
	sliceRe    = regexp.MustCompile(`gocpp_probe_internal_pr::slice<([^<>]*)>`)
)

// plainType writes a type of the compiler's message in Go's words when it
// can't be mapped: "::" becomes ".", the stand-ins of the probe become
// string and []T.
func plainType(s string) string {
	s = akaRe.ReplaceAllString(s, "")
	s = identityRe.ReplaceAllString(s, "$1")
	s = standRe.Replace(s)
	for sliceRe.MatchString(s) {
		s = sliceRe.ReplaceAllString(s, "[]$1")
	}
	s = strings.ReplaceAll(s, "::", ".")
	return strings.TrimPrefix(strings.TrimSpace(s), ".")
}
