// phase: 05

// Structs that hold arrays, slices of structs, maps of structs, interfaces and funcs; nil funcs
// print <nil>.
package main

import "fmt"

type Item struct {
	Name string
	Qty  int
}

type Order struct {
	ID    int
	Items []Item
	ByKey map[string]Item
	Grid  [2][2]Item
	Cb    func()
	Any   any
	Err   error
}

func main() {
	o := Order{ID: 1, Items: []Item{{"a", 1}, {"b", 2}}, ByKey: map[string]Item{"x": {"c", 3}}, Any: Item{"d", 4}}
	fmt.Printf("%v\n%+v\n", o, o)
	fmt.Printf("%#v\n", o)
	fmt.Println(Order{}, Order{}.Items == nil)
	fmt.Printf("%v %+v\n", []Order{{ID: 2}}, []Order{{ID: 3}})
	fmt.Printf("%T %T %T\n", o, o.Items, o.Grid)
}

// Output:
// {1 [{a 1} {b 2}] map[x:{c 3}] [[{ 0} { 0}] [{ 0} { 0}]] <nil> {d 4} <nil>}
// {ID:1 Items:[{Name:a Qty:1} {Name:b Qty:2}] ByKey:map[x:{Name:c Qty:3}] Grid:[[{Name: Qty:0} {Name: Qty:0}] [{Name: Qty:0} {Name: Qty:0}]] Cb:<nil> Any:{Name:d Qty:4} Err:<nil>}
// main.Order{ID:1, Items:[]main.Item{main.Item{Name:"a", Qty:1}, main.Item{Name:"b", Qty:2}}, ByKey:map[string]main.Item{"x":main.Item{Name:"c", Qty:3}}, Grid:[2][2]main.Item{[2]main.Item{main.Item{Name:"", Qty:0}, main.Item{Name:"", Qty:0}}, [2]main.Item{main.Item{Name:"", Qty:0}, main.Item{Name:"", Qty:0}}}, Cb:(func())(nil), Any:main.Item{Name:"d", Qty:4}, Err:error(nil)}
// {0 [] map[] [[{ 0} { 0}] [{ 0} { 0}]] <nil> <nil> <nil>} true
// [{2 [] map[] [[{ 0} { 0}] [{ 0} { 0}]] <nil> <nil> <nil>}] [{ID:3 Items:[] ByKey:map[] Grid:[[{Name: Qty:0} {Name: Qty:0}] [{Name: Qty:0} {Name: Qty:0}]] Cb:<nil> Any:<nil> Err:<nil>}]
// main.Order []main.Item [2][2]main.Item
