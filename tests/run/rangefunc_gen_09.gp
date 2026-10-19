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
fmt.Println("e1")
fmt.Println("e2")
fmt.Println("e3")
fmt.Println("e4")
	return acc
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
fmt.Println("e5")
if acc == 3 {
	res = acc + 83
	return
}
L6:
switch acc % 3 {
case 0:
L7:
	for x8 := range it1(1) {
		fmt.Println("body", 8, x8)
		if x8 == 3 {
			break L6
		}
		if x8%2 == 1 {
			break L6
		}
		for range it0(1) {
			fmt.Println("body", 11)
			if x8%3 == 0 {
				if acc == 1 {
					continue L7
				}
			} else {
				acc++
			}
			fmt.Println("e15", x8)
		}
	}
	if acc > 1 {
		for x18 := range it1(1) {
			fmt.Println("body", 18, x18)
			break
			fmt.Println("e20", x18)
			defer fmt.Println("defer", 21, acc)
		}
		fmt.Println("e22")
		if acc > 1 {
			break L6
		}
	} else {
		res = acc + 37
		return
	}
case 1:
	for n25 := range 1 {
		fmt.Println("body", 25, n25)
		for range it1(3) {
			fmt.Println("body", 28)
			acc += n25 % 5
			switch n25 % 3 {
case 0:
				if n25 > 3 {
					goto end
				}
				if n25%3 == 0 {
					goto top
				}
case 1:
				if n25%3 == 0 {
					goto b29
				}
				if acc%2 == 0 {
					fmt.Println("e31", n25)
					res = acc + 44
					return
				}
			}
b29:
		}
	}
}
end:
	res = acc
	return
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
return acc + 83
fmt.Println("e32")
if acc == 3 {
	fmt.Println("e33")
	fmt.Println("e34")
}
acc++
	return acc
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
if acc == 3 {
	fmt.Println("e35")
	return acc + 31
}
if acc > 1 {
	if acc%3 == 2 {
		goto end
	}
L37:
	for x38 := range it1(2) {
		fmt.Println("body", 38, x38)
		for range it1(3) {
			fmt.Println("body", 41)
			for x44 := range it1(3) {
				fmt.Println("body", 44, x44)
				fmt.Println("e46", x38, x44)
			}
			if x38%3 == 2 {
				goto end
			}
		}
		for x48 := range it1(2) {
			fmt.Println("body", 48, x48)
			acc++
			for x51 := range it1(2) {
				fmt.Println("body", 51, x51)
				fmt.Println("e53", x38, x48, x51)
			}
		}
		break L37
	}
	{
		var fs []func() int
		for n55 := range 2 {
			fmt.Println("body", 55, n55)
			fs = append(fs, func() int { return n55 })
L57:
			for range it1(1) {
				fmt.Println("body", 58)
				fmt.Println("e60", n55)
				break L57
			}
			if n55%3 == 2 {
				goto b56
			}
			fmt.Println("e61", n55)
b56:
		}
		for _, f := range fs {
			fmt.Println("kept", f())
		}
	}
} else {
	fmt.Println("e63")
}
end:
	return acc
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e64")
if acc > 3 {
	fmt.Println("e65")
	return acc + 33
}
acc++
return acc + 4
	res = acc
	return
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e66")
if acc%3 == 1 {
	goto end
}
return acc + 41
fmt.Println("e67")
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
// e3
// e4
// deferred acc 0
// f0 = 0
// e5
// body 8 0
// body 11
// e15 0
// it0 done 1
// it1 done 1
// f1 = 37
// f2 = 83
// e63
// f3 = 0
// e64
// f4 = 5
// e66
// deferred acc 0
// f5 = 41
