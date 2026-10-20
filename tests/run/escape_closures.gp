// phase: 08

// Local closures (step 15.2, ADR-114): a func literal that is only called where it is made, directly or through
// a variable that nothing else uses, captures by reference: its variables stay on the C++ stack and
// the call is a direct call of the lambda. Everything else must still share the variable with the
// closure and keep the closure alive: returned, stored, passed, run by go or defer, used by another
// closure, assigned, compared or address-taken. The output is what real Go prints; the run is also
// ASan-clean.
package main

import (
	"fmt"
	"sync"
)

type Box struct {
	fn func() int
}

var saved func() int

func apply(f func(int) int, n, x int) int {
	for i := 0; i < n; i++ {
		x = f(x)
	}
	return x
}

func makeAdder(k int) func(int) int {
	return func(x int) int { return x + k }
}

func counterLoop(n int) int {
	count := 0
	counter := func() int {
		count++
		return count
	}
	sum := 0
	for i := 0; i < n; i++ {
		sum += counter()
	}
	return sum*1000 + count
}

func immediate() int {
	total := 0
	func() {
		total += 5
	}()
	x := func() int { return total * 2 }()
	return total*100 + x
}

func seesLaterChanges() int {
	v := 1
	read := func() int { return v }
	a := read()
	v = 10
	b := read()
	v += 5
	return a*10000 + b*100 + read()
}

func evaluationOrder() int {
	count := 0
	bump := func() int {
		count++
		return count
	}
	x := count + bump()
	y := bump() + count
	return x*100 + y
}

func nested() int {
	n := 0
	outer := func() {
		inner := func() { n += 2 }
		inner()
		inner()
		n++
	}
	outer()
	outer()
	return n
}

func stringsAndSlices() string {
	s := "a"
	xs := []int{1}
	var st struct{ a, b int }
	add := func(t string, k int) {
		s += t
		xs = append(xs, k)
		st.a += k
		st.b++
	}
	add("b", 2)
	add("c", 3)
	return fmt.Sprint(s, xs, st)
}

func perIteration() int {
	sum := 0
	for i := 0; i < 4; i++ {
		acc := i
		step := func() { acc += 10 }
		step()
		step()
		sum += acc
	}
	return sum
}

func loopVariable() int {
	sum := 0
	for i := 0; i < 4; i++ {
		get := func() int { return i * i }
		sum += get()
	}
	return sum
}

func sharedWithEscaping() int {
	n := 0
	local := func() { n++ }
	escaping := func() int { n += 10; return n }
	local()
	saved = escaping
	local()
	return n*100 + saved()
}

func returned() int {
	k := 7
	add := makeAdder(k)
	k = 100 // the closure has its own k
	f := func(x int) int { return x * 3 }
	return add(1) + apply(f, 3, 1) + apply(add, 2, 0)
}

func stored() int {
	n := 0
	inc := func() int { n++; return n }
	b := Box{fn: inc}
	fs := []func() int{inc}
	m := map[string]func() int{"x": inc}
	return b.fn() + fs[0]() + m["x"]() + n
}

func passedToGo() int {
	n := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	add := func() {
		mu.Lock()
		n++
		mu.Unlock()
		wg.Done()
	}
	wg.Add(2)
	go add()
	go func() { add() }()
	wg.Wait()
	return n
}

func deferred() (r int) {
	n := 1
	double := func() { n *= 2 }
	defer double()
	defer func() { r = n * 10 }()
	n = 5
	return n
}

func calledInClosure() int {
	n := 0
	inc := func() { n++ }
	g := func() { inc(); inc() }
	g()
	return n
}

func compared() int {
	n := 0
	f := func() { n++ }
	if f != nil {
		f()
	}
	return n
}

func reassigned() int {
	n := 0
	f := func() { n++ }
	f()
	f = func() { n += 100 }
	f()
	return n
}

func addressTaken() int {
	n := 0
	f := func() { n++ }
	p := &f
	(*p)()
	return n
}

func loopVariableChanged() int {
	sum := 0
	for i := 0; i < 10; i++ {
		skip := func() { i++ }
		skip()
		sum += i
	}
	return sum
}

func rangeVariable() int {
	sum := 0
	for i, v := range []int{3, 4, 5} {
		get := func() int { return i*10 + v }
		sum += get()
		func() { v *= 2 }()
		sum += v
	}
	return sum
}

func loopVariableEscapes() int {
	var fs []func() int
	sum := 0
	for i := 0; i < 3; i++ {
		get := func() int { return i }
		sum += get()
		fs = append(fs, func() int { i += 10; return i })
	}
	for _, f := range fs {
		sum += f() + f()
	}
	return sum
}

func loopVariableGo() int {
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	for i := 1; i <= 4; i++ {
		get := func() int { return i * i }
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			total += get()
			mu.Unlock()
		}()
	}
	wg.Wait()
	return total
}

func nestedLoopVariables() int {
	sum := 0
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			f := func() int { return i*3 + j }
			sum += f()
		}
	}
	return sum
}

func main() {
	fmt.Println(counterLoop(10))
	fmt.Println(immediate())
	fmt.Println(seesLaterChanges())
	fmt.Println(evaluationOrder())
	fmt.Println(nested())
	fmt.Println(stringsAndSlices())
	fmt.Println(perIteration())
	fmt.Println(loopVariable())
	fmt.Println(sharedWithEscaping())
	fmt.Println(returned())
	fmt.Println(stored())
	fmt.Println(passedToGo())
	fmt.Println(deferred())
	fmt.Println(calledInClosure())
	fmt.Println(compared())
	fmt.Println(reassigned())
	fmt.Println(addressTaken())
	fmt.Println(loopVariableChanged())
	fmt.Println(rangeVariable())
	fmt.Println(loopVariableEscapes())
	fmt.Println(loopVariableGo())
	fmt.Println(nestedLoopVariables())
}

// Output:
// 55010
// 510
// 11015
// 204
// 10
// abc[1 2 3] {5 2}
// 86
// 14
// 1212
// 49
// 9
// 2
// 50
// 2
// 1
// 101
// 1
// 25
// 66
// 99
// 30
// 36
