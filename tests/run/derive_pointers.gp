// phase: 10

// Pointers to types of other packages and field types with a Clone method of their own (step 10.10).
// A pointer to a type of another package (*time.Location, *bytes.Buffer, *sync.Mutex) is shared by
// Clone and compared by identity by Equal, so a copy equals its original and a locked mutex stays
// the one mutex. A type that has a Clone() T method gets it called for a field, for the elements of
// a slice, array or map and for what a pointer points at, also when the type holds no pointers.
// The twin (testdata/derive_twin/derive_pointers.go) has the methods written by hand.

package main

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

#Derive(Clone, Equal)
type Shared struct {
	Loc  *time.Location
	Buf  *bytes.Buffer
	Mu   *sync.Mutex
	When time.Time
	Name string
}

// ID has a Clone method of its own, though it holds only a number.
type ID int

func (i ID) Clone() ID { return i + 1000 }

#Derive(Clone)
type Holder struct {
	One  ID
	Many []ID
	ByID map[string]ID
	Ptr  *ID
	Arr  [2]ID
	Any  Cloner
}

type Cloner interface{ Clone() Cloner }

type Tag string

func (t Tag) Clone() Cloner { return t + "!" }

func main() {
	var mu sync.Mutex
	s := Shared{Loc: time.UTC, Buf: bytes.NewBufferString("abc"), Mu: &mu, When: time.Unix(5, 0), Name: "s"}
	mu.Lock()
	c := s.Clone()
	fmt.Println(c.Loc == s.Loc, c.Buf == s.Buf, c.Mu == s.Mu, s.Equal(c), c.Equal(s))
	c.Buf.WriteString("def")
	fmt.Println(s.Buf.String(), mu.TryLock())
	mu.Unlock()
	d := c
	d.Loc = time.FixedZone("x", 0)
	fmt.Println(s.Equal(d), d.Equal(s))

	n := ID(7)
	h := Holder{One: 1, Many: []ID{2, 3}, ByID: map[string]ID{"k": 4}, Ptr: &n, Arr: [2]ID{5, 6}, Any: Tag("t")}
	k := h.Clone()
	fmt.Println(k.One, k.Many, k.ByID, *k.Ptr, k.Arr, k.Any)
	fmt.Println(h.One, h.Many, h.ByID, *h.Ptr, h.Arr, h.Any, n)
	var zero Holder
	z := zero.Clone()
	fmt.Println(z.One, z.Many == nil, z.ByID == nil, z.Ptr == nil, z.Arr, z.Any == nil)
}

// Output:
// true true true true true
// abcdef false
// false false
// 1001 [1002 1003] map[k:1004] 1007 [1005 1006] t!
// 1 [2 3] map[k:4] 7 [5 6] t 7
// 1000 true true true [1000 1000] true
