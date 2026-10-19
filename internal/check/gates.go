package check

import (
	"fmt"
	"strings"

	"github.com/huuhait/gocpp/internal/version"
)

// The phase gates (Phase 01 README §2.9): after type checking, a pass over
// the user packages (never the standard packages, whose stubs use methods,
// interfaces, generics and channels) rejects every feature that a later
// phase builds, with a message that names the phase:
//
//	./main.gp:5:12: interfaces are not supported yet (Phase 05)
//
// so that nobody gets broken C++. The tables in this file are the data:
// the language features (features), the standard packages (stdPackages),
// the members of a package that come in another phase than the package
// (stdMembers), the built-in functions (builtins) and the verbs of format
// strings (formatVerbs). featureMatrix makes docs/spec/feature-matrix.md
// from the same tables.
//
// The checker reports the first use of each feature in each file (by
// position), not every use: one error per occurrence would bury the other
// errors under the same message, and the first use says enough to find the
// feature. It reports each import of a package, which is one use per file.
// A gate error that takes the place of a go/types error (a promoted member
// of an embedded C++ type, errors.Trace) is reported at every use, so that
// each of those go/types errors gets its better message.

// LastPhase is the last phase of the plan. Config{Phase: LastPhase} turns
// every gate off.
const LastPhase = 15

// Never is the phase of a feature that gocpp code never gets.
const Never = 99

// floorPhase is the phase whose subset gocpp check always accepts: Phase
// 02's walking skeleton (functions, basic types, loops, switch, fmt.Print
// and friends). The Phase 01 exit criteria want the Phase 02 programs to
// pass gocpp check, so the gates start above it, whatever Config.Phase
// says.
const floorPhase = 2

// phase returns the phase that the gates allow, from Config.Phase.
func (c Config) phase() int {
	p := c.Phase
	if p == 0 {
		p = version.Phase
	}
	return max(p, floorPhase)
}

// A featureID names a row of the features table.
type featureID int

const (
	// Phase 02: documented exemptions, never reported.
	featPanicBasic featureID = iota

	// Phase 03.
	featComposite
	featPointers
	featMethods
	featFuncValues
	featClosures
	featDefer
	featTypeDecls
	featTypeAliases
	featStringSlice
	featRangeString
	featStringConv
	featComplex
	featVariadic
	featPackageVars
	featInitFuncs
	featFormatFlags

	// Phase 04.
	featHeaderImports
	featNamespaces
	featCNames
	featDirectives
	featNativeFuncs
	featModuleHeaders

	// Phase 05.
	featInterfaces
	featAny
	featErrors
	featTypeAssert
	featTypeSwitch
	featEmbedding
	featPanicValue

	// Phase 06.
	featMultiPackage
	featExport

	// Phase 07.
	featGenerics // generic functions (step 7.1, 7.2)
	featGenericTypes
	featRangeFunc

	// Phase 08.
	featGo
	featChannels
	featSelect

	// Phase 09.
	featTestFiles
	featFormatMore
	featFormatDynamic

	// Phase 10.
	featMustCalls
	featMustFuncs
	featMacros
	featGoEmbed

	// Phase 12.
	featPromotedCpp
	featPromotedCppShadow
	featPromotedCppExpr
	featCConstants

	// Phase 14.
	featWideStrings
	featNoWin32Defaults

	// Never.
	featLinkname

	numFeatures
)

// A feature is one row of the features table: a language feature that a
// phase builds.
type feature struct {
	id featureID

	// Name is the feature in the message and in the feature matrix:
	// "composite types". Plural says whether it takes "are" or "is".
	Name   string
	Plural bool

	// Phase is the phase that builds the feature: the gate allows it
	// when Phase <= Config.Phase. Never: gocpp code never gets it.
	Phase int

	// Since, if it is not zero, is the phase from which the gate allows
	// the feature although Phase is later: the part of it that programs
	// need is built and tested, and Phase finishes the rest (programs of
	// more than one package: Phase 05 has them, Phase 06 adds the build
	// modes and the cache). The message still names Phase, for a program
	// that is checked before Since. A feature whose built part would
	// break programs (half a feature) gets no Since.
	Since int

	// Detect says how the checker finds a use, for the people who work
	// on it (gatewalk.go and gateparse.go do it).
	Detect string

	// Notes are the details for the feature matrix: what the feature
	// covers, exemptions, links. Plain English, short.
	Notes string
}

