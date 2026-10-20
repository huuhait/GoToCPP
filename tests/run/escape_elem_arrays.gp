// phase: 08

// Element references (step 15.2, ADR-114): a string, a name of a string type, an interface or a
// pointer is read out of a slice that is a view of a local array, and the code after it writes the
// array (an element, an element of an array inside an array or a struct, the whole array, through
// copy or through append). The variable keeps the value it was made with, so it is a copy. The
// program imports nothing and starts no thread, so only the analysis of the region keeps them
// copies. The last functions only read: their variables are references (see -gp:m).
package main

type Name string

type Wrap struct{ arr [2][2]string }

type Sq struct{ side int }

func make1(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		s += "x"
	}
	return s
}

func wipe(a [3]string) [3]string { return [3]string{"w", "w", "w"} }

func indexStore() string {
	arr := [3]string{make1(1), make1(2), make1(3)}
	xs := arr[:]
	r := ""
	{
		v := xs[1]
		arr[1] = "Q"
		r = v
	}
	return r
}

func rangeStore() string {
	arr := [3]string{make1(1), make1(2), make1(3)}
	xs := arr[:]
	r := ""
	for _, v := range xs {
		arr[2] = "Z"
		r += v
	}
	return r
}

func nestedArray() string {
	var w Wrap
	w.arr[0][1] = make1(2)
	xs := w.arr[0][:]
	r := ""
	{
		v := xs[1]
		w.arr[0][1] = "Q"
		r = v
	}
	return r
}

func nestedArrayWhole() string {
	var w Wrap
	w.arr[1][0] = make1(3)
	xs := w.arr[1][:]
	r := ""
	{
		v := xs[0]
		w.arr[1] = [2]string{"a", "b"}
		r = v
	}
	return r
}

func structAssign() string {
	var w Wrap
	w.arr[0][0] = make1(2)
	xs := w.arr[0][:]
	r := ""
	{
		v := xs[0]
		w = Wrap{}
		r = v
	}
	return r
}

func wholeArray() string {
	arr := [3]string{make1(1), make1(2), make1(3)}
	xs := arr[:]
	r := ""
	{
		v := xs[0]
		arr = wipe(arr)
		r = v
	}
	return r
}

func namedString() Name {
	arr := [2]Name{Name(make1(2)), "b"}
	xs := arr[:]
	var r Name
	{
		v := xs[0]
		arr[0] = "Q"
		r = v
	}
	return r
}

func viaCopy() string {
	arr := [3]string{make1(1), make1(2), make1(3)}
	xs := arr[:]
	other := []string{"c0", "c1", "c2"}
	r := ""
	{
		v := xs[1]
		copy(xs, other)
		r = v
	}
	return r
}

func viaAppend() string {
	arr := [3]string{make1(1), make1(2), make1(3)}
	xs := arr[:]
	r := ""
	{
		v := xs[1]
		ys := append(xs[:1], "A")
		r = v + ys[1]
	}
	return r
}

func interfaceArray() int {
	arr := [2]any{&Sq{3}, &Sq{4}}
	xs := arr[:]
	r := 0
	{
		v := xs[0]
		arr[0] = &Sq{9}
		r = v.(*Sq).side
	}
	return r
}

func pointerArray() int {
	arr := [2]*Sq{{3}, {4}}
	xs := arr[:]
	r := 0
	for _, v := range xs {
		arr[0] = nil
		r += v.side
	}
	return r
}

func sliceArray() int {
	arr := [2][]int{{1, 2}, {3}}
	xs := arr[:]
	r := 0
	{
		v := xs[0]
		arr[0] = nil
		r = len(v)
	}
	return r
}

func mapArray() int {
	arr := [1]map[int]int{{1: 2}}
	xs := arr[:]
	r := 0
	{
		v := xs[0]
		arr[0] = nil
		r = len(v)
	}
	return r
}

func channelArray() bool {
	arr := [1]chan int{make(chan int)}
	xs := arr[:]
	r := false
	{
		v := xs[0]
		arr[0] = nil
		r = v != nil
	}
	return r
}

func functionArray() bool {
	arr := [1]func(){func() {}}
	xs := arr[:]
	r := false
	{
		v := xs[0]
		arr[0] = nil
		r = v != nil
	}
	return r
}

func stringRecordArray() string {
	type Record struct{ name string }
	arr := [1]Record{{make1(2)}}
	xs := arr[:]
	r := ""
	{
		v := xs[0]
		arr[0].name = "Q"
		r = v.name
	}
	return r
}

func viaPointerToArray() string {
	arr := [3]string{make1(1), make1(2), make1(3)}
	xs := arr[:]
	p := &arr
	r := ""
	{
		v := xs[1]
		p[1] = "Q"
		r = v
	}
	return r
}

func viaPointerToStruct() string {
	var w Wrap
	w.arr[0][1] = make1(2)
	pw := &w
	xs := w.arr[0][:]
	r := ""
	{
		v := xs[1]
		pw.arr[0][1] = "Q"
		r = v
	}
	return r
}

func viaClear() string {
	arr := [3]string{make1(1), make1(2), make1(3)}
	xs := arr[:]
	r := ""
	{
		v := xs[2]
		clear(xs)
		r = v
	}
	return r
}

func viaPlusAssign() string {
	arr := [2]string{make1(1), make1(2)}
	xs := arr[:]
	r := ""
	{
		v := xs[1]
		arr[1] += "!"
		r = v + "/" + arr[1]
	}
	return r
}

func viaRangeAssign() string {
	arr := [2]string{make1(1), make1(2)}
	xs := arr[:]
	r := ""
	{
		v := xs[0]
		for i := range arr {
			arr[i] = "z"
		}
		r = v
	}
	return r
}

func readOnly() (int, int) {
	arr := [3]string{make1(1), make1(2), make1(3)}
	xs := arr[:]
	n := 0
	for _, s := range xs {
		n += len(s)
	}
	m := 0
	for i := range xs {
		v := xs[i]
		m += len(v)
	}
	return n, m
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
	expectStr("indexStore", indexStore(), "xx")
	expectStr("rangeStore", rangeStore(), "xxxZ")
	expectStr("nestedArray", nestedArray(), "xx")
	expectStr("nestedArrayWhole", nestedArrayWhole(), "xxx")
	expectStr("structAssign", structAssign(), "xx")
	expectStr("wholeArray", wholeArray(), "x")
	expectStr("namedString", string(namedString()), "xx")
	expectStr("viaCopy", viaCopy(), "xx")
	expectStr("viaAppend", viaAppend(), "xxA")
	expectInt("interfaceArray", interfaceArray(), 3)
	expectInt("pointerArray", pointerArray(), 7)
	expectInt("sliceArray", sliceArray(), 2)
	expectInt("mapArray", mapArray(), 1)
	if !channelArray() || !functionArray() {
		panic("channel or function slot changed")
	}
	expectStr("stringRecordArray", stringRecordArray(), "xx")
	expectStr("viaPointerToArray", viaPointerToArray(), "xx")
	expectStr("viaPointerToStruct", viaPointerToStruct(), "xx")
	expectStr("viaClear", viaClear(), "xxx")
	expectStr("viaPlusAssign", viaPlusAssign(), "xx/xx!")
	expectStr("viaRangeAssign", viaRangeAssign(), "x")
	n, m := readOnly()
	expectInt("readOnly n", n, 6)
	expectInt("readOnly m", m, 6)
}

// Output:
