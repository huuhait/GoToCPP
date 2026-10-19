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
		msgTable = append(append([]Message(nil), curated...), gateMessages()...)
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
		ID: "lower/generic-local-types", Source: "lowering", Phase: 7,
		Text:  "type declarations inside generic functions are not supported yet (Phase 07)",
		Title: "Type declarations inside generic functions",
		Why:   "A generic function is lowered once for each set of type arguments. A type that it declares would have to be a different C++ type in each instance, and the lowering does not name those yet.",
		Fix:   "Move the type declaration out of the function (to package level), as a generic type if it needs the type parameters, once those are built.",
		Topic: "generics",
	},
	{
		ID: "lower/std-generics", Source: "lowering", Phase: 7,
		Pattern: "%s.%s is not available yet (Phase 07)",
		Title:   "A generic function of the standard library",
		Why:     "Functions such as slices.Contains, maps.Keys and cmp.Compare are generic. The standard library of gocpp gets them in Phase 07, with the generic functions of the program.",
		Fix:     "Write the loop yourself, or wait for Phase 07.",
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
		ID: "lower/cpp-in-iface", Source: "lowering", Phase: 12,
		Pattern: "a C/C++ value in an interface with methods (%s) is not available yet: only interfaces without methods hold C/C++ values until Phase 12 (typed interop)",
		Title:   "A C/C++ value in an interface with methods",
		Why:     "A C/C++ value can be held by an interface without methods (any): it is copied into a box with a descriptor made from its C++ type. An interface with methods needs a method table, and C/C++ types have no Go methods until typed interop (Phase 12).",
		Fix:     "Wrap the C/C++ value in a Go struct that has the methods, and put the struct in the interface.",
		Topic:   "interop",
	},
	{
		ID: "loader/macro-decl", Source: "loader", Phase: 10,
		Text:  "macro declarations are not supported yet (Phase 10, step 10.9)",
		Title: "A macro declaration",
		Why:   "The built-in macros #file, #line, #func, #text and #embed work today (step 10.8). Declaring your own macros comes with user macros in step 10.9.",
		Fix:   "Write the code the macro would generate, or use a built-in macro.",
	},
	{
		ID: "loader/macro-on-decl", Source: "loader", Phase: 10,
		Pattern: "macros on top of a declaration are not supported yet (Phase 10, step 10.9): %s",
		Title:   "A macro on top of a declaration",
		Why:     "A macro written above a func or type (#Trace, #Locked, #Derive) is a user macro or a derive macro: both come in Phase 10, steps 10.9 and 10.10.",
		Fix:     "Write the generated code by hand for now.",
	},
	{
		ID: "loader/macro-use", Source: "loader", Phase: 10,
		Pattern: "macro %s is not supported yet (Phase 10, step 10.9)",
		Title:   "A user macro",
		Why:     "Only the built-in macros #file, #line, #func, #text and #embed exist today (step 10.8). Any other macro name needs user macros (step 10.9).",
		Fix:     "Use a built-in macro, or write the code out.",
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
		Why:     "== and switch on values with C/C++ parts need the typed interop of Phase 12 (a comparison that C++ may not have, and a type that may not be hashable).",
		Fix:     "Compare the fields you mean, or convert the value to a Go type first.",
		Topic:   "interop",
	},
	{
		Fallback: true, ID: "lower/phase-07", Source: "lowering", Phase: 7,
		Pattern: "%s (Phase 07)",
		Title:   "A construct that Phase 07 builds",
		Why:     "The checker lets a few generic constructs through (a type parameter that is only passed on), and the lowering stops where the instance would need something that comes in Phase 07: constraint interfaces, range over functions, instantiations of types.",
		Fix:     "Use a plain type, or wait for Phase 07.",
		Topic:   "generics",
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
		ID: "lower/range-func", Source: "lowering", Phase: 7,
		Text:  "range over functions (Phase 07)",
		Title: "Range over a function",
		Why:   "for x := range seq, where seq is an iterator func, is built in a later step of Phase 07 (range-over-func rewrites the loop body into a function).",
		Fix:   "Call the iterator with a func literal that holds the loop body: seq(func(x T) bool { ...; return true }).",
		Topic: "range",
	},
	{
		ID: "lower/range-typeparam", Source: "lowering", Phase: 7,
		Text:  "range over type parameters (Phase 07)",
		Title: "Range over a value of a type parameter",
		Why:   "A range over a value whose type is a type parameter is lowered in the instance, where the type argument is known. Generic bodies are not lowered as such yet.",
		Fix:   "Range over a slice or map type that you name, and instantiate the function with it.",
		Topic: "generics",
	},
	{
		ID: "lower/constraint-iface", Source: "lowering", Phase: 7,
		Text:  "constraint interfaces (Phase 07)",
		Title: "Constraint interfaces",
		Why:   "An interface with type sets (~int | ~string) is a constraint, not a type of values. Constraints are checked by the type checker; the lowering does not write them as C++ types.",
		Fix:   "Use the constraint only as the bound of a type parameter, not as the type of a variable.",
		Topic: "generics",
	},
	{
		ID: "lower/instantiation", Source: "lowering", Phase: 7,
		Text:  "instantiations (Phase 07)",
		Title: "An explicit instantiation of a generic type",
		Why:   "F[int] and T[int] as expressions: functions are instantiated by the lowering where they are called, and generic types come in a later step of Phase 07.",
		Fix:   "Call the generic function and let the arguments give the type arguments.",
		Topic: "generics",
	},
	{
		ID: "lower/cpp-eq-switch", Source: "lowering", Phase: 12,
		Pattern: "%s on values with C/C++ parts (Phase 12)",
		Title:   "Comparing or switching on values with C/C++ parts",
		Why:     "== on a struct or array that has a C or C++ field, and a switch on such a value, need to know whether the C++ type has an operator==. Typed interop (Phase 12) maps that.",
		Fix:     "Compare the fields you mean (a.x == b.x), or convert the value to a Go type first.",
		Topic:   "interop",
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
}