// allowed reports whether the gates of the phase let the feature through.
func (f *feature) allowed(phase int) bool {
	return f.Phase <= phase || (f.Since != 0 && f.Since <= phase)
}

// SinceOf returns the Since of the feature whose gate error is msg (the text after "file:line:col: "),
// and whether there is one: the phase from which the gates stop reporting that error although the
// message names a later phase. cmd/gocpp's tests use it to know which lines of examples.txt `gocpp
// check` still reports.
func SinceOf(msg string) (since int, ok bool) {
	for i := range features {
		if f := &features[i]; f.Since != 0 && strings.HasSuffix(msg, f.Message()) {
			return f.Since, true
		}
	}
	return 0, false
}

// Message returns the error message for a use of the feature.
func (f *feature) Message() string {
	return gateMessage(f.Name, f.Plural, f.Phase)
}

// gateMessage returns the message for a use of something that a later
// phase brings, or that gocpp never has (Never).
func gateMessage(name string, plural bool, phase int) string {
	verb := "is"
	if plural {
		verb = "are"
	}
	if phase == Never {
		return fmt.Sprintf("%s %s not supported in gocpp", name, verb)
	}
	return fmt.Sprintf("%s %s not supported yet (Phase %02d)", name, verb, phase)
}

// features is the table of the language features, in the order of
// featureID.
var features = [numFeatures]feature{
	{id: featPanicBasic, Name: "panic with a value of a basic type", Phase: 2,
		Detect: "never reported: an exemption from the any gate",
		Notes:  "`panic(\"msg\")`, `panic(42)`. Any other value comes in Phase 05."},

	{id: featComposite, Name: "composite types", Plural: true, Phase: 3,
		Detect: "an array, slice, map or struct type in a type expression, a declared variable or a value; a composite literal",
		Notes:  "Arrays, slices, maps and structs, their literals, and `range` over them."},
	{id: featPointers, Name: "pointers", Plural: true, Phase: 3,
		Detect: "a pointer type; &x; *p",
		Notes:  "`*T`, `&x`, `*p`, `new(T)`. Pointers are reference counted (divergence D1)."},
	{id: featMethods, Name: "methods", Plural: true, Phase: 3,
		Detect: "a func declaration with a receiver",
		Notes:  "Value and pointer receivers, method values and method expressions."},
	{id: featFuncValues, Name: "function values", Plural: true, Phase: 3,
		Detect: "a func type in a declaration; a value of func type that is not called",
		Notes:  "Variables, parameters and fields of func type, and a function used without calling it."},
	{id: featClosures, Name: "closures", Plural: true, Phase: 3,
		Detect: "a func literal",
		Notes:  "Function literals."},
	{id: featDefer, Name: "defer", Phase: 3,
		Detect: "a defer statement",
		Notes:  "Also a deferred closure that changes the named results."},
	{id: featTypeDecls, Name: "type declarations", Plural: true, Phase: 3,
		Detect: "a type spec without =",
		Notes:  "Named types: `type Celsius float64`, `type Point struct{...}`."},
	{id: featTypeAliases, Name: "type aliases", Plural: true, Phase: 3,
		Detect: "a type spec with =",
		Notes:  "`type A = B`. A generic alias comes in Phase 07."},
	{id: featStringSlice, Name: "string slicing", Phase: 3,
		Detect: "a slice expression on a string",
		Notes:  "`s[a:b]`. Phase 02 has `+`, comparisons, `len(s)` and the byte `s[i]`."},
	{id: featRangeString, Name: "range over strings", Phase: 3,
		Detect: "a range statement over a string",
		Notes:  "It decodes UTF-8, like Go."},
	{id: featStringConv, Name: "conversions between strings and bytes, runes or integers", Plural: true, Phase: 3,
		Detect: "a conversion to a string from a non-string, or to a slice from a string",
		Notes:  "`[]byte(s)`, `[]rune(s)`, `string(b)`, `string(r)`."},
	{id: featComplex, Name: "complex numbers", Plural: true, Phase: 3,
		Detect: "a complex64 or complex128 type or value",
		Notes:  "With the built-ins `complex`, `real` and `imag`."},
	{id: featVariadic, Name: "variadic functions", Plural: true, Phase: 3,
		Detect: "... before the type of the last parameter of a func",
		Notes:  "Declaring them. Calling `fmt.Println` and friends works from Phase 02."},
	{id: featPackageVars, Name: "package-level variables", Plural: true, Phase: 3,
		Detect: "a var declaration at package level",
		Notes:  "With Go's initialization order. Package-level constants work from Phase 02."},
	{id: featInitFuncs, Name: "init functions", Plural: true, Phase: 3,
		Detect: "a func init at package level"},
	{id: featFormatFlags, Name: "width, precision and flags in format strings", Plural: true, Phase: 3,
		Detect: "a constant format string of fmt.Printf, Sprintf or Errorf (formatVerbs has the verbs)",
		Notes:  "`%5d`, `%-8s`, `%.2f`, `%+v`, `%05d`. Phase 02 has `%v %d %s %t %f` and `%%` without them."},

	{id: featHeaderImports, Name: "header imports", Plural: true, Phase: 4,
		Detect: "load.FileInfo.Includes",
		Notes:  "`import \"<raylib.h>\"` and `import \"./mathx.h\"`, and the header packages they give (`raylib.InitWindow`)."},
	{id: featNamespaces, Name: "C++ namespaces", Plural: true, Phase: 4,
		Detect: "a namespace use (load.Rewrite.Uses) or a namespace rename",
		Notes:  "`std.vector[int]`, `fs \"::std::filesystem\"`."},
	{id: featCNames, Name: "C types and helpers", Plural: true, Phase: 4,
		Detect: "a selector on C that the user wrote (not one made by the header rewrite)",
		Notes:  "`C.int`, `C.size_t`, `C.CString`, `C.free`, `C.cstr`, `C.GoString`, `C.GoStringN`, `C.GoBytes`."},
	{id: featDirectives, Name: "//gocpp: directives", Plural: true, Phase: 4,
		Detect: "load.File.Directives",
		Notes:  "`//gocpp:pkg-config`, `link`, `cflags`, `ldflags`, `extern-c`, `no-extern-c`."},
	{id: featNativeFuncs, Name: "functions without a body", Plural: true, Phase: 4,
		Detect: "a func declaration without a body",
		Notes:  "Native functions: the body is in a C or C++ file of the package."},
	{id: featModuleHeaders, Name: "module headers", Plural: true, Phase: 4,
		Detect: "a local header import whose first path element has a dot (load.IsModuleHeaderPath)",
		Notes:  "`import \"github.com/ocornut/imgui/imgui.h\"`, and `import \"<own module>/native/x.h\"` for a header in the project."},

	{id: featInterfaces, Name: "interfaces", Plural: true, Phase: 5,
		Detect: "an interface type with methods or embedded interfaces",
		Notes:  "Interface types, conversions to them, interface embedding."},
	{id: featAny, Name: "any values", Plural: true, Phase: 5,
		Detect: "the empty interface (any, interface{}) in a type or a value",
		Notes:  "Exemption: the `...any` parameters of `fmt.Print`, `fmt.Println` and `fmt.Printf` take values of basic types from Phase 02, and those of `fmt.Sprint`, `fmt.Sprintf` and `fmt.Sprintln` from Phase 03. A `nil` argument needs Phase 05."},
	{id: featErrors, Name: "error values", Plural: true, Phase: 5,
		Detect: "the error type in a type or a value",
		Notes:  "The `error` type. Exemption: a call of `fmt.Print`, `fmt.Println` or `fmt.Printf` as a statement, which drops its results, works from Phase 02; using the results needs Phase 05."},
	{id: featTypeAssert, Name: "type assertions", Plural: true, Phase: 5,
		Detect: "x.(T) outside a type switch",
		Notes:  "`x.(T)` and `v, ok := x.(T)`."},
	{id: featTypeSwitch, Name: "type switches", Plural: true, Phase: 5,
		Detect: "a type switch statement"},
	{id: featEmbedding, Name: "struct embedding", Phase: 5,
		Detect: "a struct field without a name",
		Notes:  "With promoted fields and methods."},
	{id: featPanicValue, Name: "panic with a value of a non-basic type", Phase: 5,
		Detect: "a call of panic whose argument is not of a basic type",
		Notes:  "`panic(err)`, `panic(myStruct)`."},

	{id: featMultiPackage, Name: "programs of more than one package", Plural: true, Phase: 6, Since: 5,
		Detect: "an import of a package that is not a standard package",
		Notes:  "Importing packages of your module (`gocpp.mod`), of `replace`d, vendored and downloaded modules. The standard packages are fine. Works from Phase 05 (step 6.2: one translation unit per file, Go's initialization order across packages); Phase 06 finishes the build system (cache, build modes)."},
	{id: featExport, Name: "//export functions", Plural: true, Phase: 6,
		Detect: "an //export line in the doc comment of a func",
		Notes:  "Functions that C code can call (`-buildmode=c-archive`, `c-shared`)."},

	{id: featGenerics, Name: "generic functions", Plural: true, Phase: 7,
		Detect: "type parameters on a func (FuncType), info.Instances of functions, type sets and comparable",
		Notes:  "Generic functions, constraints with type sets, instantiating them with explicit or inferred type arguments. Exemption: `weak.Make` from Phase 03. The standard packages `cmp`, `slices`, `maps` and `iter` are ported in step 7.10: until then their functions cannot be instantiated."},
	{id: featGenericTypes, Name: "generic types", Plural: true, Phase: 7,
		Detect: "type parameters on a type declaration (TypeSpec), info.Instances of types, a Named type with type arguments",
		Notes:  "Generic types and their methods, generic aliases (step 7.3 and 7.7). Exemption: `weak.Pointer[T]` from Phase 03."},
	{id: featRangeFunc, Name: "range over functions", Phase: 7,
		Detect: "a range statement over a value of func type",
		Notes:  "`for x := range f` over any func(yield func(...) bool) (step 7.8: Go's rewrite and its misuse checks). The types `iter.Seq` and `iter.Seq2` come with the port of `iter` in step 7.10; until then declare the func types."},

	{id: featGo, Name: "go statements", Plural: true, Phase: 8,
		Detect: "a go statement",
		Notes:  "One OS thread per goroutine (divergence D7)."},
	{id: featChannels, Name: "channels", Plural: true, Phase: 8,
		Detect: "a chan type or value, a send statement, a receive",
		Notes:  "Also `close`, and `range` over a channel."},
	{id: featSelect, Name: "select statements", Plural: true, Phase: 8,
		Detect: "a select statement"},

	{id: featTestFiles, Name: "test files", Plural: true, Phase: 9,
		Detect: "a _test.gp file (load.File.Test)",
		Notes:  "`_test.gp` files and `gocpp test`."},
	{id: featFormatMore, Name: "the # flag, * and argument indexes in format strings", Plural: true, Phase: 5,
		Detect: "a constant format string of fmt.Printf, Sprintf or Errorf",
		Notes:  "`%#v`, `%*d`, `%[1]d`."},
	{id: featFormatDynamic, Name: "format strings that are not constants", Plural: true, Phase: 5,
		Detect: "a format argument of fmt.Printf, Sprintf or Errorf without a constant value",
		Notes:  "From Phase 05 `fmt` checks nothing at compile time: a bad verb prints Go's error form at run time."},

	{id: featMustCalls, Name: "must calls", Plural: true, Phase: 10,
		Detect: "a syntax error in a file with name!( or name![ (a name right before !)",
		Notes:  "`F!(...)` (divergence D12)."},
	{id: featMustFuncs, Name: "must functions", Plural: true, Phase: 10,
		Detect: "a syntax error in a file with func name!",
		Notes:  "`func f!()` (divergence D12)."},
	{id: featMacros, Name: "macros", Plural: true, Phase: 10,
		Detect: "a syntax error in a file with #Name or a package-level macro Name",
		Notes:  "`macro` declarations with `lazy` parameters, and `#Name(...)` uses, `#body()` and the other built-in macros (divergence D13)."},
	{id: featGoEmbed, Name: "//go:embed", Phase: 10,
		Detect: "a //go:embed line",
		Notes:  "Comes with `#embed(...)`."},

	{id: featPromotedCpp, Name: "promoted fields and methods of several embedded C/C++ types", Plural: true, Phase: 12,
		Detect: "a selector that go/types can't find, on a struct that embeds more than one C/C++ type (one is fine: check/promoted.go)",
		Notes:  "A struct that embeds one C/C++ type has its members promoted from Phase 05 (`mv.push_back(1)` for `std.vector[int]`). With several, check can't tell which one has the member: name the field, `mv.vector.push_back(1)`."},
	{id: featPromotedCppShadow, Name: "selectors that a Go member and a member of an embedded C/C++ type could both match (name the field)", Plural: true, Phase: 12,
		Detect: "a selector that go/types resolved to a Go member of a struct that also embeds a C/C++ type at the same or a shallower depth (check/promoted.go: cMayShadow)",
		Notes:  "In Go the shallowest member wins and the same depth is ambiguous, but check can't know the members of a C/C++ type, so it can't tell whether `s.x` is the Go field or the C++ member. Name the field: `s.epoint.x` or `s.Deep.Inner.x`."},
	{id: featPromotedCppExpr, Name: "method expressions of promoted members of an embedded C/C++ type", Plural: true, Phase: 12,
		Detect: "T.name where name can only be a member of the one C/C++ type that T embeds",
		Notes:  "`MV.size`, `(*MV).push_back` for a struct that embeds `std.vector[int]`. Use a func literal instead: `func(m MV) { m.size() }`."},
	{id: featCConstants, Name: "C macros in constant expressions", Plural: true, Phase: 12,
		Detect: "a C/C++ name in an array length or a const declaration",
		Notes:  "`[cstdio.BUFSIZ]byte`, `const n = cstdio.BUFSIZ`."},

	{id: featWideStrings, Name: "C.wstr and C.GoStringW", Plural: true, Phase: 14,
		Detect: "the selectors C.wstr and C.GoStringW",
		Notes:  "UTF-16 strings for Windows APIs."},
	{id: featNoWin32Defaults, Name: "//gocpp:no-win32-defaults", Phase: 14,
		Detect: "the directive"},

	{id: featLinkname, Name: "//go:linkname", Phase: Never,
		Detect: "a //go:linkname line",
		Notes:  "Only the standard library uses it, and `tools/portstd` turns those uses into native functions (Phase 09)."},
}

