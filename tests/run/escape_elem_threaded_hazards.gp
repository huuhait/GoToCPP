// phase: 08

// Element references (step 15.2, ADR-114): the same hazards as escape_elem_hazards in a program that
// can run a second thread (it uses fmt): the element variable is a copy there whenever the program
// stores elements of that type anywhere (hotElems), and for the region's own reasons too.
package main

import "fmt"

type Node struct {
	val  int
	name string
}

func (n *Node) Val() int { return n.val }

type Reg struct {
	items []*Node
	names []string
}

var global []*Node

func newReg(vals ...int) *Reg {
	r := &Reg{}
	for _, v := range vals {
		r.items = append(r.items, &Node{val: v})
		r.names = append(r.names, fmt.Sprint("n", v))
	}
	return r
}

func (r *Reg) wipe()          { r.items[0] = nil }
func (r *Reg) drop()          { r.items = nil }
func (r *Reg) rename()        { r.names[0] = "renamed" }
func (r *Reg) dropNames()     { r.names = nil }
func resetGlobal()            { global = nil }
func clearGlobalElem()        { global[0] = nil }
func (n *Node) wipeIn(r *Reg) { r.items[0] = nil }
func (n *Node) dropIn(r *Reg) { r.items = nil }

// Each function reads the element after code that could free it. The result must be the value
// the element had when the variable was made.

func viaWipe(r *Reg) int {
	v := r.items[0]
	r.wipe()
	return v.val
}

func viaDrop(r *Reg) int {
	v := r.items[0]
	r.drop()
	return v.val
}

func viaFieldStore(r *Reg) int {
	v := r.items[0]
	r.items = nil
	return v.val
}

func viaLocalSlice() int {
	xs := []*Node{{val: 31}}
	v := xs[0]
	xs = nil
	return v.val
}

func viaGlobalReset() int {
	global = []*Node{{val: 32}}
	v := global[0]
	resetGlobal()
	return v.val
}

func viaGlobalElem() int {
	global = []*Node{{val: 33}}
	v := global[0]
	clearGlobalElem()
	return v.val
}

func viaArrayAlias() int {
	var arr [2]*Node
	arr[0] = &Node{val: 34}
	xs := arr[:]
	v := xs[0]
	arr[0] = nil
	return v.val
}

func viaArrayAssign() int {
	var arr [2]*Node
	arr[0] = &Node{val: 35}
	xs := arr[:]
	v := xs[0]
	arr = [2]*Node{}
	return v.val
}

func viaElementMethod(r *Reg) int {
	v := r.items[0]
	return v.wipeInThenVal(r)
}

func (n *Node) wipeInThenVal(r *Reg) int {
	n.wipeIn(r)
	return n.val
}

func viaElementDrop(r *Reg) int {
	v := r.items[0]
	v.dropIn(r)
	return v.val
}

func rangeWipe(r *Reg) int {
	sum := 0
	for _, v := range r.items {
		r.wipe()
		sum += v.val
	}
	return sum
}

func rangeDrop(r *Reg) int {
	sum := 0
	for _, v := range r.items {
		r.drop()
		sum += v.val
	}
	return sum
}

func rangeAppend(r *Reg) int {
	sum := 0
	for _, v := range r.items {
		r.items = append(r.items, &Node{val: 100})
		sum += v.val
	}
	return sum + len(r.items)
}

func stringRenamed(r *Reg) string {
	s := r.names[0]
	r.rename()
	return s + "/" + r.names[0]
}

func stringDropped(r *Reg) string {
	s := r.names[0]
	r.dropNames()
	return s
}

func stringAppended(xs []string) string {
	s := xs[0]
	xs[0] += "-more"
	return s + "/" + xs[0]
}

func rangeString(xs []string) string {
	out := ""
	for i, s := range xs {
		xs[i] = "x"
		out += s
	}
	return out + "/" + xs[0]
}

// These only read: the variables can refer to the elements.
func readOnly(r *Reg) (int, int) {
	sum, n := 0, 0
	for _, v := range r.items {
		sum += v.Val()
	}
	for i := range r.items {
		v := r.items[i]
		n += v.val
	}
	return sum, n
}

func readStrings(r *Reg) int {
	n := 0
	for _, s := range r.names {
		n += len(s)
	}
	t := r.names[1]
	return n + len(t)
}

func main() {
	fmt.Println(viaWipe(newReg(1, 2)))
	fmt.Println(viaDrop(newReg(2, 3)))
	fmt.Println(viaFieldStore(newReg(3, 4)))
	fmt.Println(viaLocalSlice())
	fmt.Println(viaGlobalReset())
	fmt.Println(viaGlobalElem())
	fmt.Println(viaArrayAlias())
	fmt.Println(viaArrayAssign())
	fmt.Println(viaElementMethod(newReg(4, 5)))
	fmt.Println(viaElementDrop(newReg(5, 6)))
	fmt.Println(rangeWipe(newReg(6, 7)))
	fmt.Println(rangeDrop(newReg(7, 8)))
	fmt.Println(rangeAppend(newReg(8, 9)))
	fmt.Println(stringRenamed(newReg(1, 2)))
	fmt.Println(stringDropped(newReg(1, 2)))
	fmt.Println(stringAppended([]string{"a", "b"}))
	fmt.Println(rangeString([]string{"p", "q"}))
	fmt.Println(readOnly(newReg(1, 2, 3)))
	fmt.Println(readStrings(newReg(1, 22, 3)))
}

// Output:
// 1
// 2
// 3
// 31
// 32
// 33
// 34
// 35
// 4
// 5
// 13
// 15
// 21
// n1/renamed
// n1
// a/a-more
// pq/x
// 6 6
// 10
