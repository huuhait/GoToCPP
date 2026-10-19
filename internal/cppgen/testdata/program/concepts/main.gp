package main

import (
	"fmt"

	"example.com/concepts/k"
)

func main() {
	n := k.Named{Name: "a"}
	w := &k.Wrapper{Named: k.Named{Name: "w"}}
	fmt.Println(k.Sum([]int{1, 2, 3}), k.Sum([]k.Celsius{1.5, 2}), k.Max("a", "b"), k.Max(k.Meters(3), 2), k.Eq(1, 1))
	fmt.Println(k.Greet(n), k.Greet(w), k.Greet(k.Tagged(4)), k.Len(k.Names{"x", "y"}), k.Conv[int, float64](3))
	fmt.Println(k.Size("ab"), k.Size([]byte("cd")), k.Both2(&n), k.Show(k.Tagged(5)), k.Anon(k.Tagged(6)))
	k.Rename(&n, "z")
	fmt.Println(n.Name, k.Any(1))
}
