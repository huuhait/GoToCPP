// phase: 05

// A pointer to a struct, array, slice or map prints as &{...} only at the top level; one level
// down it prints its address; pointers inside slices and maps print addresses.
package main

import "fmt"

// norm replaces every hexadecimal address (0x...) with 0xP: addresses differ from run to run.
func norm(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '0' && i+1 < len(s) && s[i+1] == 'x' {
			j := i + 2
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] >= 'a' && s[j] <= 'f') {
				j++
			}
			if j > i+2 {
				out = append(out, "0xP"...)
				i = j - 1
				continue
			}
		}
		out = append(out, s[i])
	}
	return string(out)
}
type N struct {
	V int
	P *N
}

func main() {
	n := &N{1, &N{2, nil}}
	fmt.Println(norm(fmt.Sprint(n)))
	fmt.Println(norm(fmt.Sprint(*n)))
	fmt.Println(norm(fmt.Sprint([]*N{n})))
	fmt.Println(norm(fmt.Sprint(map[string]*N{"a": n})))
	fmt.Println(norm(fmt.Sprint(struct{ P *N }{n})))
	fmt.Println(norm(fmt.Sprint([1]*[]int{&[]int{1}})))
	pp := &n
	fmt.Println(norm(fmt.Sprint(pp)))
	var a any = n
	fmt.Println(norm(fmt.Sprint(a)))
	fmt.Println(norm(fmt.Sprint([]any{n, *n})))
	fmt.Println(norm(fmt.Sprintf("%+v %#v", n, n)))
	arr := &[2]int{1, 2}
	sl := &[]string{"a"}
	mp := &map[int]int{1: 2}
	fmt.Println(arr, sl, mp, &arr != nil)
	fmt.Println(norm(fmt.Sprint(&arr, &sl)))
	fmt.Printf("%v %+v %#v %d\n", arr, arr, arr, arr)
	fmt.Printf("%s %x\n", &[]string{"s"}, &[]int{255})
}

// Output:
// &{1 0xP}
// {1 0xP}
// [0xP]
// map[a:0xP]
// {0xP}
// [0xP]
// 0xP
// &{1 0xP}
// [0xP {1 0xP}]
// &{V:1 P:0xP} &main.N{V:1, P:(*main.N)(0xP)}
// &[1 2] &[a] &map[1:2] true
// 0xP 0xP
// &[1 2] &[1 2] &[2]int{1, 2} &[1 2]
// &[s] &[ff]
