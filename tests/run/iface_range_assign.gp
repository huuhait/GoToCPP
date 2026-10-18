// phase: 05

// Range loops that assign to interface variables, and conversions in assignments to existing
// variables, parallel assignment and return of several values (step 5.4).
package main

import "fmt"

type Named interface{ Name() string }
type A string
type B string

func (a A) Name() string { return "A" + string(a) }
func (b B) Name() string { return "B" + string(b) }

func pair() (A, B) { return "1", "2" }

func main() {
	var n Named
	names := ""
	for _, a := range []A{"x", "y"} {
		n = a
		names += n.Name() + " "
	}
	for _, b := range []B{"p"} {
		n = b
		names += n.Name() + " "
	}
	fmt.Println(names[:len(names)-1])
	var x any
	for _, x = range []int{1, 2, 3} {
	}
	fmt.Println(x.(int))
	var k, v any
	for k, v = range map[string]bool{"only": true} {
	}
	fmt.Println(k.(string), v.(bool))
	var na, nb Named
	na, nb = pair()
	fmt.Println(na.Name(), nb.Name())
	na, nb = nb, na
	fmt.Println(na.Name(), nb.Name())
	var p, q any = 1, "s"
	p, q = q, p
	fmt.Println(p.(string), q.(int))
	var arr [2]any
	arr[0], arr[1] = A("z"), nil
	fmt.Println(arr[0].(Named).Name(), arr[1] == nil)
}

// Output:
// Ax Ay Bp
// 3
// only true
// A1 B2
// B2 A1
// s 1
// Az true
