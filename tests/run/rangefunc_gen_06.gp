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
top:
	steps++
	if steps > 3 {
		goto end
	}
for acc = range it1(2) {
	fmt.Println("body", 2)
	if acc == 0 {
		return acc + 50
	}
	for acc = range it1(1) {
		fmt.Println("body", 5)
		if acc > 0 {
			goto top
		} else {
			for acc = range it1(1) {
				fmt.Println("body", 10)
				if acc%2 == 1 {
					fmt.Println("e12")
					goto b11
				}
b11:
			}
		}
	}
}
defer fmt.Println("defer", 13, acc)
end:
	return acc
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
for x15, s15 := range it2(2) {
	fmt.Println("body", 15, x15)
	fmt.Println("body", 15, s15)
	fmt.Println("e17", x15)
	if x15%3 == 2 {
		fmt.Println("e18", x15)
		return acc + 10
	}
}
for x20 := range it1(2) {
	fmt.Println("body", 20, x20)
	if x20 == 2 {
		fmt.Println("e22", x20)
		continue
	}
	fmt.Println("e23", x20)
}
fmt.Println("e24")
if acc%3 == 1 {
	fmt.Println("e25")
	goto end
}
end:
	return acc
}

func f2() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e26")
for x28 := range it1(3) {
	fmt.Println("body", 28, x28)
L30:
	switch x28 % 3 {
case 0:
		switch x28 % 3 {
case 0:
			fmt.Println("e32", x28)
case 1:
			break L30
		}
		defer fmt.Println("defer", 33, acc)
case 1:
		if x28%2 == 0 {
			break
		}
default:
		acc++
	}
	if acc == 3 {
		fmt.Println("e34", x28)
	}
	fmt.Println("e35", x28)
}
switch acc % 3 {
case 0:
	fmt.Println("e37")
case 1:
	if acc == 2 {
		fmt.Println("e38")
		res = acc + 29
		return
	}
default:
	if acc%2 == 0 {
		break
	}
}
	res = acc
	return
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
acc++
fmt.Println("e39")
fmt.Println("e40")
fmt.Println("e41")
	return acc
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
for acc = range it1(2) {
	fmt.Println("body", 43)
	acc++
	if acc == 3 {
		continue
	}
	fmt.Println("e45")
}
for range it1(1) {
	fmt.Println("body", 47)
	fmt.Println("e49")
	fmt.Println("e50")
}
defer fmt.Println("defer", 51, acc)
	return acc
}

func f5() (res int) {
	acc, steps := 0, 0
	_ = steps
if acc%2 == 0 {
	res = acc + 80
	return
}
switch acc % 3 {
case 0:
	for range it1(3) {
		fmt.Println("body", 54)
		fmt.Println("e56")
	}
case 1:
	for n58 := range 1 {
		fmt.Println("body", 58, n58)
		fmt.Println("e60", n58)
		if acc > 3 {
			fmt.Println("e62")
		} else {
			return acc + 43
		}
		for acc = range it1(2) {
			fmt.Println("body", 65)
			switch n58 % 3 {
case 0:
				defer fmt.Println("defer", 68, acc)
case 1:
				if n58%2 == 0 {
					res = acc + 95
					return
				}
				fmt.Println("e69", n58)
			}
			for x71 := range it1(2) {
				fmt.Println("body", 71, x71)
				if n58%3 == 1 {
					goto b72
				}
b72:
			}
			for n74 := range 3 {
				fmt.Println("body", 74, n74)
				fmt.Println("e76", n58, n74)
			}
		}
	}
	fmt.Println("e77")
}
fmt.Println("e78")
	res = acc
	return
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
// body 2
// it1 stopped at 0
// it1 done 2
// f0 = 50
// body 15 0
// body 15 s0
// e17 0
// body 15 10
// body 15 s1
// e17 10
// it2 done 2
// body 20 0
// e23 0
// body 20 1
// e23 1
// it1 done 2
// e24
// deferred acc 0
// f1 = 0
// e26
// body 28 0
// e32 0
// e35 0
// body 28 1
// e35 1
// body 28 2
// e35 2
// it1 done 3
// defer 33 0
// f2 = 1
// e39
// e40
// e41
// deferred acc 1
// f3 = 1
// body 43
// e45
// body 43
// e45
// it1 done 2
// body 47
// e49
// e50
// it1 done 1
// defer 51 2
// f4 = 2
// f5 = 80
