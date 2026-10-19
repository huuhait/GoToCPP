package probe

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// ErrNoClang is the error when typed interop can't run because no clang++ is
// available (step 12.11 falls back to the opaque pipeline on it). The probes
// need clang's JSON AST dump; g++ can't give it.
var ErrNoClang = errors.New("typed interop needs clang++")

// A Category is the value category of a probed expression.
type Category string

const (
	CatNone    Category = ""       // a type probe
	CatLValue  Category = "lvalue" // decltype((E)) is T&
	CatXValue  Category = "xvalue" // T&&
	CatPRValue Category = "prvalue"
)

// A Diag is a message of the compiler.
type Diag struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Col      int    `json:"col,omitempty"`
	Severity string `json:"severity"` // "error", "fatal error", "note"
	Msg      string `json:"msg"`
}

// A Const is the compile-time value of a constant.
type Const struct {
	// Kind is "int" (also an enumerator), "bool", "float" or "string".
	Kind string `json:"kind"`

	// Int is the value of an int, bool (0 or 1) or enumerator; it can be
	// negative or larger than 64 bits (clang's constant evaluator computes in
	// __int128, and an unsigned long long macro is exact).
	Int *big.Int `json:"-"`

	// Float is the value of a float constant; the probe reads its exact bits
	// (math.Float64frombits), so the value is not rounded through text.
	Float float64 `json:"-"`

	// Str is the value of a string literal (adjacent literals joined).
	Str string `json:"-"`
}

// MarshalJSON writes the value as text for Int (exact for any size).
func (c *Const) MarshalJSON() ([]byte, error) {
	type out struct {
		Kind  string `json:"kind"`
		Value any    `json:"value"`
	}
	switch c.Kind {
	case "int", "bool":
		return json.Marshal(out{c.Kind, c.Int.String()})
	case "float":
		if math.IsInf(c.Float, 0) || math.IsNaN(c.Float) {
			return json.Marshal(out{c.Kind, fmt.Sprint(c.Float)})
		}
		return json.Marshal(out{c.Kind, c.Float})
	}
	return json.Marshal(out{c.Kind, c.Str})
}

// String returns the value as text.
func (c *Const) String() string {
	switch c.Kind {
	case "int", "bool":
		return c.Int.String()
	case "float":
		return fmt.Sprint(c.Float)
	}
	return fmt.Sprintf("%q", c.Str)
}

// A Result is the answer for one reference.
type Result struct {
	ID  int    `json:"id"`
	Key string `json:"key"`

	// Skipped is why the reference was not probed (an incomplete reference,
	// a name that is only a callee, a namespace): then nothing else is set.
	Skipped string `json:"skipped,omitempty"`

	// OK says that a probe compiled: Type and the fields after it are set.
	OK bool `json:"ok"`

	// Form is whether the probe that worked was an expression or a type.
	Form Form `json:"form,omitempty"`

	// Alt says that the alternative reading of an ambiguous reference is the
	// one that worked.
	Alt bool `json:"alt,omitempty"`

	// Type is the canonical type that clang printed: for an expression,
	// the type of decltype((E)) (so "long &" for an lvalue of type long); for
	// a type probe, the type itself ("std::basic_string<char>").
	Type string `json:"type,omitempty"`

	// Written is the type with its typedef names kept, as the declaration
	// spelled it: "FILE *" where Type says "_IO_FILE *", "size_t" for
	// "unsigned long", "value_type &" for a member typedef (unqualified, as
	// the class has it). Empty when it is the same as Type, or when the
	// compiler's dump has no such tree.
	Written string `json:"written,omitempty"`

	// Base is Type without the reference, Ref is "", "&" or "&&", Category the
	// value category of an expression and Const says that Base is const at
	// the top (const int, char *const).
	Base     string   `json:"base,omitempty"`
	Ref      string   `json:"ref,omitempty"`
	Category Category `json:"category,omitempty"`
	Const    bool     `json:"const,omitempty"`

	// Value is the constant value, when a value probe worked.
	Value *Const `json:"value,omitempty"`

	// ValueError is the compiler's error for a value probe that failed (the
	// expression is not a constant, or not an arithmetic type or string); it
	// is not an error of the reference.
	ValueError string `json:"valueError,omitempty"`

	// Error is the compiler's first error for the probe when it failed, and
	// Notes are the notes that follow it (clang lists the overload
	// candidates there: "candidate function not viable: ..."). AltError is
	// the error of the alternative reading, when there is one.
	Error    string `json:"error,omitempty"`
	Notes    []Diag `json:"notes,omitempty"`
	AltError string `json:"altError,omitempty"`
	// ErrorAt is where the error is (the probe file's line, or a header's).
	ErrorAt *Diag `json:"errorAt,omitempty"`
}

// setWritten records the written type, unless it adds nothing.
func (r *Result) setWritten(w string) {
	if w != "" && w != r.Type {
		r.Written = w
	}
}

