package lib

import (
	"errors"
	"strconv"
)

var ErrEmpty = errors.New("empty")

// Parse is a must function: callers see func Parse(s string) (int, error).
func Parse!(s string) int {
	if s == "" {
		return 0, ErrEmpty
	}
	n := strconv.Atoi(s)
	return n
}
