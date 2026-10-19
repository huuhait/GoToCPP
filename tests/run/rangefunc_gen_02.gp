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

func f0() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e1")
fmt.Println("e2")
for n4 := range 1 {
	fmt.Println("body", 4, n4)
	fmt.Println("e6", n4)
	acc++
}
fmt.Println("e7")
	res = acc
	return
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e8")
if acc > 1 {
	fmt.Println("e9")
}
{
	var fs []func() int
L10:
	for x11 := range it1(3) {
		fmt.Println("body", 11, x11)
		fs = append(fs, func() int { return x11 })
		if x11 > 3 {
			fmt.Println("e13", x11)
			break L10
		}
		acc += x11 % 5
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
top:
	steps++
	if steps > 3 {
		goto end
	}
{
	var fs []func() int
L14:
	for x15 := range it1(3) {
		fmt.Println("body", 15, x15)
		{
			var fs []func() int
			for x18 := range it1(3) {
				fmt.Println("body", 18, x18)
				fs = append(fs, func() int { return x18 })
				fmt.Println("e20", x15, x18)
				fmt.Println("e21", x15, x18)
			}
			for _, f := range fs {
				fmt.Println("kept", f())
			}
		}
		{
			var fs []func() int
			for x23, s23 := range it2(3) {
				fmt.Println("body", 23, x23)
				fmt.Println("body", 23, s23)
				fs = append(fs, func() int { return x23 })
				acc++
			}
			for _, f := range fs {
				fmt.Println("kept", f())
			}
		}
		if x15%3 == 2 {
			continue L14
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
for acc = range it1(1) {
	fmt.Println("body", 26)
	fmt.Println("e28")
	return acc + 25
	for x30 := range it1(1) {
		fmt.Println("body", 30, x30)
		defer fmt.Println("defer", 32, acc)
		if acc%2 == 1 {
			goto end
		}
	}
}
switch acc % 3 {
case 0:
	for acc = range it1(2) {
		fmt.Println("body", 35)
		if acc%3 == 1 {
			fmt.Println("e38")
		} else {
			if acc%2 == 0 {
				return acc + 72
			}
		}
		fmt.Println("e40")
		continue
	}
case 1:
	switch acc % 3 {
case 0:
		for range it0(1) {
			fmt.Println("body", 43)
			for acc = range it1(1) {
				fmt.Println("body", 46)
				if acc > 2 {
					break
				}
			}
		}
		if acc%3 == 0 {
			for range it0(2) {
				fmt.Println("body", 50)
				if acc > 0 {
					fmt.Println("e53")
				} else {
					if acc%2 == 1 {
						break
					}
				}
			}
		} else {
			for x57 := range it1(2) {
				fmt.Println("body", 57, x57)
				if x57%2 == 1 {
					return acc + 70
				}
			}
		}
case 1:
		fmt.Println("e59")
default:
		fmt.Println("e60")
	}
	for range it0(1) {
		fmt.Println("body", 62)
		for acc = range it1(2) {
			fmt.Println("body", 65)
			goto b66
			fmt.Println("e67")
			fmt.Println("e68")
b66:
		}
		defer fmt.Println("defer", 69, acc)
		switch acc % 3 {
case 0:
			for x72, s72 := range it2(3) {
				fmt.Println("body", 72, x72)
				fmt.Println("body", 72, s72)
				if acc%2 == 0 {
					goto top
				}
			}
			fmt.Println("e74")
case 1:
			if acc == 0 {
				goto top
			}
default:
			fmt.Println("e75")
			goto top
		}
	}
default:
	acc++
}
defer fmt.Println("defer", 76, acc)
end:
	res = acc
	return
}

func f3() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e77")
if acc > 2 {
	fmt.Println("e78")
}
for x80 := range it1(3) {
	fmt.Println("body", 80, x80)
L82:
	for x83 := range it1(3) {
		fmt.Println("body", 83, x83)
		continue L82
	}
	fmt.Println("e85", x80)
}
	res = acc
	return
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e86")
if acc > 2 {
	fmt.Println("e87")
	return acc + 97
}
fmt.Println("e88")
	return acc
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
if acc > 0 {
	fmt.Println("e89")
}
for n91 := range 3 {
	fmt.Println("body", 91, n91)
	fmt.Println("e93", n91)
}
if acc%3 == 2 {
	fmt.Println("e94")
	fmt.Println("e95")
}
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
// e1
// e2
// body 4 0
// e6 0
// e7
// f0 = 1
// e8
// body 11 0
// body 11 1
// body 11 2
// it1 done 3
// kept 0
// kept 1
// kept 2
// f1 = 3
// body 15 0
// body 18 0
// e20 0 0
// e21 0 0
// body 18 1
// e20 0 1
// e21 0 1
// body 18 2
// e20 0 2
// e21 0 2
// it1 done 3
// kept 0
// kept 1
// kept 2
// body 23 0
// body 23 s0
// body 23 10
// body 23 s1
// body 23 20
// body 23 s2
// it2 done 3
// kept 0
// kept 10
// kept 20
// body 15 1
// body 18 0
// e20 1 0
// e21 1 0
// body 18 1
// e20 1 1
// e21 1 1
// body 18 2
// e20 1 2
// e21 1 2
// it1 done 3
// kept 0
// kept 1
// kept 2
// body 23 0
// body 23 s0
// body 23 10
// body 23 s1
// body 23 20
// body 23 s2
// it2 done 3
// kept 0
// kept 10
// kept 20
// body 15 2
// body 18 0
// e20 2 0
// e21 2 0
// body 18 1
// e20 2 1
// e21 2 1
// body 18 2
// e20 2 2
// e21 2 2
// it1 done 3
// kept 0
// kept 1
// kept 2
// body 23 0
// body 23 s0
// body 23 10
// body 23 s1
// body 23 20
// body 23 s2
// it2 done 3
// kept 0
// kept 10
// kept 20
// it1 done 3
// body 26
// e28
// it1 stopped at 0
// it1 done 1
// f2 = 25
// e77
// body 80 0
// body 83 0
// body 83 1
// body 83 2
// it1 done 3
// e85 0
// body 80 1
// body 83 0
// body 83 1
// body 83 2
// it1 done 3
// e85 1
// body 80 2
// body 83 0
// body 83 1
// body 83 2
// it1 done 3
// e85 2
// it1 done 3
// f3 = 0
// e86
// e88
// f4 = 0
// body 91 0
// e93 0
// body 91 1
// e93 1
// body 91 2
// e93 2
// f5 = 0
