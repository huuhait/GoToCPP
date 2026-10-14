// phase: 03

// Conversions between struct types that differ only in their tags (step 3.9), also in nested
// struct, array and pointer fields: a pointer field still points to the same struct (Go shares
// it), an array field is copied.
package main

import "fmt"

type W struct {
	A struct {
		Q int `json:"q"`
	}
	R [2]struct {
		Q int `json:"q"`
	}
	P *struct {
		Q int `json:"q"`
	}
}

type V struct {
	A struct{ Q int }
	R [2]struct{ Q int }
	P *struct{ Q int }
}

func main() {
	w := W{}
	w.A.Q = 1
	w.R[1].Q = 2
	w.P = &struct {
		Q int `json:"q"`
	}{3}
	v := V(w)
	v.P.Q = 30
	v.R[1].Q = 20
	fmt.Println(v.A.Q, v.R[1].Q, w.R[1].Q, v.P.Q, w.P.Q)
	back := W(v)
	fmt.Println(back.A.Q, back.R[1].Q, back.P == w.P)
}
