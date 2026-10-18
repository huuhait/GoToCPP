// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: comparing uncomparable type main\.listErr\n

// Go does not make errors.Is safe for a comparable error type that holds an uncomparable error: the == inside panics.

package main

import (
	"errors"
	"fmt"
)

type listErr struct{ parts []string }

func (listErr) Error() string { return "list" }

type holder struct{ e error }

func (holder) Error() string { return "holder" }

func main() {
	a := holder{listErr{[]string{"a"}}}
	b := holder{listErr{[]string{"a"}}}
	fmt.Println(errors.Is(a, errors.New("other")))
	fmt.Println(errors.Is(a, b))
}

// Output:
// false
