// phase: 07

package main

import "fmt"

// it1 yields 0..n-1; the body may stop it.
func it1(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println("it1 done", n)
		for i := 0; i < n; i++ {
			if !yield(i) {
				fmt.Println("it1 stopped at", i)
				return
			}
		}
	}
}

// it2 yields an int and a string.
func it2(n int) func(yield func(int, string) bool) {
	return func(yield func(int, string) bool) {
		defer fmt.Println("it2 done", n)
		for i := 0; i < n; i++ {
			if !yield(i*10, fmt.Sprint("s", i)) {
				fmt.Println("it2 stopped at", i)
				return
			}
		}
	}
}

// it0 yields nothing but calls the body n times.
func it0(n int) func(yield func() bool) {
	return func(yield func() bool) {
		defer fmt.Println("it0 done", n)
		for i := 0; i < n; i++ {
			if !yield() {
				fmt.Println("it0 stopped at", i)
				return
			}
		}
	}
}

func f0() int {
	acc, steps := 0, 0
	_ = steps
if acc%2 == 1 {
	return acc + 72
}
if acc == 1 {
	fmt.Println("e1")
	fmt.Println("e2")
}
for x4, s4 := range it2(3) {
	fmt.Println("body", 4, x4)
	fmt.Println("body", 4, s4)
	fmt.Println("e6", x4)
	if x4%3 == 1 {
		fmt.Println("e7", x4)
		goto end
	}
}
end:
	return acc
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e8")
L9:
for range it0(1) {
	fmt.Println("body", 10)
	fmt.Println("e12")
	return acc + 59
	for range it0(2) {
		fmt.Println("body", 14)
		fmt.Println("e16")
		for x18 := range it1(2) {
			fmt.Println("body", 18, x18)
			fmt.Println("e20", x18)
			for acc = range it1(3) {
				fmt.Println("body", 22)
				if x18 > 1 {
					break L9
				}
			}
		}
	}
}
fmt.Println("e24")
fmt.Println("e25")
	return acc
}

func f2() (res int) {
	acc, steps := 0, 0
	_ = steps
if acc == 3 {
	for x28 := range it1(1) {
		fmt.Println("body", 28, x28)
		if x28 > 1 {
			break
		} else {
			for x33 := range it1(3) {
				fmt.Println("body", 33, x33)
				defer fmt.Println("defer", 35, acc)
			}
		}
	}
} else {
	goto b36
	fmt.Println("e37")
b36:
}
if acc%2 == 1 {
	goto end
}
end:
	res = acc
	return
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
if acc > 3 {
	fmt.Println("e38")
	return acc + 45
}
for x40 := range it1(1) {
	fmt.Println("body", 40, x40)
L42:
	for acc = range it1(2) {
		fmt.Println("body", 43)
		for acc = range it1(2) {
			fmt.Println("body", 46)
			if acc == 0 {
				fmt.Println("e48", x40)
				break L42
			}
			fmt.Println("e49", x40)
		}
	}
}
	return acc
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
if acc > 3 {
	goto end
}
fmt.Println("e50")
end:
	return acc
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
for x52 := range it1(3) {
	fmt.Println("body", 52, x52)
	fmt.Println("e54", x52)
	defer fmt.Println("defer", 55, acc)
}
for n57 := range 3 {
	fmt.Println("body", 57, n57)
	defer fmt.Println("defer", 59, acc)
	fmt.Println("e60", n57)
L61:
	switch n57 % 3 {
case 0:
		if acc > 2 {
			goto end
		}
case 1:
		if n57%2 == 0 {
			break L61
		}
default:
		for range it1(2) {
			fmt.Println("body", 63)
			defer fmt.Println("defer", 65, acc)
		}
	}
}
end:
	return acc
}


func run(name string, f func() int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(name, "recovered", r)
		}
	}()
	fmt.Println(name, "=", f())
}

func main() {
	run("f0", f0)
	run("f1", f1)
	run("f2", f2)
	run("f3", f3)
	run("f4", f4)
	run("f5", f5)
}

// Output:
// body 4 0
// body 4 s0
// e6 0
// body 4 10
// body 4 s1
// e6 10
// e7 10
// it2 stopped at 1
// it2 done 3
// f0 = 0
// e8
// body 10
// e12
// it0 stopped at 0
// it0 done 1
// deferred acc 0
// f1 = 59
// f2 = 0
// body 40 0
// body 43
// body 46
// e48 0
// it1 stopped at 0
// it1 done 2
// it1 stopped at 0
// it1 done 2
// it1 done 1
// deferred acc 0
// f3 = 0
// e50
// deferred acc 0
// f4 = 0
// body 52 0
// e54 0
// body 52 1
// e54 1
// body 52 2
// e54 2
// it1 done 3
// body 57 0
// e60 0
// body 57 1
// e60 1
// body 57 2
// e60 2
// body 63
// body 63
// it1 done 2
// defer 65 0
// defer 65 0
// defer 59 0
// defer 59 0
// defer 59 0
// defer 55 0
// defer 55 0
// defer 55 0
// f5 = 0
