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
L1:
for range it1(1) {
	fmt.Println("body", 2)
	for i5 := 0; i5 < 3; i5++ {
		fmt.Println("body", 5, i5)
		for x8, s8 := range it2(3) {
			fmt.Println("body", 8, x8)
			fmt.Println("body", 8, s8)
			acc += x8 % 5
		}
		acc++
		if i5%2 == 1 {
			res = acc + 71
			return
		}
	}
	defer fmt.Println("defer", 10, acc)
	if acc%2 == 0 {
		fmt.Println("e11")
		continue L1
	}
}
defer fmt.Println("defer", 12, acc)
	res = acc
	return
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
{
	var fs []func() int
	for range it0(1) {
		fmt.Println("body", 14)
		{
			var fs []func() int
			for x17 := range it1(1) {
				fmt.Println("body", 17, x17)
				fs = append(fs, func() int { return x17 })
				if acc > 3 {
					fmt.Println("e19", x17)
					return acc + 26
				}
			}
			for _, f := range fs {
				fmt.Println("kept", f())
			}
		}
		for x21 := range it1(2) {
			fmt.Println("body", 21, x21)
			acc++
			fmt.Println("e23", x21)
			fmt.Println("e24", x21)
		}
L25:
		switch acc % 3 {
case 0:
			if acc == 3 {
				defer fmt.Println("defer", 27, acc)
			} else {
				acc++
			}
			fmt.Println("e29")
case 1:
			fmt.Println("e30")
default:
			fmt.Println("e31")
			switch acc % 3 {
case 0:
				fmt.Println("e33")
				break L25
case 1:
				for acc = range it1(2) {
					fmt.Println("body", 35)
					if acc%2 == 0 {
						goto end
					}
				}
L37:
				switch acc % 3 {
case 0:
					if acc%3 == 0 {
						goto end
					}
case 1:
					fmt.Println("e38")
					if acc%2 == 1 {
						break L25
					}
default:
					if acc%3 == 2 {
						break L37
					}
				}
default:
				for x40 := range it1(3) {
					fmt.Println("body", 40, x40)
					fmt.Println("e42", x40)
				}
				fmt.Println("e43")
			}
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
{
	var fs []func() int
	for x45 := range it1(2) {
		fmt.Println("body", 45, x45)
		fs = append(fs, func() int { return x45 })
		for acc = range it1(1) {
			fmt.Println("body", 48)
			fmt.Println("e50", x45)
			switch x45 % 3 {
case 0:
				fmt.Println("e52", x45)
				if x45 == 3 {
					break
				}
case 1:
				if x45%3 == 0 {
					goto top
				}
				fmt.Println("e53", x45)
default:
				switch x45 % 3 {
case 0:
					if x45%2 == 0 {
						break
					}
					acc += x45 % 5
case 1:
					acc++
					fmt.Println("e55", x45)
default:
					if x45 == 0 {
						fmt.Println("e56", x45)
						continue
					}
				}
			}
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
end:
	return acc
}

func f2() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
L57:
for acc = range it1(1) {
	fmt.Println("body", 58)
	if acc%2 == 1 {
		fmt.Println("e61")
		for n63 := range 2 {
			fmt.Println("body", 63, n63)
			if n63%3 == 0 {
				continue
			}
			for i66 := 0; i66 < 3; i66++ {
				fmt.Println("body", 66, i66)
				if n63%3 == 2 {
					return acc + 69
				}
			}
		}
		if acc == 0 {
			fmt.Println("e68")
			continue
		}
	} else {
		fmt.Println("e70")
		for i72 := 0; i72 < 3; i72++ {
			fmt.Println("body", 72, i72)
			fmt.Println("e74", i72)
			if i72%2 == 0 {
				break L57
			}
		}
		break
	}
}
fmt.Println("e75")
switch acc % 3 {
case 0:
	defer fmt.Println("defer", 77, acc)
	defer fmt.Println("defer", 78, acc)
case 1:
	if acc%2 == 1 {
		res = acc + 63
		return
	}
}
if acc == 0 {
	fmt.Println("e79")
}
	res = acc
	return
}

func f3() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e80")
L81:
for range it0(3) {
	fmt.Println("body", 82)
	break L81
}
	res = acc
	return
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e84")
{
	var fs []func() int
	for n86 := range 2 {
		fmt.Println("body", 86, n86)
		acc++
		{
			var fs []func() int
			for i89 := 0; i89 < 1; i89++ {
				fmt.Println("body", 89, i89)
				fs = append(fs, func() int { return i89 })
				acc += i89 % 5
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
	return acc
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e91")
L92:
for x93, s93 := range it2(2) {
	fmt.Println("body", 93, x93)
	fmt.Println("body", 93, s93)
	return acc + 47
	if x93 > 3 {
		fmt.Println("e96")
		for i98 := 0; i98 < 1; i98++ {
			fmt.Println("body", 98, i98)
			for i101 := 0; i101 < 2; i101++ {
				fmt.Println("body", 101, i101)
				if i98 == 0 {
					break
				}
			}
		}
		break L92
	} else {
		fmt.Println("e104")
	}
L105:
	for acc = range it1(3) {
		fmt.Println("body", 106)
		if x93%2 == 0 {
			goto end
		}
		continue L105
		break
	}
}
fmt.Println("e108")
if acc%2 == 1 {
	fmt.Println("e109")
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
// body 2
// body 5 0
// body 8 0
// body 8 s0
// body 8 10
// body 8 s1
// body 8 20
// body 8 s2
// it2 done 3
// body 5 1
// body 8 0
// body 8 s0
// body 8 10
// body 8 s1
// body 8 20
// body 8 s2
// it2 done 3
// it1 stopped at 0
// it1 done 1
// f0 = 73
// body 14
// body 17 0
// it1 done 1
// kept 0
// body 21 0
// e23 0
// e24 0
// body 21 1
// e23 1
// e24 1
// it1 done 2
// e31
// body 40 0
// e42 0
// body 40 1
// e42 1
// body 40 2
// e42 2
// it1 done 3
// e43
// it0 done 1
// body 45 0
// body 48
// e50 0
// e52 0
// it1 done 1
// body 45 1
// body 48
// e50 1
// e53 1
// it1 done 1
// it1 done 2
// kept 0
// kept 1
// f1 = 0
// body 58
// e70
// body 72 0
// e74 0
// it1 stopped at 0
// it1 done 1
// e75
// e79
// defer 78 0
// defer 77 0
// deferred acc 0
// f2 = 0
// e80
// body 82
// it0 stopped at 0
// it0 done 3
// f3 = 0
// e84
// body 86 0
// body 89 0
// kept 0
// body 86 1
// body 89 0
// kept 0
// f4 = 2
// e91
// body 93 0
// body 93 s0
// it2 stopped at 0
// it2 done 2
// f5 = 47
