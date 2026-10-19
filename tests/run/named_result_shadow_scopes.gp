// phase: 07

// The shapes of the bug report: a function with a named result and a defer, and a return of a
// variable of an inner scope with the result's name: a block, a for clause, a range, the init
// statement of if and switch, a type switch binding. Each function has a deferred closure that
// reads or changes the named result (so it is boxed), and some recover.
package main

import "fmt"

func block() (i int) {
	defer func() { i++ }()
	{
		i := 7
		return i
	}
}

func forClause() (i int) {
	defer func() { i *= 2 }()
	for i := 0; ; i++ {
		if i == 3 {
			return i
		}
	}
}

func rangeVal() (i int) {
	defer func() { i += 100 }()
	for _, i := range []int{1, 4, 9} {
		if i == 4 {
			return i
		}
	}
	return -1
}

func ifInit() (i int) {
	defer func() { fmt.Println("deferred sees", i) }()
	if i := 9; i > 0 {
		return i
	}
	return 1
}

func switchInit() (i int) {
	defer func() { i++ }()
	switch i := 3; i {
	case 3:
		return i
	}
	return 0
}

func switchTag(x int) (i int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered %v", r)
		}
	}()
	switch i := x * 2; {
	case i > 10:
		panic(i)
	case i > 4:
		return i, nil
	}
	return -1, nil
}

func typeSwitch(x any) (i int) {
	defer func() { i += 1000 }()
	switch i := x.(type) {
	case int:
		return i
	case string:
		return len(i)
	}
	return -1
}

func recovering() (i int, err error) {
	defer func() {
		if r := recover(); r != nil {
			i, err = i+1, fmt.Errorf("%v", r)
		}
	}()
	for i := 5; i > 0; i-- {
		if i == 3 {
			return i, nil
		}
	}
	return 0, nil
}

func recoveringPanic(n int) (i int, err error) {
	defer func() {
		if r := recover(); r != nil {
			i, err = i+1, fmt.Errorf("%v", r)
		}
	}()
	i = 40
	for i := 5; i > 0; i-- {
		if i == n {
			var a []int
			return a[i], nil
		}
	}
	return 0, nil
}

func Generic[T any](xs []T, pick int) (i int, v T) {
	defer func() { i++ }()
	for i, v := range xs {
		if i == pick {
			return i * 10, v
		}
	}
	return -1, v
}

func main() {
	fmt.Println(block())
	fmt.Println(forClause())
	fmt.Println(rangeVal())
	fmt.Println(ifInit())
	fmt.Println(switchInit())
	fmt.Println(switchTag(3))
	fmt.Println(switchTag(1))
	fmt.Println(switchTag(7))
	fmt.Println(typeSwitch(5), typeSwitch("abc"), typeSwitch(2.5))
	fmt.Println(recovering())
	fmt.Println(recoveringPanic(3))
	fmt.Println(recoveringPanic(9))
	fmt.Println(Generic([]string{"a", "b", "c"}, 1))
	fmt.Println(Generic([]float64{1.5}, 4))
}

// Output:
// 8
// 6
// 104
// deferred sees 9
// 9
// 4
// 6 <nil>
// -1 <nil>
// 0 recovered 14
// 1005 1003 999
// 3 <nil>
// 41 runtime error: index out of range [3] with length 0
// 0 <nil>
// 11 b
// 0 0
