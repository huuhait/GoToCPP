package b

import "errors"

type Err struct{ N int }

func (e *Err) Error() string { return "b.Err" }

func Make() error { return &Err{3} }

func Plain() error { return errors.New("b plain") }
