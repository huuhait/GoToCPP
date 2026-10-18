package a

import "errors"

type Err struct{ N int }

func (e *Err) Error() string { return "x/a.Err" }

var Sentinel = errors.New("sentinel")

func Make() error { return &Err{1} }

type errorString struct{ s string }

func (e *errorString) Error() string { return e.s }

func Own() error { return &errorString{"mine"} }
