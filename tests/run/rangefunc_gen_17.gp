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
for acc = range it1(1) {
	fmt.Println("body", 2)
	continue
	if acc%3 == 0 {
		return acc + 74
	}
}
fmt.Println("e4")
	return acc
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e5")
if acc%2 == 1 {
	fmt.Println("e6")
}
for n8 := range 1 {
	fmt.Println("body", 8, n8)
	for n11 := range 2 {
		fmt.Println("body", 11, n11)
		fmt.Println("e13", n8, n11)
		fmt.Println("e14", n8, n11)
		fmt.Println("e15", n8, n11)
	}
	return acc + 72
	switch n8 % 3 {
case 0:
		switch n8 % 3 {
case 0:
			for x19 := range it1(2) {
				fmt.Println("body", 19, x19)
				if x19 > 1 {
					return acc + 20
				}
			}
case 1:
			for range it1(1) {
				fmt.Println("body", 22)
				fmt.Println("e24", n8)
			}
default:
			if n8 > 0 {
				if acc > 0 {
					continue
				}
			} else {
				if acc%3 == 2 {
					goto end
				}
			}
		}
case 1:
		if n8 == 1 {
			return acc + 29
		}
		fmt.Println("e27", n8)
	}
}
fmt.Println("e28")
end:
	return acc
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
if acc > 1 {
	fmt.Println("e29")
}
if acc == 0 {
	fmt.Println("e30")
}
switch acc % 3 {
case 0:
	fmt.Println("e32")
case 1:
	{
		var fs []func() int
		for x34, s34 := range it2(3) {
			fmt.Println("body", 34, x34)
			fmt.Println("body", 34, s34)
			fs = append(fs, func() int { return x34 })
			for acc = range it1(2) {
				fmt.Println("body", 37)
				for range it0(3) {
					fmt.Println("body", 40)
					fmt.Println("e42", x34)
				}
				fmt.Println("e43", x34)
			}
		}
		for _, f := range fs {
			fmt.Println("kept", f())
		}
	}
}
goto top
end:
	return acc
}

func f3() (res int) {
	acc, steps := 0, 0
	_ = steps
if acc > 2 {
	if acc%3 == 0 {
		fmt.Println("e46")
		fmt.Println("e47")
	} else {
		if acc%2 == 0 {
			fmt.Println("e49")
		}
		fmt.Println("e50")
	}
	{
		var fs []func() int
		for i52 := 0; i52 < 3; i52++ {
			fmt.Println("body", 52, i52)
			fs = append(fs, func() int { return i52 })
			for range it1(3) {
				fmt.Println("body", 55)
				for n58 := range 2 {
					fmt.Println("body", 58, n58)
					if i52 > 0 {
						continue
					} else {
						if acc == 1 {
							res = acc + 7
							return
						}
					}
				}
			}
			{
				var fs []func() int
L62:
				for x63 := range it1(3) {
					fmt.Println("body", 63, x63)
					fs = append(fs, func() int { return x63 })
					if x63 > 0 {
						fmt.Println("e65", i52, x63)
					}
					if x63%3 == 1 {
						if acc%2 == 1 {
							continue L62
						}
					} else {
						acc++
					}
					for x69 := range it1(3) {
						fmt.Println("body", 69, x69)
						fmt.Println("e71", i52, x63, x69)
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
	for acc = range it1(3) {
		fmt.Println("body", 73)
		acc++
	}
} else {
	fmt.Println("e76")
	fmt.Println("e77")
}
fmt.Println("e78")
for x80 := range it1(3) {
	fmt.Println("body", 80, x80)
	fmt.Println("e82", x80)
	fmt.Println("e83", x80)
	acc++
}
if acc == 3 {
	res = acc + 43
	return
}
	res = acc
	return
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e84")
for x86 := range it1(3) {
	fmt.Println("body", 86, x86)
	if x86 > 2 {
		res = acc + 82
		return
	}
	if acc > 3 {
		fmt.Println("e88", x86)
		fmt.Println("e89", x86)
	}
}
fmt.Println("e90")
{
	var fs []func() int
	for i92 := 0; i92 < 1; i92++ {
		fmt.Println("body", 92, i92)
		fs = append(fs, func() int { return i92 })
		acc++
		break
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
	res = acc
	return
}

func f5() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e94")
fmt.Println("e95")
if acc%2 == 1 {
	fmt.Println("e96")
}
if acc%3 == 1 {
	fmt.Println("e97")
	fmt.Println("e98")
}
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
// it1 done 1
// e4
// f0 = 0
// e5
// body 8 0
// body 11 0
// e13 0 0
// e14 0 0
// e15 0 0
// body 11 1
// e13 0 1
// e14 0 1
// e15 0 1
// f1 = 72
// e30
// e32
// e30
// e32
// e30
// e32
// f2 = 0
// e76
// e77
// e78
// body 80 0
// e82 0
// e83 0
// body 80 1
// e82 1
// e83 1
// body 80 2
// e82 2
// e83 2
// it1 done 3
// f3 = 46
// e84
// body 86 0
// body 86 1
// body 86 2
// it1 done 3
// e90
// body 92 0
// kept 0
// f4 = 1
// e94
// e95
// f5 = 0
