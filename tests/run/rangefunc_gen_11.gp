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
if acc%3 == 1 {
	fmt.Println("e3")
}
	res = acc
	return
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
if acc%2 == 0 {
	fmt.Println("e4")
}
for x6 := range it1(3) {
	fmt.Println("body", 6, x6)
	switch x6 % 3 {
case 0:
		fmt.Println("e9", x6)
case 1:
		if x6 > 2 {
			fmt.Println("e11")
		} else {
			acc++
		}
default:
		acc++
	}
	if x6%2 == 1 {
		fmt.Println("e13", x6)
		continue
	}
}
fmt.Println("e14")
if acc > 1 {
	fmt.Println("e15")
	fmt.Println("e16")
}
	res = acc
	return
}

func f2() (res int) {
	acc, steps := 0, 0
	_ = steps
if acc%3 == 1 {
	fmt.Println("e17")
}
fmt.Println("e18")
	res = acc
	return
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
acc++
fmt.Println("e19")
if acc == 0 {
	fmt.Println("e20")
}
for acc = range it1(3) {
	fmt.Println("body", 22)
	for range it0(2) {
		fmt.Println("body", 25)
		if acc == 1 {
			fmt.Println("e28")
		} else {
			fmt.Println("e30")
		}
	}
}
	return acc
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
{
	var fs []func() int
	for x32 := range it1(3) {
		fmt.Println("body", 32, x32)
		fs = append(fs, func() int { return x32 })
		if x32%3 == 1 {
			break
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
for x35, s35 := range it2(3) {
	fmt.Println("body", 35, x35)
	fmt.Println("body", 35, s35)
	res = acc + 85
	return
}
	res = acc
	return
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e37")
for x39 := range it1(2) {
	fmt.Println("body", 39, x39)
	fmt.Println("e41", x39)
L42:
	for x43 := range it1(3) {
		fmt.Println("body", 43, x43)
L45:
		for x46 := range it1(2) {
			fmt.Println("body", 46, x46)
			fmt.Println("e48", x39, x43, x46)
			if acc == 0 {
				fmt.Println("e49", x39, x43, x46)
				break L45
			}
		}
		switch x43 % 3 {
case 0:
			if x43%2 == 1 {
				continue L42
			}
case 1:
			for x52 := range it1(1) {
				fmt.Println("body", 52, x52)
				if x52%2 == 1 {
					goto end
				}
			}
			if acc%2 == 0 {
				continue L42
			}
default:
			if acc%2 == 0 {
				fmt.Println("e54", x39, x43)
			}
		}
	}
	continue
}
fmt.Println("e55")
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
// e1
// e2
// f0 = 0
// e4
// body 6 0
// e9 0
// body 6 1
// e13 1
// body 6 2
// it1 done 3
// e14
// e15
// e16
// f1 = 2
// e18
// f2 = 0
// e19
// body 22
// body 25
// e30
// body 25
// e30
// it0 done 2
// body 22
// body 25
// e28
// body 25
// e28
// it0 done 2
// body 22
// body 25
// e30
// body 25
// e30
// it0 done 2
// it1 done 3
// deferred acc 2
// f3 = 2
// body 32 0
// body 32 1
// it1 stopped at 1
// it1 done 3
// kept 0
// kept 1
// body 35 0
// body 35 s0
// it2 stopped at 0
// it2 done 3
// deferred acc 0
// f4 = 85
// e37
// body 39 0
// e41 0
// body 43 0
// body 46 0
// e48 0 0 0
// e49 0 0 0
// it1 stopped at 0
// it1 done 2
// body 43 1
// body 46 0
// e48 0 1 0
// e49 0 1 0
// it1 stopped at 0
// it1 done 2
// body 52 0
// it1 done 1
// body 43 2
// body 46 0
// e48 0 2 0
// e49 0 2 0
// it1 stopped at 0
// it1 done 2
// e54 0 2
// it1 done 3
// body 39 1
// e41 1
// body 43 0
// body 46 0
// e48 1 0 0
// e49 1 0 0
// it1 stopped at 0
// it1 done 2
// body 43 1
// body 46 0
// e48 1 1 0
// e49 1 1 0
// it1 stopped at 0
// it1 done 2
// body 52 0
// it1 done 1
// body 43 2
// body 46 0
// e48 1 2 0
// e49 1 2 0
// it1 stopped at 0
// it1 done 2
// e54 1 2
// it1 done 3
// it1 done 2
// e55
// deferred acc 0
// f5 = 0
