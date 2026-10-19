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
	defer func() { fmt.Println("deferred acc", acc) }()
L1:
for x2 := range it1(3) {
	fmt.Println("body", 2, x2)
	continue L1
	fmt.Println("e4", x2)
	for i6 := 0; i6 < 1; i6++ {
		fmt.Println("body", 6, i6)
		for x9 := range it1(2) {
			fmt.Println("body", 9, x9)
			break
			fmt.Println("e11", x2, i6, x9)
		}
		for range it1(3) {
			fmt.Println("body", 13)
			for acc = range it1(2) {
				fmt.Println("body", 16)
				fmt.Println("e18", x2, i6)
			}
			for x20 := range it1(2) {
				fmt.Println("body", 20, x20)
				if x20 > 2 {
					continue
				}
			}
		}
		fmt.Println("e22", x2, i6)
	}
}
fmt.Println("e23")
	return acc
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
for acc = range it1(2) {
	fmt.Println("body", 25)
	if acc%2 == 1 {
		fmt.Println("e27")
		res = acc + 85
		return
	}
}
fmt.Println("e28")
	res = acc
	return
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
return acc + 22
L29:
for n30 := range 3 {
	fmt.Println("body", 30, n30)
	for n33 := range 3 {
		fmt.Println("body", 33, n33)
		fmt.Println("e35", n30, n33)
	}
	continue L29
	if n30%3 == 0 {
		return acc + 16
	}
}
	return acc
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
fmt.Println("e36")
return acc + 45
if acc%2 == 1 {
	fmt.Println("e37")
	goto top
}
end:
	return acc
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
for range it1(3) {
	fmt.Println("body", 39)
	fmt.Println("e41")
L42:
	switch acc % 3 {
case 0:
		fmt.Println("e43")
case 1:
		for i45 := 0; i45 < 3; i45++ {
			fmt.Println("body", 45, i45)
			acc += i45 % 5
			break L42
		}
		if acc%3 == 0 {
			fmt.Println("e48")
		} else {
			fmt.Println("e50")
		}
default:
		return acc + 76
		if acc%2 == 1 {
			fmt.Println("e51")
			return acc + 72
		}
	}
}
if acc%3 == 2 {
	fmt.Println("e52")
}
L53:
switch acc % 3 {
case 0:
	{
		var fs []func() int
L54:
		for n55 := range 3 {
			fmt.Println("body", 55, n55)
			fs = append(fs, func() int { return n55 })
			defer fmt.Println("defer", 57, acc)
			acc += n55 % 5
			switch n55 % 3 {
case 0:
				if n55%2 == 0 {
					continue L54
				}
case 1:
				if n55%3 == 2 {
					goto end
				}
				break L53
default:
				fmt.Println("e59", n55)
				acc++
			}
		}
		for _, f := range fs {
			fmt.Println("kept", f())
		}
	}
case 1:
	fmt.Println("e60")
}
end:
	return acc
}

func f5() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e61")
for range it1(2) {
	fmt.Println("body", 63)
	if acc%3 == 2 {
		fmt.Println("e65")
		break
	}
	for x67 := range it1(1) {
		fmt.Println("body", 67, x67)
		acc++
		fmt.Println("e69", x67)
L70:
		for i71 := 0; i71 < 2; i71++ {
			fmt.Println("body", 71, i71)
			for range it0(3) {
				fmt.Println("body", 74)
				if acc == 0 {
					continue L70
				}
			}
		}
	}
}
fmt.Println("e76")
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
// body 2 0
// body 2 1
// body 2 2
// it1 done 3
// e23
// deferred acc 0
// f0 = 0
// body 25
// body 25
// e27
// it1 stopped at 1
// it1 done 2
// f1 = 86
// deferred acc 0
// f2 = 22
// e36
// f3 = 45
// body 39
// e41
// e43
// body 39
// e41
// e43
// body 39
// e41
// e43
// it1 done 3
// body 55 0
// body 55 1
// defer 57 0
// defer 57 0
// deferred acc 1
// f4 = 1
// e61
// body 63
// body 67 0
// e69 0
// body 71 0
// body 74
// body 74
// body 74
// it0 done 3
// body 71 1
// body 74
// body 74
// body 74
// it0 done 3
// it1 done 1
// body 63
// body 67 0
// e69 0
// body 71 0
// body 74
// body 74
// body 74
// it0 done 3
// body 71 1
// body 74
// body 74
// body 74
// it0 done 3
// it1 done 1
// it1 done 2
// e76
// f5 = 2
