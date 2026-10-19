// phase: 09
// errors.As when the error's type and the target's type are different types with the same underlying type and one of them is unnamed (Go's AssignableTo): the value is stored, in both directions; named types with the same underlying type do not match.

package main

import (
	"errors"
	"fmt"
)

type MyErr struct{ code int }

func (e MyErr) Error() string { return fmt.Sprint("myerr ", e.code) }

type Emb struct{ MyErr }
type Emb2 struct{ MyErr }

type Int int

func (i Int) Error() string { return fmt.Sprint("int", int(i)) }

type Other int

func (i Other) Error() string { return fmt.Sprint("other", int(i)) }

func main() {
	var e error = struct{ MyErr }{MyErr{3}}
	var t Emb
	ok := errors.As(e, &t)
	fmt.Println("unnamed -> named", ok, t.code)

	e = Emb{MyErr{4}}
	var u struct{ MyErr }
	ok = errors.As(e, &u)
	fmt.Println("named -> unnamed", ok, u.code)

	e = Emb{MyErr{5}}
	var t2 Emb2
	ok = errors.As(e, &t2)
	fmt.Println("named -> other named", ok, t2.code)

	e = Emb{MyErr{6}}
	var t3 Emb
	ok = errors.As(fmt.Errorf("wrapped: %w", e), &t3)
	fmt.Println("same type through a wrap", ok, t3.code)

	var o Other
	fmt.Println("Int -> Other", errors.As(Int(7), &o), o)
	var i Int
	fmt.Println("Int -> Int", errors.As(Int(7), &i), i)
}

// Output:
// unnamed -> named true 3
// named -> unnamed true 4
// named -> other named false 0
// same type through a wrap true 6
// Int -> Other false other0
// Int -> Int true int7
