// phase: 05

// Package boot has an init! that fails: the program prints "<program>: <error>" and exits with 1
// before main starts; the packages that boot imports have been initialized.
package boot

import (
	"errors"
	"fmt"

	"example.com/initfail/first"
)

var ErrBoot = errors.New("boot failed")

func init!() {
	fmt.Println("boot: init after", first.Name)
	check(first.Name)
	fmt.Println("boot: not reached")
}

func check(name string) error {
	return fmt.Errorf("check %s: %w", name, ErrBoot)
}
