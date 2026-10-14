// phase: 03

// range over an array iterates over a copy taken when the loop starts: writes to the array in
// the body (directly, through a pointer, through a slice of it, or by a call) are not seen,
// while a range through a pointer to the array sees them; a range without a value variable
// counts to the length. A body that drops the last pointer to the array still sees its values.
package main

import "fmt"

var ga = [4]int{1, 2, 3, 4}

func bump() {
	for i := range ga {
		ga[i] += 100
	}
}

func main() {
	for i, v := range ga {
		if i == 1 {
			bump()
		}
		fmt.Print(v, " ")
	}
	fmt.Println(ga[3])

	a := [4]int{1, 2, 3, 4}
	s := a[:]
	p := &a
	for i, v := range a {
		switch i {
		case 0:
			s[1] = 20
		case 1:
			p[2] = 30
		case 2:
			a[3] = 40
		}
		fmt.Print(v, " ")
	}
	fmt.Println(a[1], a[2], a[3])
	for i, v := range p {
		if i == 0 {
			s[3] = 400
		}
		fmt.Print(v, " ")
	}
	fmt.Println()

	b := [3]string{"x", "y", "z"}
	for i, v := range b {
		b[2] = v + "!"
		fmt.Print(i, v, " ")
	}
	fmt.Println(b[2])
	c := [3]int{1, 2, 3}
	for _, v := range c {
		d := c
		d[2] = 9
		fmt.Print(v, d[2], " ")
	}
	fmt.Println(c[2])
	n := 0
	for i := range a {
		a[i] = -1
		n++
	}
	fmt.Println(n, a[0], a[3])
	m := [2][2]int{{1, 2}, {3, 4}}
	for i, row := range m {
		m[1][0] = 30
		m[i][1] = 0
		fmt.Print(row[0], row[1], " ")
	}
	fmt.Println(m[0][1], m[1][0])
	// The body drops the last pointer to the array: the copy keeps the values.
	pp := &[3]int{1, 2, 3}
	tot := 0
	for _, v := range *pp {
		pp = nil
		tot += v
	}
	rows := [][2]int{{4, 5}}
	for _, v := range rows[0] {
		rows = nil
		tot += v
	}
	fmt.Println(tot, pp == nil, rows == nil)
	var vs [3]int
	var i int
	for i, vs[i] = range [3]int{7, 8, 9} {
	}
	fmt.Println(i, vs[0], vs[1], vs[2])
}
