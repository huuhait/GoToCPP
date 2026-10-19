// phase: 08

// %T of instances whose type arguments are aliases, defined types of other kinds and types of the
// standard library; the alias is transparent (the printed name is the aliased type's).
package main

import (
	"errors"
	"fmt"
	"time"
)

type Box[T any] struct{ V T }

type IntAlias = int
type StrBox = Box[string]
type Ptr[T any] = *Box[T]
type Slice[T any] = []Box[T]

type MyInt int
type MyStr string
type MyMap map[string]MyInt
type MyFunc func(MyInt) MyStr

func (m MyInt) String() string { return fmt.Sprintf("MyInt(%d)", int(m)) }

func main() {
	fmt.Printf("%T\n", Box[IntAlias]{})
	fmt.Printf("%T\n", StrBox{})
	fmt.Printf("%T\n", Box[StrBox]{})
	fmt.Printf("%T\n", Ptr[int](nil))
	fmt.Printf("%T\n", Slice[MyStr]{})
	fmt.Printf("%T\n", Box[MyMap]{})
	fmt.Printf("%T\n", Box[MyFunc]{})
	fmt.Printf("%T\n", Box[time.Duration]{})
	fmt.Printf("%T\n", Box[time.Month]{})
	fmt.Printf("%T\n", Box[error]{})
	fmt.Printf("%T\n", Box[*time.Time]{})
	fmt.Printf("%T\n", Box[[]error]{})
	fmt.Printf("%T\n", Box[Box[time.Duration]]{})
	fmt.Println(Box[MyInt]{3}, Box[[]MyInt]{[]MyInt{1, 2}}, Box[map[MyInt]MyStr]{map[MyInt]MyStr{4: "four"}})
	fmt.Printf("%v %+v %#v\n", Box[MyInt]{5}, Box[MyInt]{5}, Box[MyInt]{5})
	fmt.Printf("%v %+v %#v\n", Box[time.Duration]{1500 * time.Millisecond}, Box[time.Duration]{time.Second}, Box[time.Duration]{time.Minute})
	err := errors.New("e1")
	fmt.Printf("%v %+v\n", Box[error]{err}, Box[error]{err})
	fmt.Printf("%d %s %x\n", Box[MyInt]{10}, Box[MyStr]{"s"}, Box[MyStr]{"hex"})
}

// Output:
// main.Box[int]
// main.Box[string]
// main.Box[main.Box[string]]
// *main.Box[int]
// []main.Box[main.MyStr]
// main.Box[main.MyMap]
// main.Box[main.MyFunc]
// main.Box[time.Duration]
// main.Box[time.Month]
// main.Box[error]
// main.Box[*time.Time]
// main.Box[[]error]
// main.Box[main.Box[time.Duration]]
// {MyInt(3)} {[MyInt(1) MyInt(2)]} {map[MyInt(4):four]}
// {MyInt(5)} {V:MyInt(5)} main.Box[main.MyInt]{V:5}
// {1.5s} {V:1s} main.Box[time.Duration]{V:60000000000}
// {e1} {V:e1}
// {10} {s} {686578}
