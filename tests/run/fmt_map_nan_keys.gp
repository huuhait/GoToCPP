// phase: 05

// Several NaN keys in a map: they sort first (cmp.Compare puts NaN before every number) and print
// in an unspecified order among themselves; here only one NaN key.
package main

import "fmt"

func main() {
	zero := 0.0
	nan := zero / zero
	m := map[float64]string{nan: "n", 1: "a", -1: "b"}
	fmt.Println(m)
	fmt.Printf("%v %+v %#v\n", m, m, m)
	fmt.Println(len(m), m[nan] == "")
}

// Output:
// map[NaN:n -1:b 1:a]
// map[NaN:n -1:b 1:a] map[NaN:n -1:b 1:a] map[float64]string{NaN:"n", -1:"b", 1:"a"}
// 3 true
