// This file is gocpp's own, not part of Go's errors package (docs/spec/language.md, D17).

package errors

// Wrapf puts context in front of the error that *errp holds, and does nothing
// when *errp is nil. It is meant for defer in a function that returns an error
// in a named result, mostly one with ! (a must function, or a function that
// uses F!(...)):
//
//	func load!(path string) (cfg Config, err error) {
//		defer errors.Wrapf(&err, "load %s", path)
//		...
//	}
//
// The new error reads "context: original", with the context formatted like
// fmt.Sprintf(format, args...) does. It wraps the original (it has an Unwrap
// method), so [Is] and [As] still find the original error, as with
// fmt.Errorf("%s: %w", ...).
//
// Wrapf takes the address of the named result: a defer sees the value that
// the function returns, also after a return statement sets it, or after a !
// call returned an error.
func Wrapf(errp *error, format string, args ...any) {
	if *errp == nil {
		return
	}
	*errp = &wrapfError{msg: sprintf(format, args), err: *errp}
}

// sprintf is fmt.Sprintf. It is a native function of the runtime (gp::errors::sprintf, the fmt engine), not a call
// of package fmt: package fmt imports io, which imports errors, so errors can not import fmt (step 9.6).
//
//gocpp:native gp::errors::sprintf
func sprintf(format string, args []any) string

type wrapfError struct {
	msg string
	err error
}

func (e *wrapfError) Error() string { return e.msg + ": " + e.err.Error() }

func (e *wrapfError) Unwrap() error { return e.err }
