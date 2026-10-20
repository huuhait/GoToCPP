// phase: 08

// Element references (step 15.2, ADR-114): the same hazards as escape_elem_threaded_hazards in a
// program that has no second thread (it imports nothing and prints nothing), so the region of the
// variable is the only thing that keeps it a copy. Each function reads the element after code
// that could overwrite the slot, drop the slice or free the array; the checks in main panic when
// the variable saw a changed element. The last functions only read: their variables are
// references (see -gp:m).
package main

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
		r.names = append(r.names, "n"+string(rune('0'+v%10)))
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
	return s + "!"
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

func expectInt(what string, got, want int) {
	if got != want {
		panic(what)
	}
}

func expectStr(what, got, want string) {
	if got != want {
		panic(what)
	}
}

func main() {
	expectInt("viaWipe", viaWipe(newReg(1, 2)), 1)
	expectInt("viaDrop", viaDrop(newReg(2, 3)), 2)
	expectInt("viaFieldStore", viaFieldStore(newReg(3, 4)), 3)
	expectInt("viaLocalSlice", viaLocalSlice(), 31)
	expectInt("viaGlobalReset", viaGlobalReset(), 32)
	expectInt("viaGlobalElem", viaGlobalElem(), 33)
	expectInt("viaArrayAlias", viaArrayAlias(), 34)
	expectInt("viaArrayAssign", viaArrayAssign(), 35)
	expectInt("viaElementMethod", viaElementMethod(newReg(4, 5)), 4)
	expectInt("viaElementDrop", viaElementDrop(newReg(5, 6)), 5)
	expectInt("rangeWipe", rangeWipe(newReg(6, 7)), 13)
	expectInt("rangeDrop", rangeDrop(newReg(7, 8)), 15)
	expectInt("rangeAppend", rangeAppend(newReg(8, 9)), 21)
	expectStr("stringRenamed", stringRenamed(newReg(1, 2)), "n1/renamed")
	expectStr("stringDropped", stringDropped(newReg(1, 2)), "n1!")
	expectStr("stringAppended", stringAppended([]string{"a", "b"}), "a/a-more")
	expectStr("rangeString", rangeString([]string{"p", "q"}), "pq/x")
	sum, n := readOnly(newReg(1, 2, 3))
	expectInt("readOnly sum", sum, 6)
	expectInt("readOnly n", n, 6)
	expectInt("readStrings", readStrings(newReg(1, 22, 3)), 8)
}

// Output:
