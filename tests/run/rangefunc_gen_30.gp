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
if acc > 1 {
	fmt.Println("e1")
}
if acc == 0 {
	goto end
}
fmt.Println("e2")
end:
	return acc
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
for range it0(3) {
	fmt.Println("body", 4)
	for x7 := range it1(3) {
		fmt.Println("body", 7, x7)
		fmt.Println("e9", x7)
		for x11 := range it1(2) {
			fmt.Println("body", 11, x11)
			defer fmt.Println("defer", 13, acc)
			for x15 := range it1(1) {
				fmt.Println("body", 15, x15)
				acc += x11 % 5
			}
			for acc = range it1(2) {
				fmt.Println("body", 18)
				if x11%2 == 0 {
					break
				}
			}
		}
	}
L20:
	for x21 := range it1(3) {
		fmt.Println("body", 21, x21)
		fmt.Println("e23", x21)
		for x25 := range it1(2) {
			fmt.Println("body", 25, x25)
			defer fmt.Println("defer", 27, acc)
			switch x25 % 3 {
case 0:
				break
				acc += x21 % 5
case 1:
				if x21%3 == 2 {
					fmt.Println("e29", x21, x25)
					fmt.Println("e30", x21, x25)
				}
				fmt.Println("e31", x21, x25)
default:
				if x21 > 2 {
					continue L20
				}
			}
		}
	}
	acc++
}
fmt.Println("e32")
{
	var fs []func() int
L33:
	for x34 := range it1(2) {
		fmt.Println("body", 34, x34)
		fs = append(fs, func() int { return x34 })
		break L33
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
	res = acc
	return
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
if acc > 2 {
	fmt.Println("e36")
	fmt.Println("e37")
}
for n39 := range 2 {
	fmt.Println("body", 39, n39)
	switch n39 % 3 {
case 0:
		if n39 == 2 {
			continue
		}
case 1:
		fmt.Println("e42", n39)
		acc++
default:
		fmt.Println("e43", n39)
	}
	return acc + 98
}
	return acc
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e44")
for range it0(3) {
	fmt.Println("body", 46)
	goto end
	defer fmt.Println("defer", 48, acc)
}
for x50, s50 := range it2(1) {
	fmt.Println("body", 50, x50)
	fmt.Println("body", 50, s50)
	fmt.Println("e52", x50)
}
end:
	return acc
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
L53:
switch acc % 3 {
case 0:
	if acc%3 == 0 {
		break
	}
	if acc%3 == 2 {
		return acc + 30
	}
case 1:
	if acc == 0 {
		for x56 := range it1(3) {
			fmt.Println("body", 56, x56)
			acc += x56 % 5
			for x59 := range it1(2) {
				fmt.Println("body", 59, x59)
				if x56 > 0 {
					fmt.Println("e61", x56, x59)
					break
				}
			}
			acc += x56 % 5
		}
		fmt.Println("e62")
	} else {
		for x65 := range it1(3) {
			fmt.Println("body", 65, x65)
			fmt.Println("e67", x65)
			acc++
		}
		return acc + 67
	}
default:
	break L53
}
if acc%3 == 2 {
	if acc > 2 {
		fmt.Println("e69")
	}
	if acc == 3 {
		return acc + 62
	}
} else {
	for x72 := range it1(1) {
		fmt.Println("body", 72, x72)
		continue
		for acc = range it1(1) {
			fmt.Println("body", 75)
			if x72 == 0 {
				fmt.Println("e77", x72)
				goto top
			}
			acc++
		}
	}
	goto top
	for range it0(2) {
		fmt.Println("body", 79)
		fmt.Println("e81")
		acc++
	}
}
{
	var fs []func() int
	for x83 := range it1(1) {
		fmt.Println("body", 83, x83)
		fs = append(fs, func() int { return x83 })
		if x83%3 == 1 {
			if acc%2 == 1 {
				defer fmt.Println("defer", 87, acc)
			} else {
L89:
				for x90, s90 := range it2(1) {
					fmt.Println("body", 90, x90)
					fmt.Println("body", 90, s90)
					if x90%3 == 1 {
						fmt.Println("e92", x90)
						continue L89
					}
				}
			}
		} else {
			fmt.Println("e94")
			return acc + 94
		}
		continue
		fmt.Println("e95", x83)
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
end:
	return acc
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
goto top
if acc > 2 {
L97:
	switch acc % 3 {
case 0:
		if acc%2 == 0 {
			fmt.Println("e98")
			break L97
		}
		fmt.Println("e99")
case 1:
		fmt.Println("e100")
default:
		fmt.Println("e101")
		if acc > 2 {
			fmt.Println("e103")
		} else {
			for x106 := range it1(1) {
				fmt.Println("body", 106, x106)
				if x106%2 == 1 {
					fmt.Println("e108", x106)
					fmt.Println("e109", x106)
				}
			}
		}
	}
	switch acc % 3 {
case 0:
		for x112 := range it1(2) {
			fmt.Println("body", 112, x112)
			return acc + 93
			switch x112 % 3 {
case 0:
				fmt.Println("e115", x112)
case 1:
				if x112 > 2 {
					continue
				}
default:
				if x112%2 == 1 {
					goto end
				}
				acc += x112 % 5
			}
		}
case 1:
		for x117 := range it1(1) {
			fmt.Println("body", 117, x117)
L119:
			switch x117 % 3 {
case 0:
				if x117 > 0 {
					break L119
				}
				fmt.Println("e120", x117)
case 1:
				fmt.Println("e121", x117)
				fmt.Println("e122", x117)
			}
			for range it0(2) {
				fmt.Println("body", 124)
				if acc > 0 {
					if acc%2 == 0 {
						fmt.Println("e127")
						return acc + 10
					}
				} else {
					acc++
				}
			}
			fmt.Println("e129", x117)
		}
		defer fmt.Println("defer", 130, acc)
default:
		for x132 := range it1(1) {
			fmt.Println("body", 132, x132)
L134:
			switch x132 % 3 {
case 0:
				fmt.Println("e135", x132)
				defer fmt.Println("defer", 136, acc)
case 1:
				fmt.Println("e137", x132)
default:
				if x132 == 3 {
					break L134
				}
			}
		}
	}
	defer fmt.Println("defer", 138, acc)
} else {
	fmt.Println("e140")
	for acc = range it1(2) {
		fmt.Println("body", 142)
		for i145 := 0; i145 < 2; i145++ {
			fmt.Println("body", 145, i145)
			acc += i145 % 5
			if i145 == 3 {
				if acc%2 == 0 {
					fmt.Println("e148")
				}
			} else {
				fmt.Println("e150")
			}
			goto end
		}
		return acc + 30
		if acc%2 == 0 {
			for range it0(2) {
				fmt.Println("body", 153)
				acc++
			}
		} else {
			fmt.Println("e156")
		}
	}
}
if acc%3 == 0 {
	fmt.Println("e158")
	fmt.Println("e159")
} else {
	for range it0(1) {
		fmt.Println("body", 162)
		for x165 := range it1(3) {
			fmt.Println("body", 165, x165)
			fmt.Println("e167", x165)
			if acc == 0 {
				goto end
			}
			acc++
		}
	}
	if acc%2 == 0 {
		goto end
	}
	{
		var fs []func() int
		for n169 := range 2 {
			fmt.Println("body", 169, n169)
			{
				var fs []func() int
				for x172 := range it1(3) {
					fmt.Println("body", 172, x172)
					fs = append(fs, func() int { return x172 })
					for acc = range it1(1) {
						fmt.Println("body", 175)
						fmt.Println("e177", n169, x172)
					}
					fmt.Println("e178", n169, x172)
					fmt.Println("e179", n169, x172)
				}
				for _, f := range fs {
					fmt.Println("kept", f())
				}
			}
L180:
			switch n169 % 3 {
case 0:
				break L180
case 1:
				for x182 := range it1(1) {
					fmt.Println("body", 182, x182)
					if n169%2 == 1 {
						return acc + 86
					}
				}
default:
				fmt.Println("e184", n169)
				if n169 == 1 {
					break L180
				}
			}
		}
		for _, f := range fs {
			fmt.Println("kept", f())
		}
	}
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
// f0 = 0
// body 4
// body 7 0
// e9 0
// body 11 0
// body 15 0
// it1 done 1
// body 18
// it1 stopped at 0
// it1 done 2
// body 11 1
// body 15 0
// it1 done 1
// body 18
// body 18
// it1 done 2
// it1 done 2
// body 7 1
// e9 1
// body 11 0
// body 15 0
// it1 done 1
// body 18
// it1 stopped at 0
// it1 done 2
// body 11 1
// body 15 0
// it1 done 1
// body 18
// body 18
// it1 done 2
// it1 done 2
// body 7 2
// e9 2
// body 11 0
// body 15 0
// it1 done 1
// body 18
// it1 stopped at 0
// it1 done 2
// body 11 1
// body 15 0
// it1 done 1
// body 18
// body 18
// it1 done 2
// it1 done 2
// it1 done 3
// body 21 0
// e23 0
// body 25 0
// body 25 1
// e31 0 1
// it1 done 2
// body 21 1
// e23 1
// body 25 0
// body 25 1
// e31 1 1
// it1 done 2
// body 21 2
// e23 2
// body 25 0
// body 25 1
// e29 2 1
// e30 2 1
// e31 2 1
// it1 done 2
// it1 done 3
// body 4
// body 7 0
// e9 0
// body 11 0
// body 15 0
// it1 done 1
// body 18
// it1 stopped at 0
// it1 done 2
// body 11 1
// body 15 0
// it1 done 1
// body 18
// body 18
// it1 done 2
// it1 done 2
// body 7 1
// e9 1
// body 11 0
// body 15 0
// it1 done 1
// body 18
// it1 stopped at 0
// it1 done 2
// body 11 1
// body 15 0
// it1 done 1
// body 18
// body 18
// it1 done 2
// it1 done 2
// body 7 2
// e9 2
// body 11 0
// body 15 0
// it1 done 1
// body 18
// it1 stopped at 0
// it1 done 2
// body 11 1
// body 15 0
// it1 done 1
// body 18
// body 18
// it1 done 2
// it1 done 2
// it1 done 3
// body 21 0
// e23 0
// body 25 0
// body 25 1
// e31 0 1
// it1 done 2
// body 21 1
// e23 1
// body 25 0
// body 25 1
// e31 1 1
// it1 done 2
// body 21 2
// e23 2
// body 25 0
// body 25 1
// e29 2 1
// e30 2 1
// e31 2 1
// it1 done 2
// it1 done 3
// body 4
// body 7 0
// e9 0
// body 11 0
// body 15 0
// it1 done 1
// body 18
// it1 stopped at 0
// it1 done 2
// body 11 1
// body 15 0
// it1 done 1
// body 18
// body 18
// it1 done 2
// it1 done 2
// body 7 1
// e9 1
// body 11 0
// body 15 0
// it1 done 1
// body 18
// it1 stopped at 0
// it1 done 2
// body 11 1
// body 15 0
// it1 done 1
// body 18
// body 18
// it1 done 2
// it1 done 2
// body 7 2
// e9 2
// body 11 0
// body 15 0
// it1 done 1
// body 18
// it1 stopped at 0
// it1 done 2
// body 11 1
// body 15 0
// it1 done 1
// body 18
// body 18
// it1 done 2
// it1 done 2
// it1 done 3
// body 21 0
// e23 0
// body 25 0
// body 25 1
// e31 0 1
// it1 done 2
// body 21 1
// e23 1
// body 25 0
// body 25 1
// e31 1 1
// it1 done 2
// body 21 2
// e23 2
// body 25 0
// body 25 1
// e29 2 1
// e30 2 1
// e31 2 1
// it1 done 2
// it1 done 3
// it0 done 3
// e32
// body 34 0
// it1 stopped at 0
// it1 done 2
// kept 0
// defer 27 1
// defer 27 1
// defer 27 1
// defer 27 1
// defer 27 1
// defer 27 1
// defer 13 0
// defer 13 1
// defer 13 0
// defer 13 1
// defer 13 0
// defer 13 2
// defer 27 1
// defer 27 1
// defer 27 1
// defer 27 1
// defer 27 1
// defer 27 1
// defer 13 0
// defer 13 1
// defer 13 0
// defer 13 1
// defer 13 0
// defer 13 2
// defer 27 1
// defer 27 1
// defer 27 1
// defer 27 1
// defer 27 1
// defer 27 1
// defer 13 0
// defer 13 1
// defer 13 0
// defer 13 1
// defer 13 0
// defer 13 0
// deferred acc 2
// f1 = 2
// body 39 0
// deferred acc 0
// f2 = 98
// e44
// body 46
// it0 stopped at 0
// it0 done 3
// f3 = 0
// body 72 0
// it1 done 1
// body 72 0
// it1 done 1
// body 72 0
// it1 done 1
// f4 = 0
// f5 = 0
