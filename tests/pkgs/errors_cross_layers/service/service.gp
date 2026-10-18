package service

import (
	"errors"

	"example.com/layers/db"
)

var ErrInvalid = errors.New("service: invalid")

type Error struct {
	Op  string
	Err error
}

func (e Error) Error() string { return e.Op + " failed: " + e.Err.Error() }
func (e Error) Unwrap() error { return e.Err }

func (e Error) Is(target error) bool { return target == ErrInvalid && e.Op == "validate" }

func Load(id int) error {
	if id == 0 {
		return Error{"validate", errors.New("zero id")}
	}
	if err := db.Find(id); err != nil {
		return Error{"load", err}
	}
	return nil
}
