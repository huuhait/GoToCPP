// Package quick stands in for testing/quick in Go's tests of the standard packages (step 9.10). Go's package builds the arguments of
// the function under test and calls it with reflect.Value.Call, which gocpp's reflect subset does not have; this one checks the
// function types that the tests use, func(int64) bool and func(int32) bool, with random values over the whole range of the type.
package quick

import (
	"fmt"
	"math/rand"
)

// A Config structure contains options for running a test.
type Config struct {
	// MaxCount sets the maximum number of iterations. If zero, MaxCountScale is used.
	MaxCount int
	// MaxCountScale is a non-negative scale factor applied to the default maximum (100). If zero, the default is unchanged.
	MaxCountScale float64
	// Rand specifies a source of random numbers. If nil, a default pseudo-random source is used.
	Rand *rand.Rand
}

var defaultMaxCount = 100

func (c *Config) getRand() *rand.Rand {
	if c == nil || c.Rand == nil {
		return rand.New(rand.NewSource(1))
	}
	return c.Rand
}

func (c *Config) getMaxCount() int {
	maxCount := c.MaxCount
	if maxCount == 0 {
		if c.MaxCountScale != 0 {
			maxCount = int(c.MaxCountScale * float64(defaultMaxCount))
		} else {
			maxCount = defaultMaxCount
		}
	}
	return maxCount
}

// A CheckError is the result of Check finding an error.
type CheckError struct {
	Count int
	In    []any
}

func (s *CheckError) Error() string {
	return fmt.Sprintf("#%d: failed on input %s", s.Count, toString(s.In))
}

// An unsupportedError says that Check does not know the type of the function.
type unsupportedError struct {
	f any
}

func (e unsupportedError) Error() string {
	return fmt.Sprintf("quick (gocpp shim): the function type %T is not supported", e.f)
}

func toString(in []any) string {
	s := ""
	for i, v := range in {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprint(v)
	}
	return s
}

// Check looks for an input to f, any function that returns bool, such that f returns false. It calls f repeatedly, with arbitrary
// values for each argument. If f returns false on a given input, Check returns that input as a *CheckError.
func Check(f any, config *Config) error {
	if config == nil {
		config = &Config{}
	}
	rnd := config.getRand()
	maxCount := config.getMaxCount()
	for i := 0; i < maxCount; i++ {
		switch fn := f.(type) {
		case func(int64) bool:
			arg := int64(rnd.Uint64())
			if !fn(arg) {
				return &CheckError{Count: i + 1, In: []any{arg}}
			}
		case func(int32) bool:
			arg := int32(rnd.Uint64())
			if !fn(arg) {
				return &CheckError{Count: i + 1, In: []any{arg}}
			}
		default:
			return unsupportedError{f}
		}
	}
	return nil
}