// splitType derives Base, Ref, Category and Const from the canonical type of
// an expression probe. It looks at the declarator only: the arguments of
// templates (std::vector<int *>, std::pair<const char *, int>) are blanked
// first, so their '*', '(' and '&' say nothing about the type.
func (r *Result) splitType(t string, expr bool) {
	r.Type = t
	r.Base = t
	if !expr {
		return
	}
	r.Category = CatPRValue
	s := blankAngles(t)
	// A reference-qualified function type or pointer to member function
	// ("void (S::*)() &") ends with '&' but is a plain value: the '&' follows
	// the parameter list. A reference to a function or array has its '&' in
	// the parentheses ("int (&)[4]").
	funcTail := false
	if i := strings.LastIndexByte(s, ')'); i >= 0 && strings.ContainsAny(s[i:], "&") {
		funcTail = true
	}
	switch {
	case funcTail:
	case strings.HasSuffix(s, "&&"):
		r.Base, r.Ref, r.Category = strings.TrimSpace(strings.TrimSuffix(t, "&&")), "&&", CatXValue
	case strings.HasSuffix(s, "&"):
		r.Base, r.Ref, r.Category = strings.TrimSpace(strings.TrimSuffix(t, "&")), "&", CatLValue
	case strings.Contains(s, "(&)"):
		// A reference to an array or a function: int (&)[4].
		i := strings.Index(s, "(&)")
		r.Base, r.Ref, r.Category = t[:i]+t[i+3:], "&", CatLValue
	case strings.Contains(s, "(&&)"):
		i := strings.Index(s, "(&&)")
		r.Base, r.Ref, r.Category = t[:i]+t[i+4:], "&&", CatXValue
	case strings.Contains(s, "*&&)"):
		// A reference to a function pointer or a member pointer: void (*&&)(int).
		i := strings.Index(s, "*&&)")
		r.Base, r.Ref, r.Category = t[:i+1]+t[i+3:], "&&", CatXValue
	case strings.Contains(s, "*&)"):
		i := strings.Index(s, "*&)")
		r.Base, r.Ref, r.Category = t[:i+1]+t[i+2:], "&", CatLValue
	}
	b := blankAngles(r.Base)
	r.Const = strings.HasSuffix(b, "*const") ||
		(strings.HasPrefix(b, "const ") && !strings.ContainsAny(b, "*[("))
}

// blankAngles returns t with everything inside <...> (nested) replaced by
// spaces, keeping the length, so that an index in the result is an index in t.
func blankAngles(t string) string {
	if !strings.Contains(t, "<") {
		return t
	}
	b := []byte(t)
	depth := 0
	for i, c := range b {
		switch {
		case c == '<':
			depth++
		case c == '>' && depth > 0:
			depth--
		case depth > 0:
			b[i] = ' '
		}
	}
	return string(b)
}

// A Timing says where the time went.
type Timing struct {
	Generate time.Duration `json:"generate"` // building the probe file (Runner.Probe only)
	Compile  time.Duration `json:"compile"`  // the compiler run, including the reading of its output
	Parse    time.Duration `json:"parse"`    // decoding the declarations of the dump (part of Compile)
	CPU      time.Duration `json:"cpu"`      // the compiler's own CPU time (user and system): does not depend on the load
	Total    time.Duration `json:"total"`
}

// Stats are numbers about the run, for tests and the developer tool.
type Stats struct {
	Decls           int   `json:"decls"`           // declarations in the dump
	JSONBytes       int64 `json:"jsonBytes"`       // size of the dump
	LargestDecl     int64 `json:"largestDecl"`     // size of its largest declaration
	TooLarge        int   `json:"tooLarge"`        // declarations over the limit (their probes failed)
	StderrBytes     int   `json:"stderrBytes"`     // the compiler's messages
	StderrTruncated bool  `json:"stderrTruncated"` // some were dropped (limit)
	ExitCode        int   `json:"exitCode"`
}

// A Table is the typed result table of one package: one Result per
// reference, indexed by the reference ID.
type Table struct {
	Source  *Source  `json:"-"`
	Results []Result `json:"results"`

	// HeaderErrors are the compiler's errors that belong to no probe: an
	// error in an unrelated header, a header that isn't found (also
	// "clang++: error: ..." lines). Probes still ran.
	HeaderErrors []Diag `json:"headerErrors,omitempty"`

	// Compiler identifies the compiler (path and version), Key is the hash
	// that a cache can use (Key), Deps are the headers the compiler read.
	Compiler string   `json:"compiler"`
	Key      string   `json:"key"`
	Deps     []string `json:"deps,omitempty"`

	// Cached says that Runner served the table from its memo.
	Cached bool `json:"cached,omitempty"`

	Timing Timing `json:"timing"`
	Stats  Stats  `json:"stats"`

	// Stderr is the compiler's own text, for debugging (not parsed lines
	// only: all of it, up to the limit).
	Stderr string `json:"-"`
}

// Result returns the result of the reference id.
func (t *Table) Result(id int) *Result {
	if id < 0 || id >= len(t.Results) {
		return nil
	}
	return &t.Results[id]
}

// compilerID returns a text that identifies the compiler.
func compilerID(cxx build.CXX) string {
	return cxx.Path + " " + cxx.Version
}
