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
for x2 := range it1(3) {
	fmt.Println("body", 2, x2)
	if acc%3 == 2 {
		return acc + 78
	}
}
goto end
fmt.Println("e4")
fmt.Println("e5")
end:
	return acc
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e6")
fmt.Println("e7")
	return acc
}

func f2() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
if acc%2 == 0 {
	fmt.Println("e8")
}
for acc = range it1(2) {
	fmt.Println("body", 10)
	fmt.Println("e12")
	fmt.Println("e13")
	fmt.Println("e14")
}
	res = acc
	return
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e15")
return acc + 61
return acc + 95
fmt.Println("e16")
	return acc
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
if acc == 2 {
	fmt.Println("e18")
	return acc + 72
} else {
	for x21 := range it1(3) {
		fmt.Println("body", 21, x21)
		fmt.Println("e23", x21)
		return acc + 47
	}
	if acc > 0 {
		fmt.Println("e25")
		fmt.Println("e26")
		for range it0(3) {
			fmt.Println("body", 28)
			for range it1(1) {
				fmt.Println("body", 31)
				fmt.Println("e33")
			}
			fmt.Println("e34")
		}
	} else {
		acc++
	}
	fmt.Println("e36")
}
fmt.Println("e37")
{
	var fs []func() int
	for x39 := range it1(3) {
		fmt.Println("body", 39, x39)
		fs = append(fs, func() int { return x39 })
		fmt.Println("e41", x39)
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
fmt.Println("e42")
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
if acc%2 == 1 {
	fmt.Println("e44")
L45:
	switch acc % 3 {
case 0:
		if acc == 2 {
			break L45
		}
		fmt.Println("e46")
case 1:
		if acc == 3 {
			break L45
		}
default:
		if acc == 3 {
			fmt.Println("e47")
			goto top
		}
	}
} else {
	switch acc % 3 {
case 0:
		for i51 := 0; i51 < 2; i51++ {
			fmt.Println("body", 51, i51)
			for x54 := range it1(1) {
				fmt.Println("body", 54, x54)
				if x54 > 0 {
					fmt.Println("e56", i51, x54)
					goto end
				}
			}
			fmt.Println("e57", i51)
		}
		for acc = range it1(3) {
			fmt.Println("body", 59)
			if acc%2 == 0 {
				fmt.Println("e61")
			}
		}
case 1:
		for x63 := range it1(3) {
			fmt.Println("body", 63, x63)
			fmt.Println("e65", x63)
			fmt.Println("e66", x63)
		}
		if acc == 2 {
			return acc + 79
		}
default:
		for n68 := range 2 {
			fmt.Println("body", 68, n68)
			fmt.Println("e70", n68)
		}
L71:
		switch acc % 3 {
case 0:
			fmt.Println("e72")
			defer fmt.Println("defer", 73, acc)
case 1:
			for n75 := range 1 {
				fmt.Println("body", 75, n75)
				if n75%3 == 2 {
					fmt.Println("e77", n75)
					break L71
				}
			}
			for x79 := range it1(1) {
				fmt.Println("body", 79, x79)
				if x79%2 == 1 {
					break L71
				}
			}
		}
	}
	for acc = range it1(2) {
		fmt.Println("body", 82)
		return acc + 81
		if acc > 3 {
			goto end
		}
		if acc == 2 {
			fmt.Println("e84")
		}
	}
L85:
	switch acc % 3 {
case 0:
		if acc == 3 {
			for range it1(1) {
				fmt.Println("body", 88)
				goto top
			}
		} else {
			goto top
		}
		for x92 := range it1(3) {
			fmt.Println("body", 92, x92)
			acc++
			break L85
L94:
			switch x92 % 3 {
case 0:
				fmt.Println("e95", x92)
				if acc > 3 {
					fmt.Println("e96", x92)
					fmt.Println("e97", x92)
				}
case 1:
				if x92 > 1 {
					if acc%3 == 2 {
						break L94
					}
				} else {
					if acc > 0 {
						fmt.Println("e100")
					}
				}
				if x92%2 == 1 {
					return acc + 62
				}
default:
				if x92 == 1 {
					return acc + 87
				}
			}
		}
case 1:
		switch acc % 3 {
case 0:
			fmt.Println("e102")
			if acc%3 == 2 {
				fmt.Println("e104")
			} else {
				if acc == 2 {
					return acc + 18
				}
			}
case 1:
			fmt.Println("e106")
		}
		if acc%3 == 2 {
			fmt.Println("e107")
		}
default:
		for x109, s109 := range it2(1) {
			fmt.Println("body", 109, x109)
			fmt.Println("body", 109, s109)
			for range it1(2) {
				fmt.Println("body", 112)
				fmt.Println("e114", x109)
			}
			goto top
		}
	}
}
return acc + 5
for acc = range it1(1) {
	fmt.Println("body", 116)
	if acc%2 == 0 {
		goto end
	}
	if acc > 3 {
		switch acc % 3 {
case 0:
			return acc + 66
			fmt.Println("e120")
case 1:
			fmt.Println("e121")
		}
		acc++
		fmt.Println("e122")
	} else {
		fmt.Println("e124")
	}
	switch acc % 3 {
case 0:
		if acc == 3 {
			return acc + 93
		}
case 1:
		fmt.Println("e126")
		continue
default:
		fmt.Println("e127")
		if acc > 0 {
			fmt.Println("e128")
			continue
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
// body 2 0
// body 2 1
// body 2 2
// it1 done 3
// deferred acc 0
// f0 = 0
// e6
// e7
// f1 = 0
// e8
// body 10
// e12
// e13
// e14
// body 10
// e12
// e13
// e14
// it1 done 2
// deferred acc 1
// f2 = 1
// e15
// deferred acc 0
// f3 = 61
// body 21 0
// e23 0
// it1 stopped at 0
// it1 done 3
// deferred acc 0
// f4 = 47
// body 51 0
// body 54 0
// it1 done 1
// e57 0
// body 51 1
// body 54 0
// it1 done 1
// e57 1
// body 59
// e61
// body 59
// body 59
// e61
// it1 done 3
// body 82
// it1 stopped at 0
// it1 done 2
// f5 = 81
