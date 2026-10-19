// phase: 05

// A named result hidden by a variable of an inner scope: a return with values in a function with a
// defer stores into the result, not into the inner variable of the same name. Loop variables
// (for, range, init statements of if and switch), nested blocks, labelled blocks, type
// switch bindings, and a func literal's parameter named like the result.
package main

import "fmt"

func rangeIdx(xs []int, v int) (i int, p any) {
	defer func() { p = recover() }()
	for i, x := range xs {
		if x == v {
			return i, nil
		}
	}
	return -1, nil
}

func forVar(n int) (i int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered: %v", r)
		}
	}()
	for i := 0; i < 10; i++ {
		if i == n {
			return i * 10, nil
		}
	}
	return 0, fmt.Errorf("none")
}

func ifInit(m map[string]int, k string) (v int, ok bool) {
	defer func() { v++ }()
	if v, ok := m[k]; ok {
		return v * 2, ok
	}
	return 100, false
}

func switchInit(n int) (s string, k int) {
	defer func() { k += 1000 }()
	switch k := n * 2; {
	case k > 10:
		return "big", k
	case k > 4:
		return "mid", k
	}
	return "small", n
}

func nested() (a int, b string) {
	defer func() {}()
	{
		a := 5
		{
			b := "inner"
			if a > 3 {
				return a, b
			}
		}
	}
	return -1, "none"
}

func labelled(n int) (i int) {
	defer func() { i += 1 }()
outer:
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if i*3+j == n {
				break outer
			}
			if i == 2 && j == 1 {
				return i*100 + j
			}
		}
	}
	return 7
}

func typeSw(x any) (i int, s string) {
	defer func() { s = s + "!" }()
	switch i := x.(type) {
	case int:
		return i + 1, "int"
	case string:
		return len(i), i
	}
	return -1, "other"
}

func closureParam() (i int, s string) {
	defer func() {}()
	f := func(i int, s string) (int, string) { return i * 2, s + s }
	i, s = f(4, "ab")
	return i + 1, s
}

func closureShadow() (i int) {
	defer func() {}()
	f := func() int {
		i := 40
		return i + 2
	}
	return f()
}

func main() {
	fmt.Println(rangeIdx([]int{5, 6, 7}, 7))
	fmt.Println(rangeIdx([]int{5, 6, 7}, 9))
	fmt.Println(forVar(3))
	fmt.Println(forVar(30))
	fmt.Println(ifInit(map[string]int{"a": 4}, "a"))
	fmt.Println(ifInit(map[string]int{"a": 4}, "b"))
	fmt.Println(switchInit(7))
	fmt.Println(switchInit(3))
	fmt.Println(switchInit(1))
	fmt.Println(nested())
	fmt.Println(labelled(4), labelled(100))
	fmt.Println(typeSw(41))
	fmt.Println(typeSw("hello"))
	fmt.Println(typeSw(1.5))
	fmt.Println(closureParam())
	fmt.Println(closureShadow())
}

// Output:
// 2 <nil>
// -1 <nil>
// 30 <nil>
// 0 none
// 9 true
// 101 false
// big 1014
// mid 1006
// small 1001
// 5 inner
// 8 202
// 42 int!
// 5 hello!
// -1 other!
// 9 abab
// 42
