// Package big is a small stand-in for math/big (which gocpp does not have): an Int with the methods that Go's tests of encoding/json call.
package big

import (
	"errors"
	"strconv"
)

// An Int is a signed integer of 64 bits.
type Int struct {
	v int64
}

// NewInt allocates and returns a new Int set to x.
func NewInt(x int64) *Int { return &Int{x} }

// String returns the decimal representation of x.
func (x *Int) String() string {
	if x == nil {
		return "<nil>"
	}
	return strconv.FormatInt(x.v, 10)
}

// MarshalText implements the encoding.TextMarshaler interface.
func (x *Int) MarshalText() ([]byte, error) {
	if x == nil {
		return []byte("<nil>"), nil
	}
	return []byte(x.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (x *Int) UnmarshalText(text []byte) error {
	v, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return errors.New("math/big: cannot unmarshal " + strconv.Quote(string(text)) + " into a *big.Int")
	}
	x.v = v
	return nil
}
