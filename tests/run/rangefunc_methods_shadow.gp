// phase: 08

package main

import "fmt"

type Bag struct {
	items []int
	log   []string
}

func (b *Bag) Each(yield func(int) bool) {
	for _, x := range b.items {
		if !yield(x) {
			return
		}
	}
}

func (b Bag) Total() (sum int) {
	for x := range b.Each {
		sum += x
	}
	return
}

func (b *Bag) First(pred func(int) bool) (int, bool) {
	for x := range b.Each {
		b.log = append(b.log, fmt.Sprint("saw ", x))
		if pred(x) {
			return x, true
		}
	}
	return 0, false
}

func (b Bag) Pairs() (res [][2]int) {
	for x := range b.Each {
		for y := range b.Each {
			if y <= x {
				continue
			}
			res = append(res, [2]int{x, y})
			if len(res) == 3 {
				return
			}
		}
	}
	return res
}

func shadow() {
	x := "outer"
	r := 0
	for x := range (&Bag{items: []int{1, 2}}).Each {
		r += x
		f := func() string { return fmt.Sprint(x, r) }
		fmt.Println(f())
		{
			x := "inner"
			for r := range (&Bag{items: []int{7}}).Each {
				fmt.Println(x, r)
			}
		}
	}
	fmt.Println(x, r)
}

func main() {
	b := &Bag{items: []int{3, 1, 4, 1, 5}}
	fmt.Println(b.Total())
	fmt.Println(b.First(func(x int) bool { return x > 3 }))
	fmt.Println(b.log)
	fmt.Println(b.Pairs())
	shadow()
	var done []func()
	for x := range b.Each {
		go func() {}()
		done = append(done, func() { fmt.Print(x, " ") })
	}
	for _, d := range done {
		d()
	}
	fmt.Println()
}

// Output:
// 14
// 4 true
// [saw 3 saw 1 saw 4]
// [[3 4] [3 5] [1 3]]
// 1 1
// inner 7
// 2 3
// inner 7
// outer 3
// 3 1 4 1 5 
