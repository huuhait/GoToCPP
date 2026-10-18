// phase: 06

// errors.As with types of another package: a pointer type, a value type, an unexported type reached through an interface that only main declares.

package main

import (
	"errors"
	"fmt"

	"example.com/crossas/apierr"
)

type Temporary interface{ Temporary() bool }

type Retrier interface {
	error
	After() int
}

func main() {
	nf := apierr.Wrap("get", apierr.Wrap("db", apierr.NotFound("user")))
	var ae *apierr.Error
	fmt.Println(errors.As(nf, &ae), ae.Code, ae.Msg)
	fmt.Println(nf.Error())

	var fatal apierr.Fatal
	b := apierr.Broken("disk")
	fmt.Println(errors.As(b, &fatal), fatal.Reason, errors.As(nf, &fatal))

	busy := apierr.Busy(30)
	var t Temporary
	fmt.Println(errors.As(busy, &t), t.Temporary())
	var r Retrier
	fmt.Println(errors.As(busy, &r), r.After(), r.Error())
	fmt.Println(errors.As(b, &t), errors.As(apierr.Plain(), &r))

	var any1 error
	fmt.Println(errors.As(busy, &any1), any1 == busy)
	var ae2 *apierr.Error
	fmt.Println(errors.As(busy, &ae2), ae2 == nil)
}

// Output:
// true 404 user not found
// get: db: user not found
// true disk false
// true true
// true 30 retry later
// false false
// true true
// false true
