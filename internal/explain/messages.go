package explain

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/huuhait/gocpp/internal/check"
)

// The messages that gocpp explain explains (Phase 13 step 13.5): the errors of the family "X is
// not supported yet (Phase NN)" / "X is not available yet (Phase NN)", which say that the
// program uses something that a later phase of the plan builds, and the few errors about
// the name C and about headers that people meet first.
//
// gocpp has no error codes: a message is its own name. FindMessage takes the text of an error
// (with or without the "file:line:col: " in front) and returns the explanations that match.
//
// Two sources fill the table. The phase gates of internal/check (check.Gates) become entries by
// themselves, so a new gate needs no edit here. The messages of the lowering, the loader and
// the driver are curated below; TestMessagesCoverSources scans the source of those packages and
// fails for a message of the family that has no entry.

// A Message is the explanation of one error message, or of a family of them.
type Message struct {
	ID      string // a stable name: gate/feature/interfaces, lower/channel-targs
	Source  string // who reports it: "checker", "lowering", "loader", "driver"
	Text    string // the exact message (generated entries), else ""
	Pattern string // the message with %s for the parts that vary (curated entries), else ""
	Phase   int    // the phase that builds what the message says is missing; 0 if none; check.Never
	Title   string
	Why     string // paragraphs separated by blank lines
	Fix     string // what to do now
	Topic   string // an explain topic that describes the mechanism, or ""
	Spec    string // a section of the specs

	// Fallback says that the entry is a catch-all for the messages of a phase ("%s (Phase 07)"): it
	// answers a user who asks about a message that has no entry of its own, and does not count for
	// the completeness test (TestMessagesCoverSources).
	Fallback bool

	re *regexp.Regexp
}

// compile returns the regular expression of the message: the text or the pattern, with the
// part before the message (a position, a command name) ignored, and the end fixed.
func (m *Message) compile() *regexp.Regexp {
	var src string
	if m.Text != "" {
		// An exact text starts the message, after the name of the command that reports it.
		return regexp.MustCompile(`(?s)^(?:gocpp \w+: |not supported yet: )?` + regexp.QuoteMeta(m.Text) + `$`)
	} else {
		parts := strings.Split(m.Pattern, "%s")
		for i := range parts {
			parts[i] = regexp.QuoteMeta(parts[i])
		}
		src = strings.Join(parts, `.+`)
	}
	return regexp.MustCompile(`(?s)(?:^|[ :])` + src + `$`)
}

// Matches reports whether the message text msg is an instance of m.
func (m *Message) Matches(msg string) bool {
	return m.re.MatchString(strings.TrimSpace(msg))
}

// Phase names, from the plan: what a message that says "Phase NN" is waiting for.
var phaseNames = map[int]string{
	2: "walking skeleton", 3: "core language", 4: "C++ interop", 5: "interfaces and errors",
	6: "packages and build", 7: "generics", 8: "concurrency", 9: "standard library and testing",
	10: "must and macros", 11: "language server", 12: "typed interop", 13: "tooling",
	14: "Windows", 15: "hardening and release",
}

// PhaseName returns the name of phase n, or "".
func PhaseName(n int) string { return phaseNames[n] }

// Messages returns the table: the curated entries, then the gates of the checker. The slice is
// built once and shared: do not change it.
func Messages() []Message {
	msgOnce.Do(func() {
		msgTable = append(append(append([]Message(nil), promotedMessages...), curated...), gateMessages()...)
		for i := range msgTable {
			msgTable[i].re = msgTable[i].compile()
		}
	})
	return msgTable
}

var (
	msgTable []Message
	msgOnce  sync.Once
)

