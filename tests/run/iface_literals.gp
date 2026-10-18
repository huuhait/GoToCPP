// phase: 05

// Interface values as elements of composite literals, map keys and values, struct fields,
// array elements and appended values (step 5.4).
package main

import "fmt"

type Item interface{ Weight() int }

type Box struct{ w int }
type Bag struct{ ws []int }

func (b Box) Weight() int  { return b.w }
func (b *Bag) Weight() int { t := 0; for _, w := range b.ws { t += w }; return t }

type Cart struct {
	Main  Item
	Extra []Item
	Tags  map[string]any
}

func main() {
	items := []Item{Box{1}, &Bag{[]int{2, 3}}, Box{4}}
	arr := [3]Item{Box{1}, nil, &Bag{}}
	byName := map[string]Item{"a": Box{10}, "b": &Bag{[]int{1}}}
	byKey := map[Item]string{Box{1}: "one", Box{2}: "two"}
	c := Cart{Main: Box{7}, Extra: items, Tags: map[string]any{"n": 1, "s": "x", "b": Box{1}}}
	t := 0
	for _, it := range items {
		t += it.Weight()
	}
	fmt.Println(t, arr[1] == nil, arr[2].Weight(), byName["a"].Weight(), byName["b"].Weight())
	fmt.Println(byKey[Box{1}], byKey[Box{2}], len(byKey), byKey[Box{3}] == "")
	fmt.Println(c.Main.Weight(), len(c.Extra), c.Tags["n"].(int), c.Tags["s"].(string), c.Tags["b"].(Box).w)
	items = append(items, Box{5}, nil)
	fmt.Println(len(items), items[4] == nil)
	nested := [][]any{{1, "a"}, {nil, 2.5, Box{1}}}
	fmt.Println(len(nested[1]), nested[1][0] == nil, nested[1][2].(Item).Weight())
	pairs := []struct {
		k string
		v any
	}{{"a", 1}, {"b", nil}, {"c", Box{3}}}
	for _, p := range pairs {
		fmt.Println(p.k, p.v == nil)
	}
	m := map[any]int{1: 1, "a": 2, Box{1}: 3, 2.5: 4}
	fmt.Println(m[1], m["a"], m[Box{1}], m[2.5], m[int64(1)], len(m))
}

// Output:
// 10 true 0 10 1
// one two 2 true
// 7 3 1 x 1
// 5 true
// 3 true 1
// a false
// b true
// c false
// 1 2 3 4 0 4
