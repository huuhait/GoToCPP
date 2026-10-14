package diag

import (
	"cmp"
	"fmt"
	"go/token"
	"go/types"
	"reflect"
	"slices"
	"strings"
)

// maxErrors is the number of errors that Print shows before it stops with
// a "too many errors" line. The go compiler stops after 10 errors too,
// unless it gets -e.
const maxErrors = 10

// UnusedImport is the go/types error code (see ErrorCode and
// Diagnostic.Code) of an import that is not used:
// `"strings" imported and not used`.
const UnusedImport = 8

// UnusedVar is the go/types error code of a local variable that is not
// used: "declared and not used: x".
const UnusedVar = 101

// ErrorCode returns the error code of a go/types error, such as
// UnusedImport, or 0 if it is not known. A filter in front of List.Collect
// uses it, for example to hold back the "imported and not used" errors
// and pass them on later. Collect puts the same code in Diagnostic.Code.
func ErrorCode(err types.Error) int {
	code, _ := typeErrorData(err)
	return code
}

// Severity says how serious a diagnostic is.
type Severity int

const (
	// Error means that the package cannot be built. It is the zero value.
	Error Severity = iota
	// Warning means that the package can be built, but something looks
	// wrong.
	Warning
)

// String returns "error" or "warning".
func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	}
	return fmt.Sprintf("Severity(%d)", int(s))
}

// A Diagnostic is one error or warning in the source code.
type Diagnostic struct {
	// Pos is the place of the problem, and the place that is printed.
	// Like everywhere in go/token, lines and columns start at 1, and a
	// column counts bytes, not characters.
	Pos token.Position

	// End is the end of the source range that starts at Pos. Editors use
	// it to underline the code. It is the zero Position if the range is
	// not known.
	End token.Position

	Severity Severity
	Msg      string

	// Soft is the "soft error" flag of go/types. A soft error, such as
	// "declared and not used" or "imported and not used", does not make
	// the type information wrong. An error that is not soft is a hard
	// error. Later steps use the flag: when there are no hard errors, the
	// opaque pass (step 1.11) can still trust the types, and the language
	// server can keep hover and completion working.
	Soft bool

	// Code is the go/types error code, such as UnusedImport. It is 0 for
	// an error that doesn't come from go/types, or if the code is not
	// known. The values are those of Go's internal/types/errors package,
	// and Go doesn't change them.
	Code int

	// Related lists more places that belong to the message, such as
	// "other declaration of x". Print shows each one on its own line,
	// after a tab, under the message.
	Related []Related
}

// A Related is another place that a Diagnostic points to.
type Related struct {
	Pos token.Position // the zero Position if there is no place
	Msg string
}

// A List collects the diagnostics of one package, in the order they are
// added. The zero List is empty and ready to use.
//
// A List is not safe for use by several goroutines at the same time.
// But HasErrors, Diagnostics and Printer.Print only read it, so they may
// run at the same time as each other.
type List struct {
	diags []Diagnostic
}

// Add adds a diagnostic.
func (l *List) Add(d Diagnostic) {
	l.diags = append(l.diags, d)
}

