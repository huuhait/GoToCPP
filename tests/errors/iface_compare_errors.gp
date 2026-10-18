// phase: 05

// Impossible and invalid comparisons that involve interfaces (step 5.6).
package main

type Shape interface{ Area() int }

type Sq struct{ s int }

func (q Sq) Area() int { return q.s }

type Other struct{ n int }

type HasSlice struct{ s []int }

type Fn struct{ f func() }

func main() {
	var s Shape = Sq{1}
	var a any = 1
	_ = s == Other{1}      // ERROR "invalid operation: s == Other\{…\} \(mismatched types Shape and Other\)"
	_ = a == []int{1}      // ERROR "invalid operation: a == \[\]int\{…\} \(slice can only be compared to nil\)"
	_ = a == HasSlice{}    // ERROR "invalid operation: a == HasSlice\{\} \(struct containing \[\]int cannot be compared\)"
	_ = a == Fn{}          // ERROR "invalid operation: a == Fn\{\} \(struct containing func\(\) cannot be compared\)"
	_ = a == map[int]int{} // ERROR "invalid operation: a == map\[int\]int\{\} \(map can only be compared to nil\)"
	_ = s < s              // ERROR "invalid operation: s < s \(operator < not defined on interface\)"
	_ = a + a              // ERROR "invalid operation: operator \+ not defined on a \(variable of interface type any\)"
	_ = s == 1             // ERROR "cannot convert 1 \(untyped int constant\) to type interface\{Area\(\) int\}"
	var m map[[]int]int    // ERROR "invalid map key type \[\]int"
	var n map[HasSlice]int // ERROR "invalid map key type HasSlice"
	var o map[Fn]int       // ERROR "invalid map key type Fn"
	_, _, _ = m, n, o
	var p map[any]int
	_ = p[[]int{1}]
	_ = p[nil]
	_ = p[s]
	switch a {
	case []int{1}: // ERROR "invalid case \[\]int\{…\} in switch on a \(slice can only be compared to nil\)"
	case 1, 1: // ERROR "duplicate case 1 \(constant of type int\) in expression switch"
	}
}