// A stdPackage is a row of the table of the standard packages.
type stdPackage struct {
	// Path is the import path. A path that ends in "/..." stands for
	// every package below it ("go/...": go/ast, go/types, …).
	Path string

	// Phase is the phase from which the package can be imported, or
	// Never.
	Phase int

	// Rest is the phase of the members of the package that stdMembers
	// doesn't list, if it is later than Phase (fmt: the members after
	// Print, Println and Printf come with Go's fmt in Phase 09). Zero
	// means Phase.
	Rest int

	Notes string // for the feature matrix
}

// message returns the error for an import of the package path (the row's
// Path, or a path below it).
func (p *stdPackage) message(path string) string {
	return pkgMessage(path, p.Phase)
}

// pkgMessage returns the error for an import of the standard package path
// that comes in the phase. Its words are the loader's for a standard
// package without a stub ("package net/http is not available yet (Phase
// 09)").
func pkgMessage(path string, phase int) string {
	if phase == Never {
		return fmt.Sprintf("package %s is not supported in gocpp", path)
	}
	return fmt.Sprintf("package %s is not available yet (Phase %02d)", path, phase)
}

// otherStdPhase is the phase of the standard packages that stdPackages
// doesn't list: most come in Phase 09 (its README §2.6 has the tiers), and
// the loader says Phase 09 for each one without a stub. A few of them come
// later (tier 3), without a phase yet.
const otherStdPhase = 9

