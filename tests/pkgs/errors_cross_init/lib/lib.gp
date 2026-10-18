package lib

import (
	"errors"
)

var (
	ErrA = errors.New("lib: a")
	ErrB = errors.New("lib: b")
	All  = errors.Join(ErrA, ErrB)
)

var Order []string

func init() {
	Order = append(Order, "lib init")
	ErrA = errors.New("lib: a replaced")
}

func Reset() error { return errors.Join(ErrA, nil, ErrB) }
