// bench: panic_recover
// category: runtime
// size: 3000000
// smoke: 5000
// twin: no
// about: panic and recover in a loop (a function that panics with an int or an error, and a deferred recover)
package main

import (
	"errors"
	"fmt"
)

var errBad = errors.New("bad")

func risky(i int) (result int, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case int:
				result = v
			case error:
				err = v
			}
		}
	}()
	if i%3 == 0 {
		panic(i)
	}
	if i%3 == 1 {
		panic(errBad)
	}
	return i, nil
}

func main() {
	sum, bad := 0, 0
	for i := 0; i < benchN; i++ {
		r, err := risky(i)
		sum += r
		if err != nil {
			bad++
		}
	}
	fmt.Println(sum, bad)
}
