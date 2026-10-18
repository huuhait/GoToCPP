package more

import (
	"errors"

	"example.com/crossinit/lib"
)

var Combined = errors.Join(lib.ErrA, errors.New("more"))

var Same = lib.ErrB

func init() {
	lib.Order = append(lib.Order, "more init")
}
