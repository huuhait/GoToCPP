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
L1:
for range it1(2) {
	fmt.Println("body", 2)
	defer fmt.Println("defer", 4, acc)
	for x6 := range it1(2) {
		fmt.Println("body", 6, x6)
		if x6 > 0 {
			continue L1
		}
	}
}
fmt.Println("e8")
L9:
switch acc % 3 {
case 0:
	if acc%2 == 0 {
		fmt.Println("e10")
		goto end
	}
	fmt.Println("e11")
case 1:
	if acc > 2 {
		break L9
	}
default:
	if acc%2 == 0 {
		fmt.Println("e12")
	}
}
fmt.Println("e13")
end:
	return acc
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e14")
if acc == 3 {
	fmt.Println("e15")
	fmt.Println("e16")
}
{
	var fs []func() int
L17:
	for x18, s18 := range it2(1) {
		fmt.Println("body", 18, x18)
		fmt.Println("body", 18, s18)
		fs = append(fs, func() int { return x18 })
		fmt.Println("e20", x18)
		for x22 := range it1(3) {
			fmt.Println("body", 22, x22)
			if x22%3 == 0 {
				for x26, s26 := range it2(1) {
					fmt.Println("body", 26, x26)
					fmt.Println("body", 26, s26)
					fmt.Println("e28", x26)
				}
			} else {
				for range it1(1) {
					fmt.Println("body", 31)
					acc++
				}
			}
			if acc%2 == 1 {
				continue L17
			}
			switch x22 % 3 {
case 0:
				if x22 > 2 {
					fmt.Println("e34", x18, x22)
					fmt.Println("e35", x18, x22)
				}
				fmt.Println("e36", x18, x22)
case 1:
				for x38 := range it1(2) {
					fmt.Println("body", 38, x38)
					fmt.Println("e40", x18, x22, x38)
				}
			}
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
	return acc
}

func f2() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e41")
for acc = range it1(2) {
	fmt.Println("body", 43)
	acc++
	switch acc % 3 {
case 0:
		fmt.Println("e46")
case 1:
		fmt.Println("e47")
	}
}
	res = acc
	return
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e48")
for x50 := range it1(2) {
	fmt.Println("body", 50, x50)
	for x53, s53 := range it2(1) {
		fmt.Println("body", 53, x53)
		fmt.Println("body", 53, s53)
		if x53%2 == 1 {
			return acc + 14
		} else {
			if acc == 1 {
				break
			}
		}
	}
	for range it0(1) {
		fmt.Println("body", 58)
		for x61, s61 := range it2(1) {
			fmt.Println("body", 61, x61)
			fmt.Println("body", 61, s61)
			defer fmt.Println("defer", 63, acc)
			fmt.Println("e64", x50, x61)
		}
	}
	if acc == 2 {
		continue
	}
}
	return acc
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
for x66 := range it1(3) {
	fmt.Println("body", 66, x66)
	fmt.Println("e68", x66)
	if x66%3 == 0 {
		return acc + 76
	}
	if x66 == 0 {
		goto end
	}
}
for x70 := range it1(3) {
	fmt.Println("body", 70, x70)
	fmt.Println("e72", x70)
	acc++
}
end:
	return acc
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
{
	var fs []func() int
	for n74 := range 3 {
		fmt.Println("body", 74, n74)
		fs = append(fs, func() int { return n74 })
		for x77 := range it1(1) {
			fmt.Println("body", 77, x77)
			fmt.Println("e79", n74, x77)
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
if acc%2 == 0 {
	goto end
}
if acc == 3 {
	return acc + 43
}
defer fmt.Println("defer", 80, acc)
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
// body 2
// body 6 0
// body 6 1
// it1 stopped at 1
// it1 done 2
// body 2
// body 6 0
// body 6 1
// it1 stopped at 1
// it1 done 2
// it1 done 2
// e8
// e10
// defer 4 0
// defer 4 0
// f0 = 0
// e14
// body 18 0
// body 18 s0
// e20 0
// body 22 0
// body 26 0
// body 26 s0
// e28 0
// it2 done 1
// e36 0 0
// body 22 1
// body 31
// it1 done 1
// it1 stopped at 1
// it1 done 3
// it2 done 1
// kept 0
// deferred acc 1
// f1 = 1
// e41
// body 43
// e47
// body 43
// it1 done 2
// f2 = 2
// e48
// body 50 0
// body 53 0
// body 53 s0
// it2 done 1
// body 58
// body 61 0
// body 61 s0
// e64 0 0
// it2 done 1
// it0 done 1
// body 50 1
// body 53 0
// body 53 s0
// it2 done 1
// body 58
// body 61 0
// body 61 s0
// e64 1 0
// it2 done 1
// it0 done 1
// it1 done 2
// defer 63 0
// defer 63 0
// deferred acc 0
// f3 = 0
// body 66 0
// e68 0
// it1 stopped at 0
// it1 done 3
// f4 = 76
// body 74 0
// body 77 0
// e79 0 0
// it1 done 1
// body 74 1
// body 77 0
// e79 1 0
// it1 done 1
// body 74 2
// body 77 0
// e79 2 0
// it1 done 1
// kept 0
// kept 1
// kept 2
// f5 = 0
