// phase: 09
// encoding/json: what Unmarshal does with values that already exist: struct fields that are not
// mentioned, slices, arrays, pointers, interfaces holding values, and the state after an error.

package main

import (
	"encoding/json"
	"fmt"
)

type Item struct {
	ID   int
	Tags []string
}

type Holder struct {
	Count  int
	Name   string
	Items  []Item
	Fixed  [3]int
	Next   *Holder
	Value  any
	Mapped map[string]int
}

type Shape interface{ Area() int }

type Square struct{ Side int }

func (s Square) Area() int { return s.Side * s.Side }

type Box struct {
	Shape Shape
	Any   any
}

func show(label string, value any, err error) {
	fmt.Printf("%-24s %+v err=%v\n", label, value, err)
}

func main() {
	fmt.Println("== struct fields not mentioned stay")
	holder := Holder{Count: 1, Name: "keep", Fixed: [3]int{7, 8, 9}}
	err := json.Unmarshal([]byte(`{"Count":2}`), &holder)
	show("partial", holder, err)
	err = json.Unmarshal([]byte(`{"Name":null,"Count":null}`), &holder)
	show("null leaves scalars", holder, err)

	fmt.Println("== slices")
	items := []int{1, 2, 3, 4, 5}
	original := items
	err = json.Unmarshal([]byte(`[9,8]`), &items)
	show("shorter", items, err)
	fmt.Println(len(items), cap(items), original)
	err = json.Unmarshal([]byte(`[]`), &items)
	fmt.Println(items == nil, len(items), cap(items), err)
	err = json.Unmarshal([]byte(`null`), &items)
	fmt.Println(items == nil, err)
	items = make([]int, 2, 10)
	err = json.Unmarshal([]byte(`[1,2,3,4]`), &items)
	fmt.Println(items, len(items), cap(items) >= 4, err)
	structs := []Item{{ID: 1, Tags: []string{"a", "b"}}, {ID: 2}}
	err = json.Unmarshal([]byte(`[{"ID":5},{"Tags":["z"]}]`), &structs)
	show("slice of struct", structs, err)
	structs = []Item{{ID: 1, Tags: []string{"a", "b"}}}
	err = json.Unmarshal([]byte(`[{"Tags":["z"]},{"ID":2}]`), &structs)
	show("slice grows", structs, err)
	reused := make([]Item, 1, 4)
	reused[0].ID = 7
	backing := reused[:cap(reused)]
	backing[1] = Item{ID: 99, Tags: []string{"old"}}
	err = json.Unmarshal([]byte(`[{},{}]`), &reused)
	show("reused capacity", reused, err)

	fmt.Println("== arrays")
	array := [3]int{1, 2, 3}
	err = json.Unmarshal([]byte(`[9]`), &array)
	show("short fills zero", array, err)
	err = json.Unmarshal([]byte(`[1,2,3,4,5]`), &array)
	show("long drops", array, err)
	err = json.Unmarshal([]byte(`null`), &array)
	show("null", array, err)

	fmt.Println("== pointers")
	shared := &Holder{Count: 1}
	pointer := shared
	err = json.Unmarshal([]byte(`{"Count":5,"Next":{"Count":6}}`), &pointer)
	fmt.Println(pointer == shared, shared.Count, shared.Next.Count, err)
	err = json.Unmarshal([]byte(`{"Next":{"Name":"x"}}`), &pointer)
	fmt.Println(shared.Next.Count, shared.Next.Name, err)
	err = json.Unmarshal([]byte(`{"Next":null}`), &pointer)
	fmt.Println(shared.Next == nil, err)
	number := new(int)
	*number = 4
	numberPointer := number
	err = json.Unmarshal([]byte(`5`), &numberPointer)
	fmt.Println(*number, numberPointer == number, err)
	err = json.Unmarshal([]byte(`null`), &numberPointer)
	fmt.Println(*number, numberPointer == nil, err)
	var doublePointer **int
	err = json.Unmarshal([]byte(`7`), &doublePointer)
	fmt.Println(**doublePointer, err)

	fmt.Println("== interfaces")
	box := Box{Shape: Square{Side: 2}, Any: &Item{ID: 1}}
	err = json.Unmarshal([]byte(`{"Shape":{"Side":3},"Any":{"ID":9}}`), &box)
	show("non-empty interface", box.Any, err)
	fmt.Println(box.Shape)
	any1 := any(&Item{ID: 1})
	err = json.Unmarshal([]byte(`{"ID":9,"Tags":["t"]}`), &any1)
	fmt.Printf("%+v %T %v\n", any1, any1, err)
	any2 := any(Item{ID: 1})
	err = json.Unmarshal([]byte(`{"ID":9}`), &any2)
	fmt.Printf("%+v %T %v\n", any2, any2, err)
	any3 := any(new(int))
	err = json.Unmarshal([]byte(`null`), &any3)
	fmt.Printf("%v %v\n", any3, err)
	var shape Shape
	err = json.Unmarshal([]byte(`{"Side":3}`), &shape)
	fmt.Println(shape, err)
	err = json.Unmarshal([]byte(`null`), &shape)
	fmt.Println(shape, err)
	shapePointer := Shape(&Square{Side: 1})
	err = json.Unmarshal([]byte(`{"Side":4}`), &shapePointer)
	fmt.Println(shapePointer, err)

	fmt.Println("== after an error")
	holder = Holder{Count: 1, Name: "orig"}
	err = json.Unmarshal([]byte(`{"Count":"x","Name":"new","Fixed":[1,"y",3]}`), &holder)
	show("type error continues", holder, err)
	holder = Holder{Count: 1}
	err = json.Unmarshal([]byte(`{"Count":2,"Name":"new"`), &holder)
	show("syntax error leaves untouched", holder, err)
	holder = Holder{Count: 1}
	err = json.Unmarshal([]byte(`{"Count":2,"Name":"new"} x`), &holder)
	show("trailing garbage leaves untouched", holder, err)
	var anyValue any = 1
	err = json.Unmarshal([]byte(`[1,2`), &anyValue)
	show("any after syntax error", anyValue, err)

	fmt.Println("== map in struct")
	holder = Holder{Mapped: map[string]int{"a": 1}}
	err = json.Unmarshal([]byte(`{"Mapped":{"b":2}}`), &holder)
	show("map merged", holder.Mapped, err)
	err = json.Unmarshal([]byte(`{"Mapped":null}`), &holder)
	show("map nulled", holder.Mapped == nil, err)
}

