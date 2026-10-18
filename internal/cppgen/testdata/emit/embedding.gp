// forwarders: all

package main

import "fmt"

type Base struct{ ID int }

func (b Base) Get() int                    { return b.ID }
func (b *Base) Touch()                     { b.ID++ }
func (b Base) Pair(k int) (int, string)    { return b.ID + k, "p" }
func (b *Base) Sum(xs ...int) (total int) { for _, x := range xs { total += x }; return total + b.ID }

// Wide takes a parameter larger than 128 KiB: a const reference, in the forwarders too.
func (b Base) Wide(w [20000]int) int { return w[0] + b.ID }

type Base2 struct{ Base Base }

type Mid struct {
	*Base
	M int
}

type Top struct {
	Mid
	z int
}

type Holder struct{ Base }

type Mid2 struct{ *Holder }

type Deep struct{ Mid2 }

type User struct {
	Base
	Name string
}

type Both struct {
	Mid
	User
	Own int
}

func (b Both) Get() int { return -1 }

type Celsius float64

func (c Celsius) F() float64 { return float64(c)*9/5 + 32 }

type Temp struct {
	Celsius
	unit string
}

type Node struct {
	*Node
	v int
}

func (n *Node) Val() int { return n.v }

func main() {
	u := &User{Name: "ann"}
	u.Touch()
	m := Mid{&Base{5}, 1}
	m.Touch()
	fmt.Println(u.ID, u.Get(), m.ID, m.Get(), m.Sum(1, 2), u.Base.Get())
	var b Both
	b.User.Touch()
	b.Mid.Base = &Base{}
	b.Mid.Touch()
	fmt.Println(b.Get(), b.User.Get(), b.Mid.Get(), b.Own)
	t := Temp{100, "C"}
	fmt.Println(t.F(), Base2{Base{3}}.Base.Get())
	n := &Node{&Node{nil, 2}, 1}
	fmt.Println(n.Val(), n.Node.Val())
	var wide [20000]int
	wide[0] = 4
	fmt.Println(u.Wide(wide), m.Wide(wide))
	k, s := u.Pair(2)
	fmt.Println(k, s)
	top := Top{Mid{&Base{9}, 1}, 2}
	top.Touch() // an embedded pointer one level down, then the value's address
	fmt.Println(top.ID, top.Get(), top.Sum(1))
	deep := Deep{Mid2{&Holder{Base{20}}}}
	deep.Touch() // a pointer in the middle of the path, then a value whose address is taken
	fmt.Println(deep.ID, deep.Get())
}
