// phase: 03

// Converting a slice to an array copies its first N elements, and to a pointer to an array
// shares them; a nil slice gives a nil pointer only for N == 0, and an empty one a non-nil
// pointer.
package main

import "fmt"

func main() {
	s := []int{1, 2, 3, 4, 5}
	a := [3]int(s)
	a[0] = 10
	fmt.Println(a[0], a[2], s[0])
	p := (*[2]int)(s)
	p[1] = 20
	fmt.Println(s[1], len(p))
	q := (*[4]int)(s[1:])
	q[3] = 50
	fmt.Println(s[4], q[0])
	s[1] = 21
	fmt.Println(p[1], q[0])
	bs := []byte("hey")
	b := [3]byte(bs)
	bs[0] = 'H'
	fmt.Println(string(b[:]), string(bs))
	var nilS []string
	fmt.Println((*[0]string)(nilS) == nil, (*[0]string)([]string{}) == nil, len([0]string(nilS)))
	arr := [2]int{7, 8}
	same := [2]int(arr[:])
	fmt.Println(same == arr)
	s = append(s, 6)
	r := (*[6]int)(s)
	fmt.Println(r[5], len(r))
}

// Output:
// 10 3 1
// 20 2
// 50 20
// 21 21
// hey Hey
// true false 0
// true
// 6 6
