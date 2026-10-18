// phase: 05

// Every New in a loop is its own error: no two of them are equal, and each equals itself.

package main

import (
	"errors"
	"fmt"
)

func main() {
	var errs []error
	for i := 0; i < 5; i++ {
		errs = append(errs, errors.New("same"))
	}
	same, diff := 0, 0
	for i := range errs {
		for j := range errs {
			if errs[i] == errs[j] {
				same++
			} else {
				diff++
			}
		}
	}
	fmt.Println(same, diff)
}

// Output:
// 5 20
