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
	defer func() { fmt.Println("deferred acc", acc) }()
for x2 := range it1(1) {
	fmt.Println("body", 2, x2)
	if x2%2 == 0 {
		fmt.Println("e4", x2)
		break
	}
	fmt.Println("e5", x2)
}
for i7 := 0; i7 < 3; i7++ {
	fmt.Println("body", 7, i7)
	for i10 := 0; i10 < 3; i10++ {
		fmt.Println("body", 10, i10)
		if i7 == 3 {
			fmt.Println("e12", i7, i10)
			res = acc + 39
			return
		}
	}
}
{
	var fs []func() int
	for n14 := range 2 {
		fmt.Println("body", 14, n14)
		fs = append(fs, func() int { return n14 })
		acc += n14 % 5
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
	res = acc
	return
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
acc++
fmt.Println("e16")
	res = acc
	return
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e17")
{
	var fs []func() int
L18:
	for range it1(1) {
		fmt.Println("body", 19)
		if acc%3 == 0 {
			{
				var fs []func() int
				for n23 := range 1 {
					fmt.Println("body", 23, n23)
					fs = append(fs, func() int { return n23 })
					for x26 := range it1(1) {
						fmt.Println("body", 26, x26)
						if x26 > 1 {
							fmt.Println("e28", n23, x26)
							continue L18
						}
					}
					defer fmt.Println("defer", 29, acc)
				}
				for _, f := range fs {
					fmt.Println("kept", f())
				}
			}
			if acc%3 == 2 {
				fmt.Println("e30")
				continue L18
			}
			fmt.Println("e31")
		} else {
			for x34 := range it1(3) {
				fmt.Println("body", 34, x34)
				switch x34 % 3 {
case 0:
					if acc%3 == 1 {
						break
					}
case 1:
					acc++
					fmt.Println("e37", x34)
				}
			}
L38:
			for acc = range it1(2) {
				fmt.Println("body", 39)
				switch acc % 3 {
case 0:
					fmt.Println("e42")
					if acc%2 == 1 {
						fmt.Println("e43")
						return acc + 77
					}
case 1:
					if acc%2 == 0 {
						fmt.Println("e44")
					}
					if acc%2 == 1 {
						continue
					}
default:
					if acc%2 == 0 {
						fmt.Println("e45")
						continue L38
					}
					if acc%3 == 1 {
						fmt.Println("e46")
					}
				}
				for x48 := range it1(3) {
					fmt.Println("body", 48, x48)
					if x48%3 == 2 {
						fmt.Println("e50", x48)
						break L38
					}
				}
			}
		}
		for i52 := 0; i52 < 1; i52++ {
			fmt.Println("body", 52, i52)
			for n55 := range 3 {
				fmt.Println("body", 55, n55)
				fmt.Println("e57", i52, n55)
			}
			break
		}
		goto b20
b20:
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
fmt.Println("e58")
	return acc
}

func f3() (res int) {
	acc, steps := 0, 0
	_ = steps
for x60 := range it1(1) {
	fmt.Println("body", 60, x60)
	fmt.Println("e62", x60)
	goto end
}
if acc%3 == 0 {
	defer fmt.Println("defer", 64, acc)
	if acc == 2 {
		fmt.Println("e65")
	}
	{
		var fs []func() int
		for x67 := range it1(1) {
			fmt.Println("body", 67, x67)
			fs = append(fs, func() int { return x67 })
			for range it0(3) {
				fmt.Println("body", 70)
				defer fmt.Println("defer", 72, acc)
			}
			if x67%2 == 1 {
L74:
				switch acc % 3 {
case 0:
					if acc%2 == 0 {
						break L74
					}
					fmt.Println("e75")
case 1:
					if acc > 1 {
						fmt.Println("e76")
					}
default:
					return acc + 84
					if acc%3 == 0 {
						return acc + 43
					}
				}
			} else {
				fmt.Println("e78")
			}
			goto b68
b68:
		}
		for _, f := range fs {
			fmt.Println("kept", f())
		}
	}
} else {
	{
		var fs []func() int
		for x81 := range it1(2) {
			fmt.Println("body", 81, x81)
			for x84 := range it1(1) {
				fmt.Println("body", 84, x84)
				fmt.Println("e86", x81, x84)
				if x81 > 2 {
					goto end
				}
			}
			defer fmt.Println("defer", 87, acc)
			{
				var fs []func() int
				for x89, s89 := range it2(1) {
					fmt.Println("body", 89, x89)
					fmt.Println("body", 89, s89)
					fs = append(fs, func() int { return x89 })
L91:
					for x92 := range it1(3) {
						fmt.Println("body", 92, x92)
						if acc%3 == 1 {
							continue L91
						}
					}
					fmt.Println("e94", x81, x89)
					fmt.Println("e95", x81, x89)
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
}
for n97 := range 1 {
	fmt.Println("body", 97, n97)
	acc += n97 % 5
}
L99:
for x100, s100 := range it2(3) {
	fmt.Println("body", 100, x100)
	fmt.Println("body", 100, s100)
	for n103 := range 3 {
		fmt.Println("body", 103, n103)
		continue L99
		fmt.Println("e105", x100, n103)
	}
}
end:
	res = acc
	return
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e106")
fmt.Println("e107")
	return acc
}

func f5() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
res = acc + 85
return
{
	var fs []func() int
L108:
	for x109 := range it1(2) {
		fmt.Println("body", 109, x109)
		fs = append(fs, func() int { return x109 })
		if x109 > 1 {
			break L108
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
fmt.Println("e111")
if acc%3 == 0 {
	fmt.Println("e112")
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
// body 2 0
// e4 0
// it1 stopped at 0
// it1 done 1
// body 7 0
// body 10 0
// body 10 1
// body 10 2
// body 7 1
// body 10 0
// body 10 1
// body 10 2
// body 7 2
// body 10 0
// body 10 1
// body 10 2
// body 14 0
// body 14 1
// kept 0
// kept 1
// deferred acc 1
// f0 = 1
// e16
// f1 = 1
// e17
// body 19
// body 23 0
// body 26 0
// it1 done 1
// kept 0
// e31
// body 52 0
// body 55 0
// e57 0 0
// body 55 1
// e57 0 1
// body 55 2
// e57 0 2
// it1 done 1
// e58
// defer 29 0
// deferred acc 0
// f2 = 0
// body 60 0
// e62 0
// it1 stopped at 0
// it1 done 1
// f3 = 0
// e106
// e107
// f4 = 0
// deferred acc 0
// f5 = 85
