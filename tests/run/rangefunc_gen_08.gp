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
for i2 := 0; i2 < 3; i2++ {
	fmt.Println("body", 2, i2)
	fmt.Println("e4", i2)
}
for n6 := range 1 {
	fmt.Println("body", 6, n6)
	for acc = range it1(2) {
		fmt.Println("body", 9)
L11:
		for acc = range it1(3) {
			fmt.Println("body", 12)
			fmt.Println("e14", n6)
			switch n6 % 3 {
case 0:
				if n6 == 0 {
					fmt.Println("e16", n6)
					continue L11
				}
case 1:
				if n6 > 2 {
					res = acc + 15
					return
				}
			}
		}
	}
}
fmt.Println("e17")
{
	var fs []func() int
L18:
	for x19 := range it1(2) {
		fmt.Println("body", 19, x19)
		fs = append(fs, func() int { return x19 })
		if x19 > 0 {
			res = acc + 94
			return
		}
		if x19 > 1 {
			for x23 := range it1(2) {
				fmt.Println("body", 23, x23)
				acc++
				goto end
				if acc == 0 {
					break
				}
			}
			switch acc % 3 {
case 0:
				for acc = range it1(2) {
					fmt.Println("body", 27)
					if acc%2 == 0 {
						continue
					}
				}
case 1:
				if acc > 2 {
					fmt.Println("e29")
					continue
				}
				for range it0(1) {
					fmt.Println("body", 31)
					fmt.Println("e33")
				}
			}
			acc++
		} else {
			fmt.Println("e35")
			if acc == 0 {
				fmt.Println("e36")
				break L18
			}
			{
				var fs []func() int
				for x38 := range it1(1) {
					fmt.Println("body", 38, x38)
					fs = append(fs, func() int { return x38 })
					if x38 > 2 {
						fmt.Println("e40", x38)
						goto end
					}
					fmt.Println("e41", x38)
				}
				for _, f := range fs {
					fmt.Println("kept", f())
				}
			}
		}
		fmt.Println("e42", x19)
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
end:
	res = acc
	return
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
for x44 := range it1(2) {
	fmt.Println("body", 44, x44)
	for x47 := range it1(2) {
		fmt.Println("body", 47, x47)
		acc++
		fmt.Println("e49", x44, x47)
		if x44%2 == 1 {
			fmt.Println("e51")
		} else {
			for range it0(1) {
				fmt.Println("body", 54)
				if acc%2 == 0 {
					fmt.Println("e56")
					return acc + 49
				}
			}
		}
	}
	fmt.Println("e57", x44)
}
L58:
switch acc % 3 {
case 0:
	fmt.Println("e59")
	for x61 := range it1(1) {
		fmt.Println("body", 61, x61)
		fmt.Println("e63", x61)
	}
case 1:
L64:
	for acc = range it1(2) {
		fmt.Println("body", 65)
		if acc > 1 {
			continue
		}
		continue L64
		if acc == 3 {
			if acc == 1 {
				fmt.Println("e68")
				break
			}
		} else {
			fmt.Println("e70")
		}
	}
default:
	if acc%3 == 0 {
		fmt.Println("e72")
	} else {
		for n75 := range 1 {
			fmt.Println("body", 75, n75)
			for acc = range it1(3) {
				fmt.Println("body", 78)
				if n75%2 == 1 {
					goto b76
				}
			}
			for acc = range it1(2) {
				fmt.Println("body", 81)
				if acc%2 == 0 {
					break L58
				}
			}
			if acc%3 == 1 {
				fmt.Println("e83", n75)
			}
b76:
		}
		break L58
		if acc > 1 {
			fmt.Println("e85")
		} else {
			fmt.Println("e87")
		}
	}
}
	return acc
}

func f2() (res int) {
	acc, steps := 0, 0
	_ = steps
if acc%2 == 0 {
	acc++
	{
		var fs []func() int
L89:
		for x90 := range it1(2) {
			fmt.Println("body", 90, x90)
			break L89
			{
				var fs []func() int
				for i93 := 0; i93 < 1; i93++ {
					fmt.Println("body", 93, i93)
					fs = append(fs, func() int { return i93 })
					fmt.Println("e95", x90, i93)
					fmt.Println("e96", x90, i93)
					for x98, s98 := range it2(1) {
						fmt.Println("body", 98, x98)
						fmt.Println("body", 98, s98)
						defer fmt.Println("defer", 100, acc)
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
} else {
	if acc%3 == 1 {
		fmt.Println("e102")
		fmt.Println("e103")
	}
	{
		var fs []func() int
		for n105 := range 2 {
			fmt.Println("body", 105, n105)
			fs = append(fs, func() int { return n105 })
			{
				var fs []func() int
				for n108 := range 3 {
					fmt.Println("body", 108, n108)
					fs = append(fs, func() int { return n108 })
					if n108 == 2 {
						if acc == 3 {
							goto b109
						}
					} else {
						if acc%3 == 1 {
							fmt.Println("e112")
							res = acc + 86
							return
						}
					}
b109:
				}
				for _, f := range fs {
					fmt.Println("kept", f())
				}
			}
			for n114 := range 2 {
				fmt.Println("body", 114, n114)
				fmt.Println("e116", n105, n114)
			}
			if n105 == 1 {
				goto b101
			}
		}
		for _, f := range fs {
			fmt.Println("kept", f())
		}
	}
L117:
	for x118 := range it1(2) {
		fmt.Println("body", 118, x118)
		fmt.Println("e120", x118)
		fmt.Println("e121", x118)
		for x123 := range it1(1) {
			fmt.Println("body", 123, x123)
			for acc = range it1(3) {
				fmt.Println("body", 126)
				if x123 > 3 {
					fmt.Println("e128", x118, x123)
					goto end
				}
			}
			acc++
			switch x123 % 3 {
case 0:
				if x118 == 0 {
					fmt.Println("e130", x118, x123)
				}
case 1:
				acc++
default:
				if acc > 2 {
					fmt.Println("e131", x118, x123)
					break L117
				}
			}
		}
	}
b101:
}
for i133 := 0; i133 < 1; i133++ {
	fmt.Println("body", 133, i133)
	if i133 == 2 {
		fmt.Println("e135", i133)
		break
	}
	for x137, s137 := range it2(2) {
		fmt.Println("body", 137, x137)
		fmt.Println("body", 137, s137)
		fmt.Println("e139", i133, x137)
		fmt.Println("e140", i133, x137)
		if acc == 0 {
			fmt.Println("e141", i133, x137)
			res = acc + 52
			return
		}
	}
}
end:
	res = acc
	return
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
for acc = range it1(2) {
	fmt.Println("body", 143)
	for x146 := range it1(3) {
		fmt.Println("body", 146, x146)
		for i149 := 0; i149 < 2; i149++ {
			fmt.Println("body", 149, i149)
L151:
			for x152 := range it1(3) {
				fmt.Println("body", 152, x152)
				if x152 == 1 {
					continue L151
				}
			}
			fmt.Println("e154", x146, i149)
			for x156 := range it1(2) {
				fmt.Println("body", 156, x156)
				acc++
			}
		}
		fmt.Println("e158", x146)
	}
	for x160 := range it1(2) {
		fmt.Println("body", 160, x160)
		acc += x160 % 5
		fmt.Println("e162", x160)
		acc++
	}
	if acc%3 == 1 {
		return acc + 36
	}
}
if acc%2 == 1 {
	return acc + 86
}
{
	var fs []func() int
	for x164, s164 := range it2(3) {
		fmt.Println("body", 164, x164)
		fmt.Println("body", 164, s164)
		fs = append(fs, func() int { return x164 })
		if acc%3 == 0 {
			return acc + 59
			if acc > 2 {
				fmt.Println("e167")
				fmt.Println("e168")
			}
		} else {
			for i171 := 0; i171 < 2; i171++ {
				fmt.Println("body", 171, i171)
				fmt.Println("e173", i171)
			}
		}
		acc += x164 % 5
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
if acc == 0 {
	fmt.Println("e174")
}
	return acc
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e175")
acc++
if acc > 2 {
	return acc + 27
}
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
for range it1(2) {
	fmt.Println("body", 177)
	goto top
	continue
}
{
	var fs []func() int
	for x180 := range it1(2) {
		fmt.Println("body", 180, x180)
		fs = append(fs, func() int { return x180 })
		fmt.Println("e182", x180)
	}
	for _, f := range fs {
		fmt.Println("kept", f())
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
// e4 0
// body 2 1
// e4 1
// body 2 2
// e4 2
// body 6 0
// body 9
// body 12
// e14 0
// e16 0
// body 12
// e14 0
// e16 0
// body 12
// e14 0
// e16 0
// it1 done 3
// body 9
// body 12
// e14 0
// e16 0
// body 12
// e14 0
// e16 0
// body 12
// e14 0
// e16 0
// it1 done 3
// it1 done 2
// e17
// body 19 0
// e35
// body 38 0
// e41 0
// it1 done 1
// kept 0
// e42 0
// body 19 1
// it1 stopped at 1
// it1 done 2
// deferred acc 2
// f0 = 96
// body 44 0
// body 47 0
// e49 0 0
// body 54
// it0 done 1
// body 47 1
// e49 0 1
// body 54
// e56
// it0 stopped at 0
// it0 done 1
// it1 stopped at 1
// it1 done 2
// it1 stopped at 0
// it1 done 2
// deferred acc 2
// f1 = 51
// body 90 0
// it1 stopped at 0
// it1 done 2
// body 133 0
// body 137 0
// body 137 s0
// e139 0 0
// e140 0 0
// body 137 10
// body 137 s1
// e139 0 10
// e140 0 10
// it2 done 2
// f2 = 1
// body 143
// body 146 0
// body 149 0
// body 152 0
// body 152 1
// body 152 2
// it1 done 3
// e154 0 0
// body 156 0
// body 156 1
// it1 done 2
// body 149 1
// body 152 0
// body 152 1
// body 152 2
// it1 done 3
// e154 0 1
// body 156 0
// body 156 1
// it1 done 2
// e158 0
// body 146 1
// body 149 0
// body 152 0
// body 152 1
// body 152 2
// it1 done 3
// e154 1 0
// body 156 0
// body 156 1
// it1 done 2
// body 149 1
// body 152 0
// body 152 1
// body 152 2
// it1 done 3
// e154 1 1
// body 156 0
// body 156 1
// it1 done 2
// e158 1
// body 146 2
// body 149 0
// body 152 0
// body 152 1
// body 152 2
// it1 done 3
// e154 2 0
// body 156 0
// body 156 1
// it1 done 2
// body 149 1
// body 152 0
// body 152 1
// body 152 2
// it1 done 3
// e154 2 1
// body 156 0
// body 156 1
// it1 done 2
// e158 2
// it1 done 3
// body 160 0
// e162 0
// body 160 1
// e162 1
// it1 done 2
// body 143
// body 146 0
// body 149 0
// body 152 0
// body 152 1
// body 152 2
// it1 done 3
// e154 0 0
// body 156 0
// body 156 1
// it1 done 2
// body 149 1
// body 152 0
// body 152 1
// body 152 2
// it1 done 3
// e154 0 1
// body 156 0
// body 156 1
// it1 done 2
// e158 0
// body 146 1
// body 149 0
// body 152 0
// body 152 1
// body 152 2
// it1 done 3
// e154 1 0
// body 156 0
// body 156 1
// it1 done 2
// body 149 1
// body 152 0
// body 152 1
// body 152 2
// it1 done 3
// e154 1 1
// body 156 0
// body 156 1
// it1 done 2
// e158 1
// body 146 2
// body 149 0
// body 152 0
// body 152 1
// body 152 2
// it1 done 3
// e154 2 0
// body 156 0
// body 156 1
// it1 done 2
// body 149 1
// body 152 0
// body 152 1
// body 152 2
// it1 done 3
// e154 2 1
// body 156 0
// body 156 1
// it1 done 2
// e158 2
// it1 done 3
// body 160 0
// e162 0
// body 160 1
// e162 1
// it1 done 2
// it1 stopped at 1
// it1 done 2
// deferred acc 16
// f3 = 52
// e175
// f4 = 1
// body 177
// it1 stopped at 0
// it1 done 2
// body 177
// it1 stopped at 0
// it1 done 2
// body 177
// it1 stopped at 0
// it1 done 2
// f5 = 0
