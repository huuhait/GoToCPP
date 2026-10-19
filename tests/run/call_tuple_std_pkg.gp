// phase: 05

package main

import (
	"errors"
	"fmt"
)

var sentinel = errors.New("sentinel")

func pair() (error, error) {
	return fmt.Errorf("wrapped: %w", sentinel), sentinel
}

// f(g()) where f is a function of another package (errors.Is): the call goes through std::apply
// with the function's qualified name.
func main() {
	fmt.Println(errors.Is(pair()))
	fmt.Println(errors.Is(sentinel, errors.New("other")))
}

// Output:
// true
// false
