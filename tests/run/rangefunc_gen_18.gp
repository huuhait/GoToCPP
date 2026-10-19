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
for n2 := range 2 {
	fmt.Println("body", 2, n2)
	for range it0(2) {
		fmt.Println("body", 5)
		if n2 == 3 {
			continue L1
		}
	}
	fmt.Println("e7", n2)
	fmt.Println("e8", n2)
}
{
	var fs []func() int
	for x10 := range it1(1) {
		fmt.Println("body", 10, x10)
L12:
		for n13 := range 2 {
			fmt.Println("body", 13, n13)
			if n13%2 == 0 {
				break L12
			}
		}
		{
			var fs []func() int
			for range it1(3) {
				fmt.Println("body", 16)
				{
					var fs []func() int
					for n19 := range 3 {
						fmt.Println("body", 19, n19)
						fs = append(fs, func() int { return n19 })
						fmt.Println("e21", x10, n19)
						fmt.Println("e22", x10, n19)
					}
					for _, f := range fs {
						fmt.Println("kept", f())
					}
				}
				acc++
				fmt.Println("e23", x10)
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

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
top:
	steps++
	if steps > 3 {
		goto end
	}
{
	var fs []func() int
L24:
	for x25 := range it1(3) {
		fmt.Println("body", 25, x25)
		fs = append(fs, func() int { return x25 })
		if x25 == 0 {
			goto top
		}
		fmt.Println("e27", x25)
		for acc = range it1(1) {
			fmt.Println("body", 29)
			for i32 := 0; i32 < 3; i32++ {
				fmt.Println("body", 32, i32)
				fmt.Println("e34", x25, i32)
				fmt.Println("e35", x25, i32)
			}
			continue L24
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
fmt.Println("e36")
if acc%2 == 1 {
	return acc + 91
}
end:
	res = acc
	return
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
fmt.Println("e37")
{
	var fs []func() int
	for x39 := range it1(3) {
		fmt.Println("body", 39, x39)
		fs = append(fs, func() int { return x39 })
		goto end
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
if acc == 3 {
	if acc == 1 {
		if acc == 3 {
			fmt.Println("e43")
			return acc + 27
		}
		{
			var fs []func() int
L44:
			for n45 := range 1 {
				fmt.Println("body", 45, n45)
				fs = append(fs, func() int { return n45 })
				for n48 := range 3 {
					fmt.Println("body", 48, n48)
					acc += n45 % 5
				}
				if n45%3 == 1 {
					if acc == 1 {
						continue L44
					}
				} else {
					fmt.Println("e52")
				}
			}
			for _, f := range fs {
				fmt.Println("kept", f())
			}
		}
		for x54, s54 := range it2(2) {
			fmt.Println("body", 54, x54)
			fmt.Println("body", 54, s54)
			fmt.Println("e56", x54)
		}
	} else {
		if acc > 0 {
			fmt.Println("e59")
		} else {
			return acc + 47
		}
	}
	for x62 := range it1(1) {
		fmt.Println("body", 62, x62)
		fmt.Println("e64", x62)
	}
L65:
	for x66 := range it1(1) {
		fmt.Println("body", 66, x66)
		if x66%3 == 2 {
			continue L65
		}
	}
} else {
	switch acc % 3 {
case 0:
		{
			var fs []func() int
			for x71 := range it1(3) {
				fmt.Println("body", 71, x71)
				fs = append(fs, func() int { return x71 })
				fmt.Println("e73", x71)
				for range it1(2) {
					fmt.Println("body", 75)
					fmt.Println("e77", x71)
				}
				for acc = range it1(3) {
					fmt.Println("body", 79)
					goto b80
b80:
				}
			}
			for _, f := range fs {
				fmt.Println("kept", f())
			}
		}
		switch acc % 3 {
case 0:
			goto top
case 1:
			fmt.Println("e82")
			fmt.Println("e83")
default:
			{
				var fs []func() int
				for i85 := 0; i85 < 1; i85++ {
					fmt.Println("body", 85, i85)
					fs = append(fs, func() int { return i85 })
					if i85 == 3 {
						fmt.Println("e87", i85)
						return acc + 53
					}
				}
				for _, f := range fs {
					fmt.Println("kept", f())
				}
			}
		}
case 1:
		switch acc % 3 {
case 0:
			for n90 := range 2 {
				fmt.Println("body", 90, n90)
				if n90 > 2 {
					continue
				}
			}
			fmt.Println("e92")
case 1:
			for x94 := range it1(3) {
				fmt.Println("body", 94, x94)
				goto top
			}
		}
		for acc = range it1(2) {
			fmt.Println("body", 97)
			goto end
			if acc > 0 {
				fmt.Println("e99")
				return acc + 42
			}
			for x101 := range it1(2) {
				fmt.Println("body", 101, x101)
				if x101%2 == 1 {
					fmt.Println("e103", x101)
				}
			}
		}
default:
		fmt.Println("e104")
	}
	fmt.Println("e105")
}
end:
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
if acc%3 == 0 {
L107:
	for x108, s108 := range it2(1) {
		fmt.Println("body", 108, x108)
		fmt.Println("body", 108, s108)
		if x108 > 1 {
			for x112 := range it1(3) {
				fmt.Println("body", 112, x112)
				if x112 > 3 {
					if acc > 0 {
						fmt.Println("e115")
					}
				} else {
					fmt.Println("e117")
				}
			}
		} else {
			if acc > 3 {
				fmt.Println("e119")
				goto end
			}
		}
		switch x108 % 3 {
case 0:
			break L107
			if x108%2 == 1 {
				if acc > 1 {
					continue L107
				}
			} else {
				fmt.Println("e123")
			}
case 1:
			if x108%3 == 2 {
				return acc + 83
			}
			continue
default:
			defer fmt.Println("defer", 124, acc)
		}
	}
	if acc%2 == 0 {
		goto top
	}
} else {
	fmt.Println("e126")
	for n128 := range 2 {
		fmt.Println("body", 128, n128)
		fmt.Println("e130", n128)
		fmt.Println("e131", n128)
		if n128 == 1 {
			return acc + 89
		}
	}
}
fmt.Println("e132")
fmt.Println("e133")
for x135 := range it1(3) {
	fmt.Println("body", 135, x135)
	for range it1(3) {
		fmt.Println("body", 138)
		continue
		acc += x135 % 5
	}
}
end:
	return acc
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
if acc%3 == 0 {
	fmt.Println("e140")
	fmt.Println("e141")
}
goto end
if acc == 1 {
	if acc%2 == 0 {
		goto end
	}
	for x144, s144 := range it2(2) {
		fmt.Println("body", 144, x144)
		fmt.Println("body", 144, s144)
		fmt.Println("e146", x144)
	}
	fmt.Println("e147")
} else {
	defer fmt.Println("defer", 149, acc)
	for range it1(3) {
		fmt.Println("body", 151)
		for x154 := range it1(3) {
			fmt.Println("body", 154, x154)
			fmt.Println("e156", x154)
			for range it0(1) {
				fmt.Println("body", 158)
				fmt.Println("e160", x154)
			}
		}
		fmt.Println("e161")
	}
	fmt.Println("e162")
}
fmt.Println("e163")
end:
	res = acc
	return
}

func f5() (res int) {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
switch acc % 3 {
case 0:
	if acc > 3 {
		fmt.Println("e165")
	}
	break
case 1:
	acc++
default:
	for x167 := range it1(3) {
		fmt.Println("body", 167, x167)
		acc++
	}
}
fmt.Println("e169")
if acc > 1 {
L171:
	for x172 := range it1(1) {
		fmt.Println("body", 172, x172)
		switch x172 % 3 {
case 0:
			if acc > 0 {
				fmt.Println("e176")
			} else {
				if acc%2 == 1 {
					goto end
				}
			}
case 1:
			for acc = range it1(2) {
				fmt.Println("body", 179)
				if x172%2 == 0 {
					continue L171
				}
			}
			fmt.Println("e181", x172)
default:
			if x172%3 == 0 {
				fmt.Println("e182", x172)
			}
		}
		for x184 := range it1(3) {
			fmt.Println("body", 184, x184)
			fmt.Println("e186", x172, x184)
			for n188 := range 1 {
				fmt.Println("body", 188, n188)
				break L171
			}
			if x184 > 3 {
				break
			}
		}
	}
	fmt.Println("e190")
	if acc == 3 {
		for x193, s193 := range it2(1) {
			fmt.Println("body", 193, x193)
			fmt.Println("body", 193, s193)
			for range it0(3) {
				fmt.Println("body", 196)
				if x193%2 == 1 {
					continue
				}
			}
			if acc%3 == 2 {
				if acc%2 == 0 {
					goto top
				}
			} else {
				acc++
			}
			if acc == 1 {
				fmt.Println("e201")
			} else {
				if acc > 1 {
					return acc + 12
				}
			}
		}
	} else {
		fmt.Println("e204")
L205:
		for range it1(2) {
			fmt.Println("body", 206)
			if acc > 1 {
				break L205
			}
		}
		for x209 := range it1(1) {
			fmt.Println("body", 209, x209)
			fmt.Println("e211", x209)
		}
	}
} else {
	switch acc % 3 {
case 0:
		fmt.Println("e214")
case 1:
		fmt.Println("e215")
		if acc == 3 {
			for acc = range it1(1) {
				fmt.Println("body", 218)
				if acc%3 == 0 {
					fmt.Println("e220")
					goto top
				}
			}
		} else {
			for x223 := range it1(1) {
				fmt.Println("body", 223, x223)
				fmt.Println("e225", x223)
			}
		}
default:
		for acc = range it1(1) {
			fmt.Println("body", 227)
			if acc%3 == 0 {
				break
			}
			for x230 := range it1(3) {
				fmt.Println("body", 230, x230)
				if x230 > 0 {
					fmt.Println("e233")
				} else {
					fmt.Println("e235")
				}
			}
		}
	}
}
L236:
for range it1(1) {
	fmt.Println("body", 237)
	fmt.Println("e239")
	fmt.Println("e240")
	continue L236
}
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
// body 2 0
// body 5
// body 5
// it0 done 2
// e7 0
// e8 0
// body 2 1
// body 5
// body 5
// it0 done 2
// e7 1
// e8 1
// body 10 0
// body 13 0
// body 16
// body 19 0
// e21 0 0
// e22 0 0
// body 19 1
// e21 0 1
// e22 0 1
// body 19 2
// e21 0 2
// e22 0 2
// kept 0
// kept 1
// kept 2
// e23 0
// body 16
// body 19 0
// e21 0 0
// e22 0 0
// body 19 1
// e21 0 1
// e22 0 1
// body 19 2
// e21 0 2
// e22 0 2
// kept 0
// kept 1
// kept 2
// e23 0
// body 16
// body 19 0
// e21 0 0
// e22 0 0
// body 19 1
// e21 0 1
// e22 0 1
// body 19 2
// e21 0 2
// e22 0 2
// kept 0
// kept 1
// kept 2
// e23 0
// it1 done 3
// it1 done 1
// f0 = 3
// body 25 0
// it1 stopped at 0
// it1 done 3
// body 25 0
// it1 stopped at 0
// it1 done 3
// body 25 0
// it1 stopped at 0
// it1 done 3
// deferred acc 0
// f1 = 0
// e37
// body 39 0
// it1 stopped at 0
// it1 done 3
// f2 = 0
// body 108 0
// body 108 s0
// it2 stopped at 0
// it2 done 1
// body 108 0
// body 108 s0
// it2 stopped at 0
// it2 done 1
// body 108 0
// body 108 s0
// it2 stopped at 0
// it2 done 1
// f3 = 0
// e140
// e141
// f4 = 0
// e169
// e214
// body 237
// e239
// e240
// it1 done 1
// f5 = 0
