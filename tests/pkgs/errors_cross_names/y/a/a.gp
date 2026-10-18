package a

import "errors"

type Err struct{ N int }

func (e *Err) Error() string { return "y/a.Err" }

var Sentinel = errors.New("sentinel")

func Make() error { return &Err{2} }
