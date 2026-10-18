package apierr

import "errors"

// Error is an API error with a status code.
type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

// Fatal is an error type by value.
type Fatal struct{ Reason string }

func (f Fatal) Error() string { return "fatal: " + f.Reason }

type retryable struct{ after int }

func (r retryable) Error() string   { return "retry later" }
func (r retryable) Temporary() bool { return true }
func (r retryable) After() int      { return r.after }

type wrapped struct {
	op  string
	err error
}

func (w *wrapped) Error() string { return w.op + ": " + w.err.Error() }
func (w *wrapped) Unwrap() error { return w.err }

func Wrap(op string, err error) error { return &wrapped{op, err} }

func NotFound(what string) error { return &Error{404, what + " not found"} }

func Busy(after int) error { return Wrap("busy", retryable{after}) }

func Broken(why string) error { return Wrap("broken", Fatal{why}) }

func Plain() error { return errors.New("plain") }