// stdPackages are the standard packages with the phase that implements
// them. The packages with a stub (stdlib/) are all here, with the phase
// that their stub names (TestStdPackagesMatchStubs). The loader rejects a
// standard package without a stub with "package X is not available yet
// (Phase 09)"; for the packages here, Check gives their own phase instead
// (weak: Phase 03, plugin: never).
var stdPackages = []stdPackage{
	{Path: "fmt", Phase: 2, Rest: 9, Notes: "`Print`, `Println` and `Printf` with basic values from Phase 02; from Phase 05 every value and verb, the `Fprint` functions for `os.Stdout`, `os.Stderr` and a `fmt.State`, `Append`, `Errorf`, `Stringer`, `Formatter` and the other interfaces; see the table of members below. The rest (`Sscan` and friends) comes with Go's `fmt` in Phase 09."},
	{Path: "unsafe", Phase: 3, Rest: 4, Notes: "`Sizeof`, `Alignof` and `Offsetof` from Phase 03, the rest (`unsafe.Pointer`, `Slice`, `String`, …) from Phase 04. `unsafe.Pointer` keeps gocpp memory alive (divergence D16)."},
	{Path: "weak", Phase: 3, Notes: "`weak.Pointer[T]` is a compiler intrinsic, exempt from the generics gate."},
	{Path: "errors", Phase: 5},
	{Path: "runtime", Phase: 8, Notes: "`runtime.Error`, `PanicNilError`, `TypeAssertionError`, `Gosched`, `NumGoroutine`, `GOMAXPROCS`, `NumCPU`, `Goexit`, `LockOSThread`, `Caller`, `Callers`, `CallersFrames`, `FuncForPC`, `Stack`, `GC`, `ReadMemStats`, `Version`, `GOOS`, `GOARCH`, `KeepAlive` and `SetFinalizer` from Phase 08 (step 8.10). Each goroutine has a thread of its own, so `LockOSThread` does nothing and `Gosched` only yields the CPU. Memory is reference counted: `GC` does nothing and a finalizer never runs. The profiles, traces and `AddCleanup` are not built; stdlib/runtime/PORT.md lists them."},
	{Path: "runtime/debug", Phase: 8, Notes: "`Stack`, `PrintStack`, `SetGCPercent`, `SetMemoryLimit`, `FreeOSMemory`, `SetMaxStack`, `SetMaxThreads`, `SetPanicOnFault` and `SetTraceback` from Phase 08 (step 8.10). `ReadBuildInfo`, `ReadGCStats` and `SetCrashOutput` come with Phase 09."},
	{Path: "runtime/cgo", Phase: 9, Notes: "`cgo.Handle` with `NewHandle`, `Value` and `Delete` (step 9.2): a native table, so a handle passes through C and C++ as an integer."},
	{Path: "cmp", Phase: 7},
	{Path: "iter", Phase: 7, Notes: "`iter.Pull` and `iter.Pull2` from Phase 08 (they run on a helper thread)."},
	{Path: "maps", Phase: 7},
	{Path: "slices", Phase: 7},
	{Path: "sync", Phase: 8},
	{Path: "sync/atomic", Phase: 8},
	{Path: "time", Phase: 8, Notes: "The whole package, since step 8.8 (Go's own `format.go`, `time.go` and `zoneinfo.go`, with natives for the clocks, `Sleep`, the timers and the time zone files). Timers follow Go 1.23: timer channels look unbuffered, `Stop` and `Reset` leave no stale value, and a `Timer` nobody holds is freed. `time.LoadLocation` reads the system's zoneinfo (the `ZONEINFO` variable, `/usr/share/zoneinfo`, `/etc/zoneinfo`); there is no embedded copy (`time/tzdata` is not available). On Windows (Phase 14) the zone data comes later."},
	{Path: "bufio", Phase: 9},
	{Path: "bytes", Phase: 9},
	{Path: "context", Phase: 9},
	{Path: "encoding", Phase: 9},
	{Path: "encoding/json", Phase: 9},
	{Path: "flag", Phase: 9},
	{Path: "io", Phase: 9},
	{Path: "io/fs", Phase: 9},
	{Path: "os", Phase: 5, Rest: 9, Notes: "`os.Stdout` and `os.Stderr` as the writers of `fmt.Fprint` and friends from Phase 05; the rest of the package in Phase 09."},
	{Path: "reflect", Phase: 9, Notes: "A subset, without `MakeFunc`, calling methods by name, `StructOf` and `FuncOf` (divergence D6)."},
	{Path: "regexp", Phase: 9},
	{Path: "strconv", Phase: 9},
	{Path: "strings", Phase: 9},
	{Path: "testing", Phase: 9},
	{Path: "unicode", Phase: 9},
	{Path: "unicode/utf8", Phase: 9},
	{Path: "unicode/utf16", Phase: 9},
	{Path: "math", Phase: 9},
	{Path: "math/bits", Phase: 9},
	{Path: "embed", Phase: 10, Notes: "With `//go:embed` and `#embed(...)`."},
	{Path: "go/...", Phase: Never, Notes: "`go/ast`, `go/types` and the other packages about Go source are not planned."},
	{Path: "net/http", Phase: Never, Notes: "Not planned: use a C or C++ HTTP library through a header import."},
	{Path: "plugin", Phase: Never, Notes: "gocpp builds one native binary (divergence D6)."},
}