// Errorf adds an error at pos. The arguments are formatted like
// fmt.Sprintf. A message that is already a string is added with
// l.Errorf(pos, "%s", msg).
func (l *List) Errorf(pos token.Position, format string, args ...any) {
	l.Add(Diagnostic{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

// Collect adds an error from go/types. It has the type that the Error
// field of types.Config wants:
//
//	var diags diag.List
//	conf := types.Config{Error: diags.Collect}
//
// go/types reports a message with more places, like
// "x redeclared in this block" and "other declaration of x", as several
// errors in a row, and each error after the first one starts with a tab.
// Collect adds such an error to the diagnostic before it, as a Related
// place, so that Print shows the message like go build does. So a filter
// between go/types and Collect (see ErrorCode) must drop or hold back
// these errors together with the error before them.
//
// Collect also keeps the error code (Code) and the end of the source range
// (End) of the error. An error that is not a types.Error is added without
// a position.
func (l *List) Collect(err error) {
	terr, ok := err.(types.Error)
	if !ok {
		l.Add(Diagnostic{Msg: err.Error()})
		return
	}
	var pos token.Position
	if terr.Fset != nil {
		pos = terr.Fset.Position(terr.Pos)
	}
	msg, more := strings.CutPrefix(terr.Msg, "\t")
	if more && len(l.diags) > 0 {
		last := &l.diags[len(l.diags)-1]
		last.Related = append(last.Related, Related{Pos: pos, Msg: msg})
		return
	}
	code, end := typeErrorData(terr)
	d := Diagnostic{Pos: pos, Msg: msg, Soft: terr.Soft, Code: code}
	if terr.Fset != nil && end > terr.Pos {
		d.End = terr.Fset.Position(end)
	}
	l.Add(d)
}

// typeErrorData returns the error code of a go/types error and the end of
// its source range. go/types keeps them in unexported fields, and gopls
// reads them in the same way, with reflect
// (golang.org/x/tools/internal/typesinternal). If a new Go version renames
// a field, its value is 0 (token.NoPos for end), and TestCollect fails.
func typeErrorData(err types.Error) (code int, end token.Pos) {
	v := reflect.ValueOf(err)
	if f := v.FieldByName("go116code"); f.Kind() == reflect.Int {
		code = int(f.Int())
	}
	if f := v.FieldByName("go116end"); f.Kind() == reflect.Int {
		end = token.Pos(f.Int())
	}
	return code, end
}

// Rewrite returns a new list with the diagnostics of l, in the order they
// were added, each one replaced by what f returns for it: nothing drops it,
// the diagnostic itself keeps it, and other diagnostics take its place (in
// the order that f gives them). Unlike Diagnostics, it keeps the order in
// which the diagnostics were found, which the limit of 10 errors needs.
// The continuation lines of a diagnostic (Related) are part of it. l does
// not change.
func (l *List) Rewrite(f func(d Diagnostic) []Diagnostic) *List {
	n := new(List)
	for _, d := range l.diags {
		d.Related = slices.Clone(d.Related) // f may change them
		n.diags = append(n.diags, f(d)...)
	}
	return n
}

// HasErrors reports whether the list has an error (a warning is not an
// error).
func (l *List) HasErrors() bool {
	for i := range l.diags {
		if l.diags[i].Severity == Error {
			return true
		}
	}
	return false
}

// Diagnostics returns the diagnostics that Printer.Print shows, in the same
// order, but all of them: without the limit of 10 errors. The result is a
// new slice that the caller may change.
//
// The result is sorted, so don't add it to another List. A List needs the
// order in which the diagnostics were found: the limit of 10 errors keeps
// the first 10 found, not the first 10 by position.
func (l *List) Diagnostics() []Diagnostic {
	ds, _, _ := l.view(0)
	return ds
}

// view returns the diagnostics that Print shows, in the order it shows
// them. It works like the go compiler (cmd/compile/internal/base/print.go):
//
//  1. It goes through the diagnostics in the order they were added. It
//     drops an error that has the same file, line and message as the last
//     error it kept. So "x + x" with an undefined x gives one error, not
//     two. Warnings are not dropped and do not count as the last error.
//  2. If limit > 0, it stops at the error number limit. It returns the
//     position of that error for the "too many errors" line (stop is
//     true). The errors after it are not shown, even if they come earlier
//     in the file.
//  3. It sorts by position. Files come in the order of their first
//     diagnostic, not by name, because the go compiler does it in that
//     way. Inside a file, it sorts by line, then column. Diagnostics
//     without a file name come first. The sort is stable: diagnostics at
//     the same place stay in the order they were added.
//  4. It drops exact duplicates: the same place and the same message.
func (l *List) view(limit int) (ds []Diagnostic, stopPos token.Position, stop bool) {
	fileOrder := make(map[string]int)
	var last *Diagnostic // the last error that was kept
	count := 0           // the errors that were kept
	for i := range l.diags {
		d := &l.diags[i]
		if d.Severity == Error {
			if last != nil && sameLine(last.Pos, d.Pos) && sameMessage(last, d) {
				continue
			}
			last = d
			count++
		}
		if _, ok := fileOrder[d.Pos.Filename]; !ok {
			fileOrder[d.Pos.Filename] = len(fileOrder)
		}
		c := *d
		c.Related = slices.Clone(d.Related)
		ds = append(ds, c)
		if limit > 0 && d.Severity == Error && count == limit {
			stopPos, stop = d.Pos, true
			break
		}
	}

	order := func(pos token.Position) int {
		if pos.Filename == "" {
			return -1
		}
		return fileOrder[pos.Filename]
	}
	slices.SortStableFunc(ds, func(a, b Diagnostic) int {
		return cmp.Or(
			cmp.Compare(order(a.Pos), order(b.Pos)),
			cmp.Compare(a.Pos.Line, b.Pos.Line),
			cmp.Compare(a.Pos.Column, b.Pos.Column),
		)
	})
	ds = slices.CompactFunc(ds, func(a, b Diagnostic) bool {
		return samePlace(a.Pos, b.Pos) && a.Severity == b.Severity && sameMessage(&a, &b)
	})
	return ds, stopPos, stop
}

// sameLine reports whether a and b are on the same line of the same file.
func sameLine(a, b token.Position) bool {
	return a.Filename == b.Filename && a.Line == b.Line
}

// samePlace reports whether a and b print the same: the same file, line and
// column.
func samePlace(a, b token.Position) bool {
	return a.Filename == b.Filename && a.Line == b.Line && a.Column == b.Column
}

// sameMessage reports whether a and b have the same message and the same
// related places, so that their text after the position is the same.
func sameMessage(a, b *Diagnostic) bool {
	return a.Msg == b.Msg && slices.EqualFunc(a.Related, b.Related, func(x, y Related) bool {
		return samePlace(x.Pos, y.Pos) && x.Msg == y.Msg
	})
}
