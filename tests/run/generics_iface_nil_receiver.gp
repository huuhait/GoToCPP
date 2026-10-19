// phase: 07

// Pointer type arguments with nil values: pointer-receiver methods run with a nil receiver, value methods panic.
package main

import "fmt"

type Node struct {
	val  int
	next *Node
}

func (n *Node) Len() int {
	if n == nil {
		return 0
	}
	return 1 + n.next.Len()
}

func (n *Node) Val() int { return n.val }
func (n Node) Copy() int { return n.val }

type Lener interface{ Len() int }
type Valuer interface{ Val() int }
type Copier interface{ Copy() int }

func TotalLen[T Lener](xs ...T) (t int) {
	for _, x := range xs {
		t += x.Len()
	}
	return
}

func IsNilPtr[T any](x T) bool {
	p, ok := any(x).(*Node)
	return ok && p == nil
}

func Call[T Valuer](x T) (r int, err any) {
	defer func() { err = recover() }()
	return x.Val(), nil
}

// CallCopy reports only whether the call panicked: the text of the panic of a value method called
// through a nil pointer differs between gocpp and gc.
func CallCopy[T Copier](x T) (r int, panicked bool) {
	defer func() { panicked = recover() != nil }()
	return x.Copy(), false
}

func main() {
	list := &Node{1, &Node{2, &Node{3, nil}}}
	var nilNode *Node
	fmt.Println(TotalLen(list, nilNode, list.next), TotalLen[*Node](), TotalLen[Lener](nilNode, list))
	fmt.Println(IsNilPtr(nilNode), IsNilPtr(list), IsNilPtr[any](nilNode), IsNilPtr[Lener](nilNode), IsNilPtr(5))
	fmt.Println(Call(list))
	fmt.Println(Call(nilNode))
	fmt.Println(CallCopy(list))
	fmt.Println(CallCopy(nilNode))
	fmt.Println(CallCopy(Node{val: 9}))
}

// Output:
// 5 0 3
// true false true true false
// 1 <nil>
// 0 runtime error: invalid memory address or nil pointer dereference
// 1 false
// 0 true
// 9 false
