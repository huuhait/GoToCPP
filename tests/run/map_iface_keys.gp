// phase: 05

// map[Iface]V and map[error]V (step 5.6): value and pointer keys, equal values are one key,
// distinct pointers are two, the nil interface is a key, typed nil is another, errors as keys.
package main

import "fmt"

type Shape interface{ Area() int }

type Sq struct{ s int }

func (q Sq) Area() int { return q.s }

type Rc struct{ w, h int }

func (r *Rc) Area() int { return r.w * r.h }

type Tag string

func (t Tag) Area() int { return len(t) }

type Code int

func (c Code) Error() string { return fmt.Sprint("code ", int(c)) }

type Err struct{ msg string }

func (e *Err) Error() string { return e.msg }

func main() {
	m := map[Shape]string{}
	r1, r2 := &Rc{1, 2}, &Rc{1, 2}
	m[Sq{1}] = "sq1"
	m[Sq{1}] += "+"
	m[Sq{2}] = "sq2"
	m[r1] = "r1"
	m[r2] = "r2"
	m[Tag("ab")] = "tag"
	m[nil] = "nil"
	var np *Rc
	m[np] = "typed nil"
	m[Shape(nil)] = "nil again"
	fmt.Println(len(m), m[Sq{1}], m[Sq{2}], m[r1], m[r2], m[Tag("ab")], m[Tag("ba")] == "", m[nil], m[np])
	delete(m, Sq{1})
	delete(m, r1)
	delete(m, nil)
	_, ok := m[nil]
	fmt.Println(len(m), ok, m[np])
	v, ok := m[&Rc{1, 2}]
	fmt.Println(v == "", ok)

	em := map[error]int{}
	e1, e2 := &Err{"x"}, &Err{"x"}
	em[e1]++
	em[e2]++
	em[e1]++
	em[Code(1)] += 10
	em[Code(1)] += 10
	em[Code(2)] = 5
	em[nil] = -1
	fmt.Println(len(em), em[e1], em[e2], em[Code(1)], em[Code(2)], em[nil], em[Code(3)])
	var asErr error = Code(2)
	fmt.Println(em[asErr])

	// Interface-typed keys of different interface types holding the same dynamic type.
	var s Shape = Sq{9}
	am := map[any]int{s: 1}
	fmt.Println(am[Sq{9}], am[s], am[Shape(Sq{9})], am[Tag("x")])

	// Iterate and check the value read through the key.
	sum := 0
	for k, v := range map[Shape]int{Sq{1}: 1, Sq{2}: 2, Tag("abc"): 3} {
		sum += k.Area() * v
	}
	fmt.Println(sum)
}

// Output:
// 7 sq1+ sq2 r1 r2 tag true nil again typed nil
// 4 false typed nil
// true false
// 5 2 1 20 5 -1 0
// 5
// 1 1 1 0
// 14