// FindMessage returns the entries that explain msg: the text of an error, optionally with the
// "file:line:col: " that gocpp prints in front, or "gocpp cmd: " for a command that is not built.
// The entries with the exact text answer; if there are none, the most specific patterns do.
func FindMessage(msg string) []Message {
	msg = StripPosition(msg)
	var exact, general []Message
	best := 0
	for _, m := range Messages() {
		if !m.Matches(msg) {
			continue
		}
		if m.Text != "" {
			exact = append(exact, m)
			continue
		}
		// Of the patterns, only the most specific ones (the most fixed words) answer: "%s
		// (Phase 07)" is the fallback of "%s.%s is not available yet (Phase 07)".
		n := len(strings.ReplaceAll(m.Pattern, "%s", ""))
		switch {
		case n > best:
			best, general = n, []Message{m}
		case n == best:
			general = append(general, m)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return general
}

var positionRx = regexp.MustCompile(`^(?:[A-Za-z]:[\\/])?(?:\./)?[^\s:]*\.(?:gp|go|c|cc|cpp|h|hpp|mod):\d+(?::\d+)?:\s*`)

// StripPosition removes the "file.gp:5:12: " that starts a compiler message.
func StripPosition(msg string) string {
	msg = strings.TrimSpace(msg)
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = strings.TrimSpace(msg[:i])
	}
	return positionRx.ReplaceAllString(msg, "")
}

// LooksLikeMessage reports whether arg, an argument of gocpp explain, reads as an error
// message and not as a topic name: it has a space, or says Phase.
func LooksLikeMessage(arg string) bool {
	return strings.ContainsAny(arg, " \t") || strings.Contains(arg, "(Phase")
}

// gateMessages makes the entries of the phase gates (check.Gates).
func gateMessages() []Message {
	var out []Message
	for _, g := range check.Gates() {
		m := Message{
			ID:     fmt.Sprintf("gate/%s/%s", g.Kind, g.Subject),
			Source: "checker",
			Text:   g.Message,
			Phase:  g.Phase,
		}
		m.Title, m.Why, m.Fix, m.Topic, m.Spec = describeGate(g)
		out = append(out, m)
	}
	return out
}

// gateTopics link a gate to the explain topic of the mechanism it gates, by subject.
var gateTopics = map[string]string{
	"composite types": "slices", "pointers": "boxing", "methods": "methods", "function values": "closures",
	"closures": "closures", "defer": "defer", "type declarations": "names", "string slicing": "strings",
	"range over strings": "range", "conversions between strings and bytes, runes or integers": "strings",
	"package-level variables": "initialization", "init functions": "initialization",
	"header imports": "interop", "C++ namespaces": "interop", "C types and helpers": "interop",
	"interfaces": "interfaces", "any values": "interfaces", "error values": "interfaces",
	"type assertions": "interfaces", "type switches": "interfaces", "struct embedding": "embedding",
	"panic with a value of a non-basic type": "panics", "generic functions": "generics", "generic types": "generics",
	"range over functions": "range", "go statements": "goroutines", "channels": "channels",
	"select statements": "select", "a type defined as weak.Pointer[T]": "generics", "promoted fields and methods of several embedded C/C++ types": "embedding",
}

// describeGate writes the explanation of a gate from its table row.
func describeGate(g check.Gate) (title, why, fix, topic, spec string) {
	what := g.Subject
	switch g.Kind {
	case "package":
		what = "package " + g.Subject
	case "builtin":
		what = "the built-in function " + g.Subject
	case "verb":
		what = "the format verb " + g.Subject
	}
	title = g.Message
	spec = "feature-matrix.md"
	topic = gateTopics[g.Subject]
	var b strings.Builder
	if g.Phase == check.Never {
		fmt.Fprintf(&b, "gocpp never has %s, and the checker says so before the C++ is written, so that nobody gets broken C++.", what)
		why = b.String()
		fix = "Write the program without it. docs/spec/feature-matrix.md lists the Go features that gocpp leaves out on purpose, and why."
		return
	}
	name := PhaseName(g.Phase)
	if name != "" {
		name = " (" + name + ")"
	}
	fmt.Fprintf(&b, "The checker rejects %s because the phase that builds it, Phase %02d%s, is not finished in this gocpp. Each phase of the plan adds a part of the language; the checker keeps a program inside the part that is built, so that nobody gets broken C++ or a wrong program, and the message names the phase that brings what is missing.", what, g.Phase, name)
	if g.Notes != "" {
		fmt.Fprintf(&b, "\n\nWhat the gate covers: %s", g.Notes)
	}
	why = b.String()
	switch g.Kind {
	case "package":
		fix = "Use a package that this gocpp has, or wait for Phase " + fmt.Sprintf("%02d", g.Phase) + ". gocpp check -h and docs/spec/feature-matrix.md list the standard packages and their phases."
	case "member":
		fix = "Use another member of the package, or wait for Phase " + fmt.Sprintf("%02d", g.Phase) + "."
	case "builtin":
		fix = "Write the same with the statements that are built, or wait for Phase " + fmt.Sprintf("%02d", g.Phase) + "."
	case "verb":
		fix = "Format the value another way (strconv, or a verb that is built), or wait for Phase " + fmt.Sprintf("%02d", g.Phase) + "."
	default:
		fix = "Write the program without it, or wait for Phase " + fmt.Sprintf("%02d", g.Phase) + ". docs/spec/feature-matrix.md has the whole table of features and phases."
	}
	return
}

// curated is the table of the messages that are not gates. Each Pattern ends the message (the
// matcher ignores what is in front of it).
var curated = []Message{
	{
		ID: "lower/generic-type-cpp-arg", Source: "lowering", Phase: 12,
		Pattern: "an instance of a generic type with a C/C++ type argument (%s) is not supported yet: the type arguments of a generic type are Go types",
		Title:   "A C/C++ type as a type argument of a generic type",
		Why:     "An instance of a generic type is one struct with its own descriptor, shared by the whole program and named by Go's text (main.Box[int]). go/types cannot spell a C/C++ type in that name, and the instance would have to hold values the descriptor cannot describe.",
		Fix:     "Use a Go type as the argument, or a generic function (its C/C++ type arguments are deduced by C++), or wrap the C/C++ value in a struct of your own.",
		Topic:   "generics",
	},
	{
		ID: "lower/generic-type-cpp-arg-func", Source: "lowering", Phase: 12,
		Pattern: "an instance of the generic type %s with a C/C++ type argument of a generic function is not supported yet: the type arguments of a generic type are Go types",
		Title:   "A generic function that makes an instance of a generic type of a C/C++ type argument",
		Why:     "The function is lowered once for each list of type arguments, and one of them is a C/C++ type that C++ deduces. The generic type it instantiates with that type would need a struct and a descriptor named by Go's text, which the C/C++ type does not have.",
		Fix:     "Call the function with Go types, or keep the C/C++ value out of the generic type (a field of type any, or a parameter of the function).",
		Topic:   "generics",
	},
	{
		ID: "lower/generic-anon-struct-cpp-arg", Source: "lowering", Phase: 12,
		Pattern: "an unnamed struct type with a C/C++ type argument of a generic function is not supported yet: declare a named struct type",
		Title:   "An unnamed struct type that mentions a C/C++ type argument",
		Why:     "An unnamed struct type is one shared C++ struct for the whole program, named by its fields. A field of a C/C++ type argument that C++ deduces has no name the program could share.",
		Fix:     "Declare a named struct type for it, or use the C/C++ type argument outside the struct.",
		Topic:   "generics",
	},
	{
		ID: "lower/generic-type-not-available", Source: "lowering", Phase: 7,
		Pattern: "%s.%s is not available yet",
		Title:   "A generic type of a package that is not lowered",
		Why:     "An instance of a generic type is made by the package that declares the type. This package is not one the program lowers (a standard package that is not ported).",
		Fix:     "Wait for the package, or declare the type yourself.",
		Topic:   "generics",
	},
	{
		ID: "lower/generic-cpp-names", Source: "lowering", Phase: 12,
		Pattern: "%s.%s uses C/C++ names, so it can only be instantiated in its own package (Phase 12)",
		Title:   "A generic function that uses C/C++ names",
		Why:     "A generic function is lowered again for each set of type arguments, in the package that uses it. If its body names C or C++ declarations of its own package (headers it includes), the other package would not see them.",
		Fix:     "Call it only from its own package, or wrap it in a function of that package that is not generic.",
		Topic:   "generics",
	},
	{
		ID: "lower/channel-targs", Source: "lowering", Phase: 12,
		Text:  "channel types as C++ template arguments are not available yet (Phase 12)",
		Title: "A channel as a C++ template argument",
		Why:   "A C++ template argument is a C++ type. A Go channel is a gp::chan, a runtime type that C++ templates do not know how to describe until typed interop (Phase 12).",
		Fix:   "Pass a func or a plain value to the C++ template, and keep the channel in Go code.",
		Topic: "interop",
	},
	{
		ID: "lower/iface-targs", Source: "lowering", Phase: 12,
		Text:  "interface types as C++ template arguments are not available yet (typed interop: Phase 12)",
		Title: "An interface as a C++ template argument",
		Why:   "An interface value is a gp::iface of the Go method set; C++ templates need a C++ type, which typed interop (Phase 12) will map.",
		Fix:   "Use a concrete type as the template argument.",
		Topic: "interop",
	},
	{
		ID: "lower/cpp-macro-array-len", Source: "lowering", Phase: 12,
		Pattern: "a C/C++ macro in an array length (%s) needs typed interop (install clang++ 17 or newer, and do not use -interop=opaque); or write the number, such as [260]windows.WCHAR for MAX_PATH",
		Title:   "A C/C++ macro in an array length",
		Why:     "An array length must be a constant the Go type checker knows. A C/C++ macro such as MAX_PATH is only known to the C++ compiler. Typed interop (-interop=typed, or the default auto mode when clang++ is installed) probes the constant macros of a package with clang first and puts their values into the program before type checking, so [cstdio.BUFSIZ]byte works there. This message is what opaque mode (-interop=opaque), a build without clang++ and a cross build for another system say (their headers are the host's, not the target's).",
		Fix:     "Write the number: [260]windows.WCHAR instead of [windows.MAX_PATH]windows.WCHAR, or declare a Go constant with the value.",
		Topic:   "interop",
	},
	{
		ID: "lower/cpp-in-iface", Source: "lowering", Phase: 12,
		Pattern: "a C/C++ value in an interface with methods (%s) is not available yet: only interfaces without methods hold C/C++ values until Phase 12 (typed interop)",
		Title:   "A C/C++ value in an interface with methods",
		Why:     "A C/C++ value can be held by an interface without methods (any): it is copied into a box with a descriptor made from its C++ type. An interface with methods needs a method table, and C/C++ types have no Go methods until typed interop (Phase 12).",
		Fix:     "Wrap the C/C++ value in a Go struct that has the methods, and put the struct in the interface.",
		Topic:   "interop",
	},
	{
		ID: "loader/derive-no-names", Source: "loader", Phase: 10,
		Pattern: "#Derive needs the names of the generators: #Derive(String, Equal, Clone, JSON)",
		Title:   "#Derive with no generator",
		Why:     "#Derive is written with the generators it should run: String, Equal, Clone and JSON.",
		Fix:     "Write #Derive(String), #Derive(Equal, Clone), ...",
		Spec:    "language.md#derive",
	},
	{
		ID: "loader/derive-unknown", Source: "loader", Phase: 10,
		Pattern: "#Derive: %s is not the name of a generator (String, Equal, Clone or JSON)",
		Title:   "An unknown generator in #Derive",
		Why:     "The generators are built in: String, Equal, Clone and JSON, written as plain names. Your own generators on types are on the backlog.",
		Fix:     "Use one of the four names, or write the method by hand.",
		Spec:    "language.md#derive",
	},
	{
		ID: "loader/derive-twice", Source: "loader", Phase: 10,
		Pattern: "#Derive(%s) is written twice",
		Title:   "A generator written twice",
		Why:     "A generator adds its methods once, so naming it twice in one line (or in two lines above one type) would declare the method twice.",
		Fix:     "Remove one of them.",
		Spec:    "language.md#derive",
	},
	{
		ID: "loader/derive-group", Source: "loader", Phase: 10,
		Pattern: "#Derive on a group of types: write it on top of one type declaration",
		Title:   "#Derive on type ( ... )",
		Why:     "#Derive applies to the one type that is declared right below it. A group declaration type ( A ...; B ... ) has several, so it is not clear which one the line means.",
		Fix:     "Write the types as separate declarations, each with its own #Derive line.",
		Spec:    "language.md#derive",
	},
	{
		ID: "loader/derive-alias", Source: "loader", Phase: 10,
		Pattern: "#Derive on %s, which is an alias: write it on top of the type that %s stands for",
		Title:   "#Derive on an alias",
		Why:     "An alias is another name for a type, and a method belongs to the type: the methods would be declared on the type that the alias stands for, which may be in another package.",
		Fix:     "Write #Derive on the type that the alias stands for.",
		Spec:    "language.md#derive",
	},
	{
		ID: "loader/derive-exists", Source: "loader", Phase: 10,
		Pattern: "#Derive(%s): the type %s has a method %s already (declared at %s): remove one of them",
		Title:   "A derived method that exists",
		Why:     "A type has one method of each name. Either the method that you wrote or the generator has to go.",
		Fix:     "Remove the generator from the line (#Derive(Equal) instead of #Derive(Equal, Clone) if you wrote Clone), or delete your method.",
		Spec:    "language.md#derive",
	},
	{
		ID: "loader/derive-kind", Source: "loader", Phase: 10,
		Pattern: "#Derive(%s): %s is a pointer type, which cannot have methods",
		Title:   "#Derive on a pointer type",
		Why:     "Go does not allow methods on a named pointer type.",
		Fix:     "Derive on the type that it points to.",
		Spec:    "language.md#derive",
	},
	{
		ID: "loader/derive-kind-string", Source: "loader", Phase: 10,
		Pattern: "#Derive(String): %s is not an integer type with constants",
		Title:   "String on a type that cannot have it",
		Why:     "String writes the names of the constants of an integer type (an enumeration), as the stringer tool does. fmt already prints a struct field by field (with the plus flag it shows the names), and for another type the method is yours to write.",
		Fix:     "Write the method by hand.",
		Spec:    "language.md#derive",
	},
	{
		ID: "loader/derive-string-struct", Source: "loader", Phase: 10,
		Pattern: "#Derive(String): %s is a struct: fmt prints a struct by itself (with the plus flag it shows the field names), and String is for integer types with constants",
		Title:   "String on a struct",
		Why:     "String writes the names of the constants of an integer type (an enumeration), as the stringer tool does. fmt already prints a struct field by field (with the plus flag it shows the names).",
		Fix:     "Print the struct with fmt, or write String by hand.",
		Spec:    "language.md#derive",
	},
	{
		ID: "loader/derive-kind-json", Source: "loader", Phase: 10,
		Pattern: "#Derive(JSON): %s is not a struct type (encoding/json handles slices, arrays and maps by itself)",
		Title:   "JSON on a type that is not a struct",
		Why:     "#Derive(JSON) writes MarshalJSON and UnmarshalJSON for the fields of a struct. encoding/json already handles slices, arrays and maps of values, and for an enumeration it would print the number, which surprises people.",
		Fix:     "Derive String and write MarshalText for an enumeration; use the type as it is otherwise.",
		Spec:    "language.md#derive",
	},
	{
		ID: "loader/derive-blank", Source: "loader", Phase: 10,
		Pattern: "#Derive on the blank type _: the methods need a name to belong to",
		Title:   "#Derive on the type _",
		Why:     "A method belongs to a named type; the blank identifier names nothing.",
		Fix:     "Give the type a name.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-eq-signature", Source: "checker", Phase: 10,
		Pattern: "#Derive(Equal): %s has the type %s, whose Equal method does not have the signature Equal(%s) bool: change it or write the comparison by hand",
		Title:   "An Equal method with another signature",
		Why:     "The generated Equal calls the Equal method of a field's type when it is Equal(T) bool. A method of that name with another signature (Equal(*T) bool, Equal(any) bool) would not be the same comparison, so the generator does not guess.",
		Fix:     "Change the method to Equal(T) bool, or write Equal for the outer type by hand.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-cl-signature", Source: "checker", Phase: 10,
		Pattern: "#Derive(Clone): %s has the type %s, whose Clone method does not have the signature Clone() %s: change it or write the copy by hand",
		Title:   "A Clone method with another signature",
		Why:     "The generated Clone calls the Clone method of a field's type when it is Clone() T. A method of that name with another signature would not be a copy of the same type, so the generator does not guess.",
		Fix:     "Change the method to Clone() T, or write Clone for the outer type by hand.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-cl-foreign", Source: "checker", Phase: 10,
		Pattern: "#Derive(Clone): %s has the type %s, which shows slices, maps or pointers and has no Clone method: write one for it, or keep the value out of the type",
		Title:   "Clone of a type of another package that shares memory",
		Why:     "A type of another package is copied as a value, and so would share the slices, maps and pointers that it shows in its exported fields. The generator cannot copy them for you: it does not know the type's rules.",
		Fix:     "Give the type a Clone method (in its package), or hold it behind a pointer, which is shared, or write Clone by hand.",
		Spec:    "language.md#derive",
	},
	{
		ID: "loader/derive-ellipsis", Source: "loader", Phase: 10,
		Pattern: "#Derive: cannot use ... in the arguments",
		Title:   "... in #Derive",
		Why:     "The arguments of #Derive are the names of generators.",
		Fix:     "Write the names one by one.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-kind", Source: "checker", Phase: 10,
		Pattern: "#Derive(%s): %s: the generators are for structs and for named array, slice, map and integer types",
		Title:   "#Derive on a type that cannot have the methods",
		Why:     "The methods are added to the named type, so it must be able to have them: a struct, or a named array, slice, map or integer type. An interface, a pointer, a func or a channel type cannot be the receiver of String, Equal or Clone.",
		Fix:     "Derive on the struct that holds the value, or write the code by hand.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-field", Source: "checker", Phase: 10,
		Pattern: "#Derive(%s): the type %s has a field %s already: a field and a method cannot have the same name",
		Title:   "A field with the name of a derived method",
		Why:     "A struct cannot have a field and a method with one name, and a generator adds String, Equal, Clone, MarshalJSON, UnmarshalJSON and GoCPPJSONPlainPtr.",
		Fix:     "Rename the field, or leave the generator out.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-import", Source: "checker", Phase: 10,
		Pattern: "#Derive(%s): the type %s is from the package %s, which the file does not import: add the import to the file",
		Title:   "A package that the file does not import",
		Why:     "The generated code writes the types of the fields as the file of the type writes them, so the file has to import their packages.",
		Fix:     "Add the import to the file that declares the type.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-json-import", Source: "checker", Phase: 9,
		Pattern: "#Derive(JSON): the methods call encoding/json, which the file does not import: add import \"encoding/json\" to the file of %s",
		Title:   "#Derive(JSON) in a file that does not import encoding/json",
		Why:     "MarshalJSON and UnmarshalJSON call json.Marshal and json.Unmarshal on a copy of the struct type that has no methods (type gp_plain T), so the output is exactly encoding/json's. The calls are written with the name that the file gives the package, so the file of the type must import it.",
		Fix:     "Add import \"encoding/json\" to the file that declares the type.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-cpp", Source: "checker", Phase: 10,
		Pattern: "#Derive(%s): %s has a C or C++ type: the generators cannot look inside it, write the method by hand",
		Title:   "A field of a C or C++ type",
		Why:     "The generators know what a field is made of from its Go type. A C or C++ type has no Go structure (and may not be comparable or copyable in C++), so they cannot compare, copy or print it for you.",
		Fix:     "Write the method by hand, or keep the C/C++ value behind a pointer in a type that you handle yourself.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-noconsts", Source: "checker", Phase: 10,
		Pattern: "#Derive(String): no constants of type %s are declared in this package",
		Title:   "String with no constants",
		Why:     "The generator writes a switch over the constants of the type that the package declares. With none, there is nothing to name.",
		Fix:     "Declare the constants of the type in the package, or remove the generator.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-eq-foreign", Source: "checker", Phase: 10,
		Pattern: "#Derive(Equal): %s has the type %s, which has no Equal method and cannot be compared with ==: write the method by hand",
		Title:   "Equal on a type of another package that cannot be compared",
		Why:     "The generator cannot look inside a type of another package (its fields may be private). It calls the type's Equal method, or compares with == when the type allows it.",
		Fix:     "Write Equal by hand, or leave the field out of the struct.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-eq-func", Source: "checker", Phase: 10,
		Pattern: "#Derive(Equal): %s has a func type, which cannot be compared",
		Title:   "Equal on a func",
		Why:     "Functions cannot be compared in Go (except with nil).",
		Fix:     "Leave the field out of the type, or write Equal by hand.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-eq-needs", Source: "checker", Phase: 10,
		Pattern: "#Derive(Equal): %s has the type %s, which has no Equal method and holds pointers, slices or maps: add #Derive(Equal) to %s or write the method by hand",
		Title:   "Equal needs the Equal of the field's type",
		Why:     "The generator calls the Equal method of a field's type. A struct of your package with pointers, slices or maps and no Equal method cannot be compared with == to the depth that Equal means.",
		Fix:     "Add #Derive(Equal) to that type, or write the method.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-cl-needs", Source: "checker", Phase: 10,
		Pattern: "#Derive(Clone): %s has the type %s, which refers to memory and has no Clone method: add #Derive(Clone) to %s or write the method by hand",
		Title:   "Clone needs the Clone of the field's type",
		Why:     "Copying a struct of your package that holds pointers, slices or maps with = would share them. The generator calls the Clone method of the field's type to copy them.",
		Fix:     "Add #Derive(Clone) to that type, or write the method.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-cannot", Source: "checker", Phase: 10,
		Pattern: "#Derive(%s): %s has the type %s, which the generator cannot %s",
		Title:   "A type the generator cannot handle",
		Why:     "The generator has no rule for this kind of type.",
		Fix:     "Write the method by hand.",
		Spec:    "language.md#derive",
	},
	{
		ID: "checker/derive-cycle", Source: "checker", Phase: 10,
		Pattern: "#Derive(%s): the type %s refers to itself and has no %s method: add #Derive(%s) to it",
		Title:   "A type that refers to itself",
		Why:     "A named slice, map or array type of your package without the method is expanded where it is used. Types that refer to each other would be expanded forever.",
		Fix:     "Add #Derive(...) to one of them, so that the others call its method.",
		Spec:    "language.md#derive",
	},
	{
		ID: "loader/macro-redeclared", Source: "loader",
		Pattern: "macro %s redeclared in this package",
		Title:   "Two macros with one name",
		Why:     "A macro is a function of the package that the compiler writes for it, so a package can declare a macro once, and not next to a func, type or variable of the same name.",
		Fix:     "Rename one of them.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-builtin-name", Source: "loader",
		Pattern: "%s is a built-in macro: choose another name for the macro",
		Title:   "A macro named like a built-in macro",
		Why:     "#file, #line, #func, #text, #embed, #body and #Derive are the compiler's own macros. A macro of yours with one of those names could not be told from them at a use.",
		Fix:     "Choose another name, such as Where or Source.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-predeclared", Source: "loader",
		Pattern: "%s is a predeclared name: a macro with that name would be hidden by the built-in. Choose another name",
		Title:   "A macro named like a predeclared identifier",
		Why:     "The compiler writes a use #min(a, b) as the call min(a, b) of the function that the macro became. Go finds the built-in min first when the macro's package is the current one, and a macro named error or string would hide a type.",
		Fix:     "Choose another name.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-receiver", Source: "loader",
		Text:  "a macro has no receiver",
		Title: "A macro with a receiver",
		Why:   "A macro is a package-level declaration. It has parameters, but it is not a method.",
		Fix:   "Write a func with a receiver, or a macro with the receiver as its first parameter.",
		Spec:  "language.md#macros",
	},
	{
		ID: "loader/macro-variadic", Source: "loader",
		Text:  "can only use ... with the final parameter",
		Title: "... on a parameter that is not the last",
		Why:   "As in a func, only the last parameter of a macro can be variadic.",
		Fix:   "Move the ... parameter to the end of the parameter list.",
		Spec:  "language.md#macros",
	},
	{
		ID: "loader/macro-reserved", Source: "loader",
		Pattern: "the name %s is reserved: names that start with gp_ are used by the compiler in macros",
		Title:   "A name that starts with gp_ in a macro",
		Why:     "The compiler adds the parameters gp_file, gp_line, gp_func, gp_text_x and gp_body to a macro, and the type parameters gp_L_x and gp_B. A name of yours that starts with gp_ could hide one of them.",
		Fix:     "Rename the parameter or variable.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-lazy-type", Source: "loader",
		Pattern: "lazy parameter %s: %s",
		Title:   "A lazy parameter whose type can not be written at every use",
		Why:     "A lazy argument is passed as a func literal, so the use must write the type that it returns: func() T { return arg }. That works for a predeclared type, for a type of the macro's package that the use can name, and for a type of a package that both files import. It does not work for a type parameter of the macro (lazy T), for a struct or interface type with members, or for an array whose length is not a number.",
		Fix:     "Declare a named type for it, or use a plain parameter if the argument can be evaluated at the use.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-use-shape", Source: "loader",
		Pattern: "a macro is used as #Name(...) or #pkg.Name(...), not %s",
		Title:   "A macro name that is not Name or pkg.Name",
		Why:     "A use names a macro of this package (#Assert) or of an imported package (#util.Assert), and calls it. A longer selector, a value in parentheses or a macro that is not called is not a use.",
		Fix:     "Write #Name(...) or #pkg.Name(...).",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-undefined", Source: "loader",
		Pattern: "undefined macro #%s",
		Title:   "A macro that is not declared",
		Why:     "The name after # is not a macro of this package, of a package imported with a dot, or (for #pkg.Name) of the imported package. Macros are declared with macro Name(...) { ... }; a func is not a macro.",
		Fix:     "Declare the macro, import its package, or check the spelling. For a func, call it without #.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-not-a-package", Source: "loader",
		Pattern: "#%s: %s is not an imported package",
		Title:   "#pkg.Name where pkg is not an imported package",
		Why:     "A macro of another package is used as #pkg.Name(...), and pkg is the name that the file gives to the import.",
		Fix:     "Import the package of the macro in this file.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-unexported", Source: "loader",
		Pattern: "macro %s is not exported by package %s: it can only be used there",
		Title:   "A macro of another package that is not exported",
		Why:     "As for a func, a macro whose name starts with a lower-case letter can only be used in its own package.",
		Fix:     "Export the macro (start its name with a capital letter) or use it inside its package.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-not-generic", Source: "loader",
		Pattern: "macro %s is not generic: it takes no type arguments",
		Title:   "Type arguments for a macro that has none",
		Why:     "#Dbg[int](x) gives the type parameter of a generic macro. This macro declares none.",
		Fix:     "Remove the type arguments.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-lazy-unexported", Source: "loader",
		Pattern: "#%s: the type %s of the lazy parameter %s is not exported by package %s, so the use can't write it",
		Title:   "A lazy parameter of an unexported type of another package",
		Why:     "The use writes the type of a lazy argument (func() T { return arg }), so it must be able to name T. A type of another package that is not exported can not be named.",
		Fix:     "Export the type, or use a plain parameter.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-lazy-import", Source: "loader",
		Pattern: "#%s: the lazy parameter %s has the type %s.%s, and this file does not import %q: add the import",
		Title:   "A lazy parameter of a type from a package the file does not import",
		Why:     "The use writes the type of a lazy argument, so the file of the use must import the package that the type is from.",
		Fix:     "Import the package in this file.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-args", Source: "loader",
		Pattern: "#%s: %s arguments (have %s, want %s)",
		Title:   "A macro use with the wrong number of arguments",
		Why:     "A macro takes the arguments that its parameters list (the place of the use and the text of the arguments are added by the compiler). A call that passes several results of one call, #M(f()), counts as one argument.",
		Fix:     "Pass one argument for each parameter.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-ellipsis", Source: "loader",
		Pattern: "#%s: cannot use ... in the arguments: the macro is not variadic",
		Title:   "... in the arguments of a macro that is not variadic",
		Why:     "f(xs...) passes a slice to the variadic parameter of f. This macro has none.",
		Fix:     "Pass the elements one by one, or declare the last parameter of the macro with ...",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-ellipsis-count", Source: "loader",
		Pattern: "#%s: the arguments with ... must be %s, one for each parameter (have %s)",
		Title:   "... with other arguments for the variadic parameter",
		Why:     "xs... passes the slice itself as the variadic parameter, so no argument can come before it for that parameter.",
		Fix:     "Pass the elements one by one, or only the slice.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-func-statement", Source: "loader",
		Pattern: "#%s returns a func(): as a statement it would drop the func that ends its work. Use it on top of a func, or call the result: f := #%s(...)",
		Title:   "A macro that returns a func() used as a statement",
		Why:     "A macro like Locked returns the func() that undoes its work. As a statement inside code the result would be dropped, and the lock would never be released. On top of a func, the compiler defers the result for you.",
		Fix:     "Write the macro on top of the func, or keep the result: unlock := #Locked(&mu); defer unlock().",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-defer-go", Source: "loader",
		Pattern: "#%s can't be the call of a defer or go statement: use it before, and defer or start what it returns",
		Title:   "defer #M() or go #M()",
		Why:     "A macro is not a call that can be deferred or started: the compiler rewrites it into a call with more arguments, and what the macro does at its start would happen at the wrong time.",
		Fix:     "Write the macro before, and defer or start the func it returns, or put the macro on top of a func.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-wrap-in-code", Source: "loader",
		Pattern: "#%s uses #body(): it is a wrap macro, which can only be used on top of a func",
		Title:   "A wrap macro used inside code",
		Why:     "A macro that calls #body() runs the code of the function it is written above. Inside code there is no function to take the code from.",
		Fix:     "Write the macro on top of a func.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-lazy-must", Source: "loader",
		Pattern: "%s: ! can't be used in a lazy macro argument (it would turn into a panic there): evaluate it before the macro",
		Title:   "! in a lazy macro argument",
		Why:     "A lazy argument is a func literal that the macro may run, or not. A ! in it returns the error from that literal, not from your function, so it would silently become a panic or a lost error.",
		Fix:     "Evaluate the call before the macro: v := f!(x), then #M(v).",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-derive-in-code", Source: "loader",
		Pattern: "#Derive is for types: put it on top of a type declaration",
		Title:   "#Derive used on a func or inside code",
		Why:     "#Derive(String), #Derive(Equal), #Derive(Clone) and #Derive(JSON) generate methods of the type that is declared right below them.",
		Fix:     "Write #Derive(...) on the line above a type declaration.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-builtin-on-func", Source: "loader",
		Pattern: "%s is a built-in macro for use inside code, not on top of a func",
		Title:   "A built-in macro on top of a func",
		Why:     "#file, #line, #func, #text, #embed and #body give a value or the code of a function; they are written inside code (or inside a macro). Only your own macros and #Derive go on top of declarations.",
		Fix:     "Write it inside the function, or write a macro that uses it.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-no-body", Source: "loader",
		Pattern: "#%s: the func %s has no body: a macro on top of it has nothing to work on",
		Title:   "A macro on top of a func without a body",
		Why:     "A macro on top of a func puts a call at the start of the function's body, or takes the body as its code. A func declared without a body (implemented in C, C++ or assembly) has none.",
		Fix:     "Remove the macro, or wrap the call in a func with a body.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-wrap-must", Source: "loader",
		Pattern: "#%s is a wrap macro: it can't be used on top of the must function %s (yet): write the function without !",
		Title:   "A wrap macro on a must function",
		Why:     "A must function (func f!() T) gets its error result and its returns rewritten, and a wrap macro moves the body into a func literal and rewrites the returns again. The two rewrites do not combine yet. Start-style macros work on a must function.",
		Fix:     "Write the function with an explicit error result, or use a start-style macro.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-wrap-recover", Source: "loader",
		Pattern: "recover() called directly by %s, which has the wrap macro #%s: its code runs in a func literal, so recover() would not see the panic. Call it in a deferred func literal",
		Title:   "recover() called by a function that has a wrap macro",
		Why:     "recover only stops a panic when the deferred function itself calls it. A wrap macro runs the function's code inside a func literal, so a function that is deferred (defer rec()) and has a wrap macro on top would call recover one level too deep, and it would return nil without a word.",
		Fix:     "Call recover in a deferred func literal inside the function (defer func() { recover() }()), or write the function without the wrap macro.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-on-type", Source: "loader",
		Pattern: "a macro on top of a type: only #Derive can be used there (user macros on types are not supported): %s",
		Title:   "A user macro on top of a type",
		Why:     "In this version, a macro above a type declaration must be one of the built-in generators, #Derive(...). Macros that generate code for a type are on the backlog.",
		Fix:     "Write the code by hand, or use #Derive.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/macro-leftover", Source: "loader",
		Pattern: "%s: a macro can only be used as a call, #Name(...)",
		Title:   "A macro name that is not called",
		Why:     "A use is a call: #Name(args). The name alone is not a value.",
		Fix:     "Add the parentheses and the arguments.",
		Spec:    "language.md#macros",
	},
	{
		ID: "checker/macro-plain-use", Source: "checker",
		Pattern: "%s is a macro: write #%s(...)",
		Title:   "A macro used as a func",
		Why:     "A macro is a function with parameters that the compiler adds (the place of the use, the text of the arguments), so a plain call Assert(x) or a value f := util.Assert would pass the wrong arguments. A macro is only used with # : #Assert(x), #util.Assert(x), or on top of a func.",
		Fix:     "Write # before the name.",
		Spec:    "language.md#macros",
	},
	{
		ID: "checker/macro-hidden", Source: "checker",
		Pattern: "the macro %s is hidden here by %s with the same name: rename it",
		Title:   "A variable, import or declaration with the name of a macro, in a place that uses the macro",
		Why:     "The compiler turns #show(1) into a call of the function show. A variable named show in scope at the use is what Go finds first, so the call would be a call of the variable. Macros are not hygienic about their own name.",
		Fix:     "Rename the variable or the parameter, or the macro.",
		Spec:    "language.md#macros",
	},
	{
		ID: "checker/macro-redeclared", Source: "checker",
		Pattern: "%s redeclared in this block (a macro and a func, or two macros, can't have the same name)",
		Title:   "A macro and another declaration with the same name",
		Why:     "The compiler writes a macro as a function of the same name in the package, so no func, type, variable or constant can have the name of a macro of its package.",
		Fix:     "Rename one of them.",
		Spec:    "language.md#macros",
	},
	{
		ID: "loader/embed-fs", Source: "loader", Phase: 9,
		Text:  "embed.FS is not available yet (Phase 09: it needs io/fs); //go:embed works on string and []byte variables",
		Title: "embed.FS",
		Why:   "//go:embed and #embed work on string and []byte variables. An embed.FS value is a file system over the embedded files: it needs package io/fs, which is being ported in Phase 09.",
		Fix:   "Embed one file into a string or []byte variable, or embed several variables.",
	},
	{
		ID: "lower/fmt-writer", Source: "lowering", Phase: 9,
		Text:  "fmt.Fprint to a writer other than os.Stdout, os.Stderr or a fmt.State is not available yet (package io: Phase 09)",
		Title: "fmt.Fprint to another writer",
		Why:   "fmt.Fprint, Fprintf and Fprintln are lowered for os.Stdout, os.Stderr and a fmt.State. An io.Writer in general needs package io, which comes in Phase 09.",
		Fix:   "Write to os.Stdout or os.Stderr, or build the string with fmt.Sprint and write it yourself.",
	},
	{
		ID: "lower/io-value", Source: "lowering", Phase: 9,
		Pattern: "%s is not available yet (it needs package io: Phase 09)",
		Title:   "An io value",
		Why:     "os.Stdout and os.Stderr can be the writer argument of fmt.Fprint, where the lowering knows them. Used as a value of type io.Writer they need package io (Phase 09).",
		Fix:     "Use them only as the writer of fmt.Fprint until Phase 09.",
	},
	{
		ID: "lower/osfile-value", Source: "lowering", Phase: 9,
		Pattern: "%s used as a value, outside the writer argument of fmt.Fprint, is not available yet (it needs the type os.File: Phase 09)",
		Title:   "os.Stdout, os.Stderr, os.Stdin as values",
		Why:     "They are *os.File values, and the type os.File comes with package os in Phase 09.",
		Fix:     "Use them as the writer of fmt.Fprint, or print with fmt.Println.",
	},
	{
		ID: "lower/os-member", Source: "lowering", Phase: 9,
		Pattern: "%s is not available yet (Phase 09)",
		Title:   "A member of a package that comes in Phase 09",
		Why:     "The lowering has a few members of os and io wired in (the writers of fmt.Fprint). Everything else of those packages comes with the standard library in Phase 09.",
		Fix:     "Wait for Phase 09, or use fmt.Print and its relatives.",
	},
	{
		ID: "lower/std-type", Source: "lowering", Phase: 9,
		Pattern: "the type %s is not available yet (package %s: Phase 09)",
		Title:   "A type of a package that comes in Phase 09",
		Why:     "The program names a type of a standard package that has a stub but not its implementation yet.",
		Fix:     "Wait for Phase 09.",
	},
	{
		ID: "loader/package", Source: "loader", Phase: 9,
		Pattern: "package %s is not available yet (Phase 09)",
		Title:   "A standard package without a stub",
		Why:     "The standard library of gocpp is the packages written in Go++ (stdlib/) and the runtime. The packages that are not written yet come in Phase 09, in tiers: the first ones are the most used (strings, strconv, os, io, sort, errors, time...).",
		Fix:     "Use a package that exists (gocpp list std), call a C/C++ library instead (import \"C\"), or wait for Phase 09.",
		Spec:    "feature-matrix.md",
	},
	{
		ID: "driver/cross-target", Source: "driver", Phase: 6,
		Pattern: "building for %s/%s on %s/%s is not supported yet (Phase 06)",
		Title:   "Building for another OS or architecture",
		Why:     "gocpp emit and gocpp check write the C++ for any target, but gocpp build and gocpp run call the C++ compiler of this machine, and cross builds are a later step of Phase 06.",
		Fix:     "Build on the target machine, or use gocpp emit to get the C++ and build it yourself.",
	},
	{
		ID: "driver/buildmode-windows", Source: "driver", Phase: 14,
		Pattern: "-buildmode=%s for windows is not supported yet (Phase 14: .dll and import libraries)",
		Title:   "A shared library or archive for Windows",
		Why:     "The C++ for a -buildmode other than a program is built for Linux and macOS today. A Windows .dll with an import library comes with the Windows work of Phase 14.",
		Fix:     "Build the library on Linux or macOS, or build a program for Windows (-os windows).",
	},
	{
		ID: "driver/test-json", Source: "driver", Phase: 9,
		Text:  "-json is not supported yet",
		Title: "gocpp test -json",
		Why:   "gocpp test prints the text report of go test; the machine-readable JSON stream is not written yet.",
		Fix:   "Read the text report (-v shows every test).",
	},
	{
		ID: "driver/test-flag", Source: "driver", Phase: 9,
		Pattern: "-%s is not supported yet",
		Title:   "A go test flag that gocpp test does not have",
		Why:     "The race detector, coverage, fuzzing and the profiles of go test are not built into gocpp test yet. Use -asan or -tsan of gocpp build for memory and thread errors.",
		Fix:     "Run the tests without that flag.",
	},
	{
		ID: "driver/test-files", Source: "driver", Phase: 9,
		Text:  "gocpp test: a list of .gp files is not supported yet; name the package",
		Title: "gocpp test with a list of files",
		Why:   "gocpp test builds a package, not a list of files.",
		Fix:   "Name the package (gocpp test ./path/to/pkg) instead of its files.",
	},
	{
		ID: "driver/command", Source: "driver", Phase: 0,
		Pattern: "gocpp %s: not available yet (Phase %s)",
		Title:   "A command that is not built yet",
		Why:     "gocpp help lists every command of the plan; each one is built by the phase named in the message.",
		Fix:     "gocpp help shows which commands exist now.",
	},
	{
		Fallback: true, ID: "lower/later-step", Source: "lowering", Phase: 4,
		Pattern: "%s (a later step of Phase 04)",
		Title:   "A use of C/C++ values that a later step of Phase 04 builds",
		Why:     "Interop with C and C++ is built in steps (aliases, named types over C types, conversions). A construct that the checker can't tell apart from the supported ones is stopped in the lowering with this message.",
		Fix:     "Write the C/C++ type itself where the message says an alias or a named type is not lowered; declare variables of C/C++ values with their type (var x T = ...).",
		Topic:   "interop",
	},
	{
		Fallback: true, ID: "lower/typed-interop", Source: "lowering", Phase: 12,
		Pattern: "%s (typed interop: Phase 12)",
		Title:   "Typed interop",
		Why:     "Phase 12 maps C++ types to Go++ types, including methods and interfaces. Until then a C/C++ value is opaque: it can be stored, passed to C++ and have its members used, but it has no Go method set.",
		Fix:     "Use plain Go types in the signature, and convert at the border to C/C++.",
		Topic:   "interop",
	},
	{
		Fallback: true, ID: "lower/phase-12", Source: "lowering", Phase: 12,
		Pattern: "%s (Phase 12)",
		Title:   "A construct on C/C++ values that Phase 12 builds",
		Why:     "The typed interop of Phase 12 builds what the message names: a construct on C/C++ values that needs the C++ types themselves (a generic function of another package whose body uses C/C++ names, a channel type as a C++ template argument, promoted members of several embedded C++ classes).",
		Fix:     "Keep the construct in the package that declares the C/C++ names, or write it with Go types at the border.",
		Topic:   "interop",
	},
	{
		Fallback: true, ID: "lower/phase-08", Source: "lowering", Phase: 8,
		Pattern: "%s (Phase 08)",
		Title:   "A construct that Phase 08 builds",
		Why:     "Channels, go and select come in Phase 08; the lowering stops at an operation on a channel that it has no code for yet.",
		Fix:     "Wait for the step of Phase 08 that builds it.",
		Topic:   "channels",
	},
	{
		ID: "lower/only-main", Source: "lowering", Phase: 6,
		Pattern: "package %s: only package main can be built until Phase 06",
		Title:   "Only package main",
		Why:     "Phase 06 builds programs of several packages. Until then the lowering writes only the package main.",
		Fix:     "Put the program in package main.",
	},
	{
		Fallback: true, ID: "lower/generic", Source: "lowering", Phase: 0,
		Pattern: "not supported yet: %s",
		Title:   "A construct that the lowering has no C++ for",
		Why:     "The checker accepted the program, but the lowering has no C++ for this construct yet. It is reported as an internal error because the checker's gates are meant to stop a program before it gets here.",
		Fix:     "Rewrite that line with a simpler construct (split an assignment, name an intermediate value), and report the program as a bug: gocpp emit shows where the lowering stopped.",
	},
	{
		ID: "loader/reserved-c", Source: "loader", Phase: 4,
		Pattern: "%s - C is reserved for C's built-in types",
		Title:   "C is reserved",
		Why:     "In a file that imports \"C\", the name C is the built-in package of C types and functions (C.int, C.puts). It can't also name a package, a type, a variable or anything else.",
		Fix:     "Rename your declaration or the import (import cc \"example.com/c\"). The \"C\" import itself is the one for C.",
		Spec:    "interop.md",
		Topic:   "interop",
	},
	{
		ID: "loader/reserved-c-1", Source: "loader", Phase: 4,
		Pattern: "cannot import %s as C - C is reserved for C's built-in types",
		Title:   "A package imported as C",
		Why:     "The name C belongs to the built-in C package.",
		Fix:     "Import the package under another name.",
		Spec:    "interop.md",
	},
	{
		ID: "cxx/value-eq", Source: "C++ compiler (gocpp/interop.hpp)",
		Pattern: "== compares the C/C++ parts of this value with C++'s ==, which a C/C++ type in it doesn't have%s",
		Title:   "== on a value with a C/C++ part that has no ==",
		Why:     "Go's == on a struct or an array with C/C++ parts compares those parts with C++'s operator== (gp::cgo::value_eq), as a switch on such a value does for its cases. A C++ type in the value has none, so the compiler stops at the comparison.",
		Fix:     "Compare the other parts yourself (a.n == b.n), or declare an operator== for the C++ type.",
		Topic:   "interop",
		Spec:    "interop.md",
	},
	{
		ID: "cxx/map-key", Source: "C++ compiler (gocpp/interop.hpp)",
		Pattern: "this C/C++ type can't be the key of a gocpp map%s",
		Title:   "A C/C++ type without std::hash or == as a map key",
		Why:     "A gocpp map hashes a key with C/C++ parts with std::hash and compares it with == (gp::cgo::map_key): a C++ type, an array of such elements, a struct with such fields. A C++ type in the key has no std::hash or no ==, so the compiler stops at the map's line.",
		Fix:     "Use a Go key (a string, a struct of Go fields), a C++ map such as std.map[K, V], or declare std::hash and operator== for the C++ type.",
		Topic:   "interop",
		Spec:    "interop.md",
	},
	{
		ID: "cxx/fmt-key-order", Source: "C++ compiler (gocpp/interop.hpp)",
		Pattern: "fmt prints the keys of a map in order, and a C++ class key needs an operator< for that%s",
		Title:   "Printing a map whose key is a C++ class without operator<",
		Why:     "fmt prints a map with its keys sorted, as Go's fmtsort does: a C++ string by its compare, a C++ class by its operator<. The key's class has none, so the compiler stops at the print.",
		Fix:     "Declare an operator< for the class, or print the entries yourself.",
		Topic:   "interop",
		Spec:    "interop.md",
	},
	{
		ID: "lower/cpp-iface-method", Source: "lowering", Phase: 12,
		Pattern: "the interface method %s has a C/C++ type in its signature (typed interop: Phase 12)",
		Title:   "An interface method with a C/C++ type in its signature",
		Why:     "The method table of an interface is made of Go types. A method whose parameter or result is a C or C++ type needs typed interop (Phase 12).",
		Fix:     "Use a Go type in the interface's methods and convert at the call.",
		Topic:   "interop",
	},
	{
		ID: "lower/cpp-alias", Source: "lowering", Phase: 4,
		Pattern: "aliases of C/C++ types (%s): write the C/C++ type (a later step of Phase 04)",
		Title:   "An alias of a C/C++ type",
		Why:     "type T = C.int is built in a later step of Phase 04.",
		Fix:     "Write the C/C++ type where you would write the alias.",
		Topic:   "interop",
	},
	{
		ID: "lower/cpp-conversion", Source: "lowering", Phase: 4,
		Text:  "conversions to this C/C++ type (a later step of Phase 04)",
		Title: "A conversion to a C/C++ type that is not lowered",
		Why:   "C.int(x) and the conversions to the other C types are built; a conversion to a C++ class or to a pointer-to-member is a later step of Phase 04.",
		Fix:   "Construct the value with its C++ constructor (a C++ call), or use a helper function in C++.",
		Topic: "interop",
	},
	{
		ID: "lower/cpp-type-use", Source: "lowering", Phase: 4,
		Text:  "this use of a C/C++ type (a later step of Phase 04)",
		Title: "A use of a C/C++ type that is not lowered",
		Why:   "C and C++ types are lowered where the earlier steps of Phase 04 use them (variables, fields, parameters, results, calls). The use in the message is a later step.",
		Fix:   "Give the value a name and a declared type (var x T = ...), or move the use into a C++ helper.",
		Topic: "interop",
	},
	{
		ID: "loader/reserved-c-bare", Source: "loader", Phase: 4,
		Text:  "C is reserved for C's built-in types",
		Title: "C is reserved (a name of a declaration)",
		Why:   "A declaration or an import was given the name C. In gocpp, as in cgo, C is the package of C's built-in types and functions (C.int, C.puts), so no other thing can have that name.",
		Fix:   "Choose another name.",
		Spec:  "interop.md",
		Topic: "interop",
	},
	{
		ID: "loader/import-as-c-name", Source: "loader", Phase: 4,
		Pattern: "cannot import package as C - C is reserved for C's built-in types (give it another name, like import c %s)",
		Title:   "A package whose own name is C",
		Why:     "The package's name is C, which is reserved for C's built-in types, so importing it without a name of your own would clash with import \"C\".",
		Fix:     "Give it another name in the import, as the message shows.",
		Spec:    "interop.md",
		Topic:   "interop",
	},
	{
		ID: "lower/later-step-comma", Source: "lowering", Phase: 4,
		Pattern: "%s, a later step of Phase 04)",
		Title:   "A named type over a C/C++ type",
		Why:     "type T C.int-style declarations (a named type whose underlying type is a C/C++ type) are built in a later step of Phase 04: the emitter has no C++ for them yet.",
		Fix:     "Use the C/C++ type itself, or wrap it in a struct (type T struct{ v C.int }).",
		Topic:   "interop",
	},
	{
		ID: "loader/pattern-dots", Source: "loader", Phase: 6,
		Pattern: "pattern %s: patterns with ... are not supported yet",
		Title:   "Patterns with ...",
		Why:     "gocpp build, check and emit take files, folders and package paths; the ./... pattern that expands to every package below a folder comes with the package build in Phase 06.",
		Fix:     "Name the package folders, or the .gp files, one by one.",
	},
	{
		ID: "loader/dot-import-c", Source: "loader", Phase: 4,
		Pattern: "cannot dot-import %s: it exports C, which is reserved for C's built-in types",
		Title:   "A dot import of a package that exports C",
		Why:     "A dot import adds the package's names to the file, including a name C, which the C built-in package owns.",
		Fix:     "Import the package with a name (import p \"path\") and write p.Name.",
		Spec:    "interop.md",
	},
	{
		ID: "loader/init-func", Source: "loader", Phase: 0,
		Pattern: "cannot import %s as init - init must be a func",
		Title:   "A package imported as init",
		Why:     "As in Go, init is the name of the init functions, so a package can't have it.",
		Fix:     "Import the package under another name.",
	},
	{
		ID: "check/header", Source: "checker", Phase: 4,
		Pattern: "cannot find header %s",
		Title:   "A header that was not found",
		Why:     "An import of a C/C++ header (import \"C/stdio.h\", or a module header) is looked up by the C++ compiler's include path: the system, the module's include folders and pkg-config. The compiler found no such file.",
		Fix:     "Check the spelling, add the include folder in gocpp.mod (cxx recipe) or with #cgo CFLAGS: -I..., or use pkg-config for a library. gocpp env shows the compiler and its include paths.",
		Spec:    "interop.md",
		Topic:   "interop",
	},
	{
		ID: "interop/undefined-name", Source: "lowering", Phase: 0,
		Pattern: "undefined: %s (the imported headers declare no %s)",
		Title:   "A C/C++ name that the headers do not declare",
		Why:     "A name of a header package (cstdio.printff) or of a namespace (std.vectr) is looked up in the headers that the program imports. The C++ compiler's probe found nothing of that name, so it is a typo, a header that is not imported, or a name that is only declared for another platform. The lowering reports this error when it translates the name; the typed checker reports it for the same place with the same words.",
		Fix:     "Check the spelling, import the header that declares the name (import \"<cstdio>\"), or look at the namespace it is in (std.chrono.steady_clock).",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/no-member", Source: "interop checker", Phase: 0,
		Pattern: "%s undefined (type %s has no field or method %s)",
		Title:   "A member that the C/C++ type does not have",
		Why:     "The typed checker asked the C++ compiler about the type of the value, and the class (or struct, or union) has no data member and no member function of that name, also not in its base classes. Private members are not found from Go either.",
		Fix:     "Check the spelling (the message suggests a close name when there is one) and the type: the member may belong to a base class that is not public, or to another overload set.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/no-overload", Source: "interop checker", Phase: 0,
		Pattern: "no overload of %s matches the arguments (%s)",
		Title:   "No overload of a C/C++ function accepts the arguments",
		Why:     "C++ chooses among the functions of one name by the argument types, and none of them can take these ones: the compiler's candidates are listed under the message, each with the reason it was rejected (the number of arguments, or the argument that cannot be converted). An untyped constant is passed as C++'s int, double or char array, as a C call would pass it.",
		Fix:     "Change the arguments to one of the candidates: convert a value explicitly (C.int(x)), pass the address of a variable (&x), or use C.cstr(s) for a Go string that a C function takes as char*.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/no-overload-call", Source: "interop checker", Phase: 0,
		Pattern: "no overload of %s matches the call",
		Title:   "No overload of a C/C++ function accepts the call",
		Why:     "As \"no overload matches the arguments\", for a call whose argument types gocpp cannot write in Go's words (a result of another C++ call, a type with no Go name).",
		Fix:     "See the candidates listed under the message and change the arguments to fit one of them.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/const-array-len", Source: "interop constants", Phase: 0,
		Pattern: "array length %s must be constant: %s",
		Title:   "A C/C++ name that is no constant used as an array length",
		Why:     "Typed interop puts the compile-time value of a C/C++ name (a macro, a constexpr or const variable, an enumerator) into the program where Go needs a constant, so [cstdio.BUFSIZ]byte works. This name was evaluated by the C++ compiler and has no such value: its macro has a side effect or calls a function, it is a function-like macro without its arguments, a variable that is not constant, a value that Go constants cannot have (NaN, infinity), a type, or an object of a class. The reason is at the end of the message.",
		Fix:     "Use a name that is a constant in C++ (a macro with a plain value, a constexpr variable), or write the number.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/const-decl", Source: "interop constants", Phase: 0,
		Pattern: "%s is not constant: %s",
		Title:   "A C/C++ name that is no constant in a constant declaration",
		Why:     "As \"array length must be constant\", for the value of a const declaration: const n = lib.NAME. Go constants are numbers, characters, booleans and strings; the C++ compiler could not give this name a value of those kinds at compile time.",
		Fix:     "Declare a variable instead (n := lib.NAME), or use a name that is a constant in C++.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/const-warning", Source: "interop constants", Phase: 0,
		Pattern: "cannot evaluate the C/C++ constants of this package: %s (they stay opaque)",
		Title:   "The constants of a package could not be evaluated",
		Why:     "Typed interop asks clang for the values of the C/C++ names that sit where Go needs a constant. The question failed (the compiler crashed or timed out), so those names are left to the C++ compiler as in opaque mode, and a use in an array length or a const declaration is then an error of the phase gate.",
		Fix:     "Run the build again; check that clang++ works on the package's headers (gocpp env shows the compiler).",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/ambiguous", Source: "interop checker", Phase: 0,
		Pattern: "call to %s is ambiguous: several overloads match equally well",
		Title:   "An ambiguous call of an overloaded C/C++ function",
		Why:     "Two or more overloads need a conversion of the same rank for the arguments (a double to int or to long; an int to long or to unsigned long), so C++ refuses to choose. The candidates are listed under the message with their places in the header.",
		Fix:     "Say which one you mean by giving the argument the exact type of one overload: C.long(x), C.int(x), or a typed variable.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/cannot-use", Source: "interop checker", Phase: 0,
		Pattern: "cannot use %s (%s) as %s value in %s",
		Title:   "A value that has no C++ conversion to the type that is wanted",
		Why:     "The value and the parameter, variable or result have types between which C++ has no implicit conversion: an integer for an enum, a number for a pointer, a Go string for a char*, a class for a pointer to it, a temporary for a reference parameter that needs a variable, a void result for anything. gocpp applies C++'s rules to C/C++ values (not Go's), so a conversion that C++ allows silently, such as int to long, is accepted.",
		Fix:     "Convert the value explicitly, pass the address (&x) where a pointer is wanted, name the enumerator, or use C.cstr(s) for a string. The line under the message says which mistake it looks like.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/too-many-arguments", Source: "interop checker", Phase: 0,
		Pattern: "too many arguments in call to %s (have %s, want %s)",
		Title:   "A call of a C/C++ function with too many arguments",
		Why:     "The function takes fewer arguments than the call passes, and it has no overload or default arguments that take more.",
		Fix:     "Remove the extra arguments, or call the overload that takes them.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/not-enough-arguments", Source: "interop checker", Phase: 0,
		Pattern: "not enough arguments in call to %s (have %s, want %s)",
		Title:   "A call of a C/C++ function with too few arguments",
		Why:     "The function needs more arguments than the call passes, and its remaining parameters have no default values.",
		Fix:     "Pass the missing arguments.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/call-non-function", Source: "interop checker", Phase: 0,
		Pattern: "invalid operation: cannot call non-function %s (type %s)",
		Title:   "A call of a C/C++ name that is not a function",
		Why:     "The name is a variable, a constant or a macro with a value (errno, stdout, a global), not a function or an object with operator().",
		Fix:     "Use the name without the parentheses, or call the function that you meant.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/binary-operator", Source: "interop checker", Phase: 0,
		Pattern: "invalid operation: %s (operator %s is not defined on %s and %s)",
		Title:   "An operator that C++ does not define for these types",
		Why:     "Operators on C/C++ values follow C++'s rules: the usual arithmetic conversions between numbers, pointer arithmetic, and the operator functions that the types declare. This pair of types has none.",
		Fix:     "Convert an operand (C.int(x)), call the member function that does the job (v.size()), or compare the contents of the two values.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/unary-operator", Source: "interop checker", Phase: 0,
		Pattern: "invalid operation: operator %s not defined on %s (type %s)",
		Title:   "A unary operator that C++ does not define for this type",
		Why:     "The operand's C++ type has no such operator (a class without operator!, or ~ on a double).",
		Fix:     "Convert the operand or call a member function.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/const-method", Source: "interop checker", Phase: 0,
		Pattern: "cannot call method %s on %s (type %s): the method is not marked %s",
		Title:   "A method called on a const value",
		Why:     "The value is const (a const reference result, a const object), and the C++ member function is not declared const, so it may change the object and C++ refuses to call it. gocpp does not hide const-ness.",
		Fix:     "Call it on a non-const value (a copy in a variable), or use a const member function.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/non-static-method", Source: "interop checker", Phase: 0,
		Pattern: "cannot call %s without a value: it is a method, not a static function",
		Title:   "A method called through its type",
		Why:     "Type.method(...) is the form for static member functions. This member function needs an object to work on.",
		Fix:     "Call it on a value: v.method(...).",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/address-of-value", Source: "interop checker", Phase: 0,
		Pattern: "invalid operation: cannot take address of %s (value of type %s)",
		Title:   "The address of a temporary C++ value",
		Why:     "A call result or an operator result that is not a reference is a temporary: it has no address, in Go and in C++.",
		Fix:     "Store the value in a variable first and take the address of the variable.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/not-assignable", Source: "interop checker", Phase: 0,
		Pattern: "cannot assign to %s (neither addressable nor a map index expression)",
		Title:   "An assignment to a C++ value that is not a variable",
		Why:     "The left side is a call result or an operator result of a scalar type: a temporary, which C++ does not let you assign to. A function that returns a reference (v.front() = 5) is fine.",
		Fix:     "Assign to a variable, a field, an element, or the result of a function that returns a non-const reference.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/assign-const", Source: "interop checker", Phase: 0,
		Pattern: "cannot assign to %s: it is const",
		Title:   "An assignment to a const C++ value",
		Why:     "The left side is a reference to a const object, so C++ refuses the assignment.",
		Fix:     "Assign through a non-const reference, or change the value where it lives.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/incomplete-type", Source: "interop checker", Phase: 0,
		Pattern: "invalid use of %s: the type is only declared in the headers, not defined",
		Title:   "A C++ type that is only declared",
		Why:     "The headers declare the type (struct sqlite3;) without its members, so a value of it cannot exist, and its size and members are unknown. Only pointers to it can be used.",
		Fix:     "Use a pointer (*sqlite3) and let the library create the object.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/deleted", Source: "interop checker", Phase: 0,
		Pattern: "%s uses a function that C++ has deleted (%s)",
		Title:   "A use of a C++ function that is deleted",
		Why:     "The class declares the function as = delete, or the compiler made it deleted (a class with a unique_ptr member has no copy constructor). Passing or assigning such a value by value copies it.",
		Fix:     "Pass a pointer or a reference, or move the value if the class can move.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/convert", Source: "interop checker", Phase: 0,
		Pattern: "cannot convert %s (%s) to type %s",
		Title:   "A conversion that C++ does not allow",
		Why:     "T(x) of a C/C++ value is a C++ conversion. The two types have no conversion operator or constructor between them, so the cast is refused.",
		Fix:     "Convert in two steps through a type that has the conversion, or call the member function that gives the value.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/no-value", Source: "interop checker", Phase: 0,
		Pattern: "%s (no value) used as value",
		Title:   "A void C++ call used as a value",
		Why:     "The function returns void, so there is nothing to assign, pass or return.",
		Fix:     "Call it as a statement, and use the object it works on afterwards.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/private-member", Source: "interop checker", Phase: 0,
		Pattern: "%s undefined (%s is a %s member of type %s)",
		Title:   "A private or protected member of a C++ class",
		Why:     "The member exists, but the class declares it private or protected (the default of a class), so code outside the class and its friends cannot use it, in C++ or from Go.",
		Fix:     "Use the public interface of the class (a getter, a public field), or declare the member public in your own header.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/method-value", Source: "interop checker", Phase: 0,
		Pattern: "cannot use %s as a value: a C++ member function must be called",
		Title:   "A C++ member function used as a value",
		Why:     "A member function has no function value in C++: c.inc without the parentheses is not a callable object (a pointer to member needs the class and an object to be called on). Go's method values (m := c.inc) do not exist for C++ methods.",
		Fix:     "Call it (c.inc()), or wrap the call in a func literal: m := func() { c.inc() }.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
	{
		ID: "interop/not-valid-cpp", Source: "interop checker", Phase: 0,
		Pattern: "%s is not valid C++: %s",
		Title:   "An expression that the C++ compiler rejects",
		Why:     "The typed checker asked clang whether the C++ that gocpp writes for this expression compiles, and it printed an error that has no special wording here. The message after the colon is the compiler's own (types in Go's spelling where gocpp could translate them).",
		Fix:     "Read the compiler's message: it names the declaration that disagrees with the use. gocpp explain interop shows how Go values become C++ arguments.",
		Topic:   "interop", Spec: "typed-interop.md",
	},
}
