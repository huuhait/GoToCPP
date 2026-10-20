// phase: 08

// Escape analysis (step 15.2, ADR-114): locals and allocations whose address never escapes stay on the C++ stack
// (gp::stack_ptr, gp::local_ptr): pointer methods, borrowed parameters, a pointer variable that
// holds the address, new(T) and &T{...}, in loops and in closures of their own. Shadow-dependent
// initializers are an exception: staging them must use movable owning pointers, not local_ptr.
// The output is what real Go prints. The run is also ASan-clean.
package main

import "fmt"

type Counter struct {
	n    int
	hist [4]int
}

func (c *Counter) Inc()      { c.n++ }
func (c *Counter) Add(k int) { c.n += k; c.hist[k%4]++ }
func (c Counter) Total() int { return c.n + c.hist[0] + c.hist[1] + c.hist[2] + c.hist[3] }
func bump(c *Counter)        { c.n += 10 }
func setInt(p *int, v int)   { *p = v }
func addPair(a, b *Counter)  { a.n += 1; b.n += 100 }
func swapN(a, b *Counter)    { a.n, b.n = b.n, a.n }

type Wrapper struct {
	Counter
	name string
}

type Stack struct{ items []int }

func (s *Stack) Push(v int) { s.items = append(s.items, v) }
func (s *Stack) Len() int   { return len(s.items) }

type IntList []int

func (l *IntList) Add(v int) { *l = append(*l, v) }

type Count int

func (c *Count) Inc() { *c++ }

func rec(n int, c *Counter) {
	if n == 0 {
		return
	}
	c.Inc()
	rec(n-1, c)
}

func rename(p *string, old string) int {
	*p = "new"
	return len(old)
}

// C++ must evaluate the initializer using the outer p, before declaring the inner p.
func stagedShadow() int {
	p := &Counter{n: 2}
	{
		p := &Counter{n: p.n + 1}
		p.Inc()
		return p.n
	}
}

func stagedVar() int {
	p := &Counter{n: 2}
	{
		var p = &Counter{n: p.n + 1}
		p.Inc()
		return p.n
	}
}

// Multiple values already stay owning; the old p must be read before either new declaration.
func stagedMultiple() int {
	p := &Counter{n: 2}
	{
		p, n := &Counter{n: p.n + 1}, p.n
		p.Inc()
		return p.n + n
	}
}

func main() {
	var c Counter
	c.Inc()
	c.Add(3)
	bump(&c)
	setInt(&c.n, c.n+1)
	setInt(&c.hist[2], 7)
	d := c // a copy
	c.Inc()
	fmt.Println(c.n, c.hist, d.n, d.hist, c.Total(), d.Total())

	var w Wrapper
	w.Inc()
	w.Counter.Add(2)
	bump(&w.Counter)
	fmt.Println(w.n, w.hist, w.name == "")

	p := &c
	p.Inc()
	p.n += 5
	fmt.Println(c.n, p.n, p.Total())
	*p = Counter{n: 100}
	fmt.Println(c.n, c.hist)
	q := &c
	fmt.Println(p == q)

	r := &Counter{n: 5}
	s := new(Counter)
	r.Inc()
	s.Add(1)
	bump(r)
	addPair(r, s)
	fmt.Println(r.n, s.n, r == s, r.Total()+s.Total())

	total := 0
	for i := 0; i < 4; i++ {
		var lc Counter
		lc.Add(i)
		t := &Counter{n: i}
		t.Inc()
		total += lc.n*10 + t.n + lc.hist[i%4]
	}
	fmt.Println(total)

	var x, y Counter
	x.n, y.n = 1, 2
	swapN(&x, &y)
	addPair(&x, &x)
	fmt.Println(x.n, y.n)

	var st Stack
	st.Push(1)
	st.Push(2)
	var il IntList
	il.Add(5)
	il.Add(6)
	var cnt Count
	cnt.Inc()
	cnt.Inc()
	fmt.Println(st.Len(), st.items, il, cnt)

	var rc Counter
	rec(5, &rc)
	fmt.Println(rc.n)

	name := "old"
	fmt.Println(rename(&name, name), name)

	n := 0
	setInt(&n, 9)
	fmt.Println(n)

	fmt.Println(func() int {
		var inner Counter
		inner.Add(2)
		pp := &inner
		pp.Inc()
		return inner.n
	}())

	fmt.Println(Counter{n: 3}.Total(), (&Counter{n: 4}).Total())
	fmt.Println(stagedShadow(), stagedVar(), stagedMultiple())
}

// Output:
// 16 [0 0 7 1] 15 [0 0 7 1] 24 23
// 13 [0 0 1 0] true
// 22 22 30
// 100 [0 0 0 0]
// true
// 17 101 false 119
// 74
// 103 1
// 2 [1 2] [5 6] 2
// 5
// 3 new
// 9
// 3
// 3 4
// 4 4 6