// A stdMember is a member of a standard package that comes in another phase
// than its package, or a built-in function.
type stdMember struct {
	Pkg string // the import path, or "" for a built-in function

	// Name is the name of the member, "Type.Method" for a method:
	// "Sprintf", "Time.Format".
	Name  string
	Phase int
}

// message returns the error for a use of the member.
func (m *stdMember) message() string {
	if m.Pkg == "" {
		return gateMessage("built-in function "+m.Name, false, m.Phase)
	}
	return fmt.Sprintf("%s.%s is not available yet (Phase %02d)", m.Pkg, m.Name, m.Phase)
}

// stdMembers are the members of standard packages with their own phase.
// The other members of a package come in its phase (stdPackage.Rest, or
// else stdPackage.Phase).
var stdMembers = []stdMember{
	{Pkg: "fmt", Name: "Print", Phase: 2},
	{Pkg: "fmt", Name: "Printf", Phase: 2},
	{Pkg: "fmt", Name: "Println", Phase: 2},
	{Pkg: "fmt", Name: "Sprint", Phase: 3},
	{Pkg: "fmt", Name: "Sprintf", Phase: 3},
	{Pkg: "fmt", Name: "Sprintln", Phase: 3},
	{Pkg: "fmt", Name: "Errorf", Phase: 5},
	{Pkg: "fmt", Name: "Stringer", Phase: 5},
	{Pkg: "fmt", Name: "GoStringer", Phase: 5},
	{Pkg: "fmt", Name: "Formatter", Phase: 5},
	{Pkg: "fmt", Name: "State", Phase: 5},
	{Pkg: "fmt", Name: "FormatString", Phase: 5},
	{Pkg: "fmt", Name: "Fprint", Phase: 5},
	{Pkg: "fmt", Name: "Fprintf", Phase: 5},
	{Pkg: "fmt", Name: "Fprintln", Phase: 5},
	{Pkg: "fmt", Name: "Append", Phase: 5},
	{Pkg: "fmt", Name: "Appendf", Phase: 5},
	{Pkg: "fmt", Name: "Appendln", Phase: 5},
	{Pkg: "os", Name: "Stdout", Phase: 5},
	{Pkg: "os", Name: "Stderr", Phase: 5},
	{Pkg: "unsafe", Name: "Alignof", Phase: 3},
	{Pkg: "unsafe", Name: "Offsetof", Phase: 3},
	{Pkg: "unsafe", Name: "Sizeof", Phase: 3},
	{Pkg: "iter", Name: "Pull", Phase: 8},
	{Pkg: "iter", Name: "Pull2", Phase: 8},
	{Pkg: "errors", Name: "Trace", Phase: 10},
}

// builtins are the built-in functions that Phase 02 doesn't have (it has
// len of a string, print, println and panic).
var builtins = []stdMember{
	{Name: "append", Phase: 3},
	{Name: "cap", Phase: 3},
	{Name: "clear", Phase: 3},
	{Name: "complex", Phase: 3},
	{Name: "copy", Phase: 3},
	{Name: "delete", Phase: 3},
	{Name: "imag", Phase: 3},
	{Name: "make", Phase: 3},
	{Name: "max", Phase: 3},
	{Name: "min", Phase: 3},
	{Name: "new", Phase: 3},
	{Name: "real", Phase: 3},
	{Name: "recover", Phase: 5},
	{Name: "close", Phase: 8},
}

// formatVerbs are the verbs of the format strings of fmt.Printf, Sprintf
// and Errorf with the phase that prints them (Phase 02 README §2.6, Phase
// 03 README §2.10, Phase 05 README §2.8 and step 5.11: Go's whole fmt for
// the values of the language). A character that is no verb of Go's fmt
// (%!, %z) is fine from Phase 02: Go and gocpp both print the error form
// %!z(int=1) for it.
var formatVerbs = []struct {
	Verb  rune
	Phase int
}{
	{'%', 2}, {'d', 2}, {'f', 2}, {'s', 2}, {'t', 2}, {'v', 2},
	{'q', 3},
	{'T', 5}, {'w', 5}, {'b', 5}, {'c', 5}, {'o', 5}, {'O', 5}, {'x', 5}, {'X', 5}, {'U', 5},
	{'e', 5}, {'E', 5}, {'F', 5}, {'g', 5}, {'G', 5}, {'p', 5},
}

// goVerbs are the verbs of Go's fmt (and %w of fmt.Errorf).
const goVerbs = "vTtbcdoOqxXUeEfFgGspw%"

// verbPhase returns the phase of the format verb.
func verbPhase(verb rune) int {
	for _, v := range formatVerbs {
		if v.Verb == verb {
			return v.Phase
		}
	}
	if strings.ContainsRune(goVerbs, verb) {
		return 9
	}
	return 2
}

// verbMessage returns the error for a format verb of a later phase.
func verbMessage(verb rune, phase int) string {
	return gateMessage(fmt.Sprintf("the format verb %%%c", verb), false, phase)
}

// lookupStdPackage returns the row of the standard package path, or nil.
func lookupStdPackage(path string) *stdPackage {
	for i := range stdPackages {
		p := &stdPackages[i]
		if p.Path == path {
			return p
		}
		if dir, ok := strings.CutSuffix(p.Path, "/..."); ok && strings.HasPrefix(path, dir+"/") {
			return p
		}
	}
	return nil
}

// lookupMember returns the phase row of the member pkg.name ("Type.Method"
// for a method), for a package whose members don't all come in the phase
// of the package, or nil.
func lookupMember(pkg, name string) *stdMember {
	for i := range stdMembers {
		if stdMembers[i].Pkg == pkg && stdMembers[i].Name == name {
			return &stdMembers[i]
		}
	}
	if row := lookupStdPackage(pkg); row != nil && row.Rest > row.Phase {
		return &stdMember{Pkg: pkg, Name: name, Phase: row.Rest}
	}
	return nil
}

// lookupBuiltin returns the row of the built-in function name, or nil for a
// built-in of Phase 02.
func lookupBuiltin(name string) *stdMember {
	for i := range builtins {
		if builtins[i].Name == name {
			return &builtins[i]
		}
	}
	return nil
}
