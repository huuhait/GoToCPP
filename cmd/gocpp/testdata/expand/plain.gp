// Plain has nothing to expand: expand prints it as it is.
package main

import "fmt"

type T struct {
	A   int    `json:"a"`
	Bee string `json:"bee"` // trailing
}

/* block
comment */
func   main( )   {
	x   :=  T{A:1}
	fmt.Println( x )
}

var (
	a = 1 // one
	b = 2
)
