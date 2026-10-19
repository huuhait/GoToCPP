package boot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"example.com/initfail/first"
)

var ErrBoot = errors.New("boot failed")

func init() {
	err := func() error {
		fmt.Println("boot: init after", first.Name)
		if err := check(first.Name); err != nil {
			return err
		}
		fmt.Println("boot: not reached")
		return nil
	}()
	if err != nil {
		fmt.Fprintln(os.Stderr, filepath.Base(os.Args[0])+": "+err.Error())
		os.Exit(1)
	}
}

func check(name string) error {
	return fmt.Errorf("check %s: %w", name, ErrBoot)
}
