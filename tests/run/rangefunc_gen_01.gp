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
fmt.Println("e1")
defer fmt.Println("defer", 2, acc)
defer fmt.Println("defer", 3, acc)
fmt.Println("e4")
	return acc
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
top:
	steps++
	if steps > 3 {
		goto end
	}
for x6 := range it1(1) {
	fmt.Println("body", 6, x6)
L8:
	for x9 := range it1(2) {
		fmt.Println("body", 9, x9)
		for acc = range it1(2) {
			fmt.Println("body", 12)
			acc++
			fmt.Println("e14", x6, x9)
		}
		break L8
	}
	fmt.Println("e15", x6)
}
if acc == 1 {
	goto top
}
end:
	res = acc
	return
}

func f2() (res int) {
	acc, steps := 0, 0
	_ = steps
if acc > 0 {
	fmt.Println("e16")
	fmt.Println("e17")
}
fmt.Println("e18")
fmt.Println("e19")
	res = acc
	return
}

func f3() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
for x21 := range it1(3) {
	fmt.Println("body", 21, x21)
	fmt.Println("e23", x21)
}
fmt.Println("e24")
fmt.Println("e25")
	res = acc
	return
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
{
	var fs []func() int
	for x27, s27 := range it2(2) {
		fmt.Println("body", 27, x27)
		fmt.Println("body", 27, s27)
		fs = append(fs, func() int { return x27 })
		break
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
goto top
end:
	res = acc
	return
}

func f5() (res int) {
	acc, steps := 0, 0
	_ = steps
if acc%3 == 0 {
	{
		var fs []func() int
		for n31 := range 1 {
			fmt.Println("body", 31, n31)
L33:
			for x34, s34 := range it2(2) {
				fmt.Println("body", 34, x34)
				fmt.Println("body", 34, s34)
				for x37, s37 := range it2(3) {
					fmt.Println("body", 37, x37)
					fmt.Println("body", 37, s37)
					fmt.Println("e39", n31, x34, x37)
				}
				if n31 > 2 {
					continue L33
				}
				for x41 := range it1(2) {
					fmt.Println("body", 41, x41)
					if x34%3 == 1 {
						fmt.Println("e43", n31, x34, x41)
					}
				}
			}
			for x45 := range it1(3) {
				fmt.Println("body", 45, x45)
				res = acc + 82
				return
			}
			{
				var fs []func() int
				for x48 := range it1(1) {
					fmt.Println("body", 48, x48)
					fs = append(fs, func() int { return x48 })
					if n31%2 == 0 {
						fmt.Println("e50", n31, x48)
						goto end
					}
				}
				for _, f := range fs {
					fmt.Println("kept", f())
				}
			}
		}
		for _, f := range fs {
			fmt.Println("kept", f())
		}
	}
	defer fmt.Println("defer", 51, acc)
	{
		var fs []func() int
L52:
		for acc = range it1(1) {
			fmt.Println("body", 53)
			{
				var fs []func() int
L55:
				for range it0(1) {
					fmt.Println("body", 56)
					if acc%2 == 0 {
						fmt.Println("e58")
						goto b57
					}
					fmt.Println("e59")
					{
						var fs []func() int
						for x61 := range it1(1) {
							fmt.Println("body", 61, x61)
							fs = append(fs, func() int { return x61 })
							break L55
						}
						for _, f := range fs {
							fmt.Println("kept", f())
						}
					}
b57:
				}
				for _, f := range fs {
					fmt.Println("kept", f())
				}
			}
			if acc == 3 {
				continue L52
			}
			for n64 := range 2 {
				fmt.Println("body", 64, n64)
				if n64 > 1 {
					return acc + 75
				} else {
					fmt.Println("e68")
				}
				if n64 > 2 {
					break
				}
			}
		}
		for _, f := range fs {
			fmt.Println("kept", f())
		}
	}
} else {
	if acc%2 == 0 {
		return acc + 90
	}
}
fmt.Println("e70")
end:
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
// e1
// e4
// defer 3 0
// defer 2 0
// f0 = 0
// body 6 0
// body 9 0
// body 12
// e14 0 0
// body 12
// e14 0 0
// it1 done 2
// it1 stopped at 0
// it1 done 2
// e15 0
// it1 done 1
// deferred acc 2
// f1 = 2
// e18
// e19
// f2 = 0
// body 21 0
// e23 0
// body 21 1
// e23 1
// body 21 2
// e23 2
// it1 done 3
// e24
// e25
// deferred acc 0
// f3 = 0
// body 27 0
// body 27 s0
// it2 stopped at 0
// it2 done 2
// kept 0
// body 27 0
// body 27 s0
// it2 stopped at 0
// it2 done 2
// kept 0
// body 27 0
// body 27 s0
// it2 stopped at 0
// it2 done 2
// kept 0
// f4 = 0
// body 31 0
// body 34 0
// body 34 s0
// body 37 0
// body 37 s0
// e39 0 0 0
// body 37 10
// body 37 s1
// e39 0 0 10
// body 37 20
// body 37 s2
// e39 0 0 20
// it2 done 3
// body 41 0
// body 41 1
// it1 done 2
// body 34 10
// body 34 s1
// body 37 0
// body 37 s0
// e39 0 10 0
// body 37 10
// body 37 s1
// e39 0 10 10
// body 37 20
// body 37 s2
// e39 0 10 20
// it2 done 3
// body 41 0
// e43 0 10 0
// body 41 1
// e43 0 10 1
// it1 done 2
// it2 done 2
// body 45 0
// it1 stopped at 0
// it1 done 3
// f5 = 82
