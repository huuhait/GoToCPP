// phase: 07

// A generic function of another package, instantiated by main with inferred and explicit type arguments.
package main

import (
	"fmt"

	"example.com/gbasic/util"
)

type Celsius float64

type Point struct{ X, Y int }

func main() {
	fmt.Println(util.Sum(1, 2, 3), util.Sum(1.5, 2.5), util.Sum[int64](), util.Sum(Celsius(36.6), Celsius(0.4)))
	fmt.Println(util.Map([]int{1, 2, 3}, func(i int) string { return fmt.Sprint(i * i) }))
	fmt.Println(util.Map([]Point{{1, 2}, {3, 4}}, func(p Point) int { return p.X + p.Y }))
	fmt.Println(util.Max(3, 7), util.Max("a", "b"), util.Max[float64](1, 2.5), util.Max(Celsius(1), 2))
	fmt.Println(util.Filter([]string{"go", "c++", "gc"}, func(s string) bool { return s[0] == 'g' }))
	fmt.Println(len(util.Keys(map[Point]bool{{1, 1}: true})), len(util.Keys(map[string]int{"a": 1, "b": 2})))
	f := util.Max[int]
	g := util.Map[string, int]
	fmt.Println(f(1, 2), g([]string{"a", "bb"}, func(s string) int { return len(s) }))
	fmt.Printf("%T %T\n", util.Sum(Celsius(1)), util.Map([]Point(nil), func(p Point) Celsius { return 0 }))
}


// Output:
// 6 4 0 37
// [1 4 9]
// [3 7]
// 7 b 2.5 2
// [go gc]
// 1 2
// 2 [1 2]
// main.Celsius []main.Celsius
