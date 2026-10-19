package lib

import (
	"errors"
	"strconv"
)

var ErrEmpty = errors.New("empty")

func Parse(s string) (int, error) {
	if s == "" {
		return 0, ErrEmpty
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return n, nil
}