// Output:
// == struct fields not mentioned stay
// partial                  {Count:2 Name:keep Items:[] Fixed:[7 8 9] Next:<nil> Value:<nil> Mapped:map[]} err=<nil>
// null leaves scalars      {Count:2 Name:keep Items:[] Fixed:[7 8 9] Next:<nil> Value:<nil> Mapped:map[]} err=<nil>
// == slices
// shorter                  [9 8] err=<nil>
// 2 5 [9 8 3 4 5]
// false 0 0 <nil>
// true <nil>
// [1 2 3 4] 4 true <nil>
// slice of struct          [{ID:5 Tags:[a b]} {ID:2 Tags:[z]}] err=<nil>
// slice grows              [{ID:1 Tags:[z]} {ID:2 Tags:[]}] err=<nil>
// reused capacity          [{ID:7 Tags:[]} {ID:99 Tags:[old]}] err=<nil>
// == arrays
// short fills zero         [9 0 0] err=<nil>
// long drops               [1 2 3] err=<nil>
// null                     [1 2 3] err=<nil>
// == pointers
// true 5 6 <nil>
// 6 x <nil>
// true <nil>
// 5 true <nil>
// 5 true <nil>
// 7 <nil>
// == interfaces
// non-empty interface      &{ID:9 Tags:[]} err=json: cannot unmarshal object into Go struct field Box.Shape of type main.Shape
// {2}
// &{ID:9 Tags:[t]} *main.Item <nil>
// map[ID:9] map[string]interface {} <nil>
// <nil> <nil>
// <nil> json: cannot unmarshal object into Go value of type main.Shape
// <nil> <nil>
// &{4} <nil>
// == after an error
// type error continues     {Count:1 Name:new Items:[] Fixed:[1 0 3] Next:<nil> Value:<nil> Mapped:map[]} err=json: cannot unmarshal string into Go struct field Holder.Count of type int
// syntax error leaves untouched {Count:1 Name: Items:[] Fixed:[0 0 0] Next:<nil> Value:<nil> Mapped:map[]} err=unexpected end of JSON input
// trailing garbage leaves untouched {Count:1 Name: Items:[] Fixed:[0 0 0] Next:<nil> Value:<nil> Mapped:map[]} err=invalid character 'x' after top-level value
// any after syntax error   1 err=unexpected end of JSON input
// == map in struct
// map merged               map[a:1 b:2] err=<nil>
// map nulled               true err=<nil>
