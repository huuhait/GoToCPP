// Pointers to types of other packages and field types with a Clone method of their own (step 10.10).
// A pointer to a type of another package (*time.Location, *bytes.Buffer, *sync.Mutex) is shared by
// Clone and compared by identity by Equal, so a copy equals its original and a locked mutex stays
// the one mutex. A type that has a Clone() T method gets it called for a field, for the elements of
// a slice, array or map and for what a pointer points at, also when the type holds no pointers.
//

package main

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

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

func (s Shared) Equal(o Shared) bool {
	return s.Loc == o.Loc && s.Buf == o.Buf && s.Mu == o.Mu && s.When.Equal(o.When) && s.Name == o.Name
}

func (s Shared) Clone() Shared { return s }

func (h Holder) Clone() Holder {
	out := h
	out.One = h.One.Clone()
	if h.Many != nil {
		out.Many = make([]ID, len(h.Many))
		for i := range h.Many {
			out.Many[i] = h.Many[i].Clone()
		}
	}
	if h.ByID != nil {
		out.ByID = make(map[string]ID, len(h.ByID))
		for k, v := range h.ByID {
			out.ByID[k] = v.Clone()
		}
	}
	if h.Ptr != nil {
		c := h.Ptr.Clone()
		out.Ptr = &c
	}
	for i := range h.Arr {
		out.Arr[i] = h.Arr[i].Clone()
	}
	if h.Any != nil {
		out.Any = h.Any.Clone()
	}
	return out
}
