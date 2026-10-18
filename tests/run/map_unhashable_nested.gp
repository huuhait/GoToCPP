// phase: 05

// The key check of a read or delete in a nil or empty map reaches into every key shape that can
// hold an interface (step 5.6): nested structs, named array types, anonymous structs, arrays of
// structs, interfaces with methods, a dynamic value that itself holds an interface, blank fields
// (not looked at) and zero-length arrays (nothing to look at).
package main

import "fmt"

type Bad struct{ s []int }

func (Bad) M() {}

type Marker interface{ M() }

type Leaf struct{ v any }
type Mid struct {
	n    int
	leaf Leaf
}
type Top struct {
	m Mid
	_ any
}
type NA [2]any
type AS [2]Leaf
type Z [0]any
type SA2 struct {
	k  int
	as [2]any
}
type GridKey [2][2]any
type Blanky struct {
	a int
	_ any
}

func try(name string, f func()) {
	defer func() {
		r := recover()
		if r == nil {
			fmt.Println(name, "no panic")
			return
		}
		fmt.Println(name, "->", r.(error).Error())
	}()
	f()
}

func main() {
	bad := []int{1}
	var nilTop map[Top]int
	emptyTop := map[Top]int{}
	k := Top{m: Mid{1, Leaf{bad}}}
	try("nested struct nil read", func() { _ = nilTop[k] })
	try("nested struct empty delete", func() { delete(emptyTop, k) })
	try("nested struct empty assign", func() { emptyTop[k] = 1 })
	ok := Top{m: Mid{1, Leaf{"fine"}}}
	try("nested ok nil read", func() { _ = nilTop[ok] })
	try("nested ok empty assign", func() { emptyTop[ok] = 1 })
	try("nested ok read", func() { fmt.Println(emptyTop[ok]) })

	var nilNA map[NA]bool
	try("named array nil read", func() { _ = nilNA[NA{1, bad}] })
	try("named array nil ok", func() { _ = nilNA[NA{1, 2}] })
	emptyAS := map[AS]bool{}
	try("array of structs", func() { _, _ = emptyAS[AS{{1}, {map[string]int{}}}] })
	try("array of structs ok", func() { _, _ = emptyAS[AS{{1}, {2}}] })

	var anon map[struct{ a any }]int
	try("anonymous struct", func() { _ = anon[struct{ a any }{func() {}}] })
	try("anonymous struct ok", func() { _ = anon[struct{ a any }{3}] })

	var markers map[Marker]int
	try("iface with methods", func() { _ = markers[Bad{}] })
	try("iface with methods nil", func() { _ = markers[nil] })
	try("iface with methods read", func() { delete(markers, nil) })

	// A dynamic value that holds an interface itself.
	var anyKeys map[any]int
	try("dynamic struct", func() { _ = anyKeys[Leaf{bad}] })
	try("dynamic struct ok", func() { _ = anyKeys[Leaf{7}] })
	try("dynamic array", func() { _ = anyKeys[NA{1, Leaf{bad}}] })
	try("dynamic uncomparable struct", func() { _ = anyKeys[Bad{}] })
	try("dynamic pointer", func() { _ = anyKeys[&Bad{}] })

	// Arrays inside structs, arrays of such structs, arrays of arrays.
	var nilSA map[SA2]int
	emptySA := map[SA2]int{}
	badSA := SA2{1, [2]any{"ok", bad}}
	okSA := SA2{1, [2]any{"ok", 2}}
	try("struct with array nil read", func() { _ = nilSA[badSA] })
	try("struct with array empty read", func() { _, _ = emptySA[badSA] })
	try("struct with array empty delete", func() { delete(emptySA, badSA) })
	try("struct with array empty assign", func() { emptySA[badSA] = 1 })
	try("struct with array ok", func() { _ = nilSA[okSA]; delete(emptySA, okSA) })
	var nilArrSA map[[2]SA2]int
	emptyArrSA := map[[2]SA2]int{}
	try("array of structs with array nil read", func() { _ = nilArrSA[[2]SA2{okSA, badSA}] })
	try("array of structs with array empty delete", func() { delete(emptyArrSA, [2]SA2{badSA, okSA}) })
	try("array of structs with array ok", func() { _ = nilArrSA[[2]SA2{okSA, okSA}] })
	var nilGrid map[GridKey]int
	emptyGrid := map[GridKey]int{}
	try("grid nil read", func() { _ = nilGrid[GridKey{{1, 2}, {3, bad}}] })
	try("grid empty read", func() { _ = emptyGrid[GridKey{{bad, 2}, {3, 4}}] })
	try("grid empty delete", func() { delete(emptyGrid, GridKey{{1, 2}, {3, map[int]int{}}}) })
	try("grid ok", func() { _ = nilGrid[GridKey{{1, 2}, {3, 4}}] })

	// Blank fields and zero-length arrays are not part of the key.
	var blanks map[Blanky]int
	try("blank field", func() { _ = blanks[Blanky{1, bad}] })
	var zs map[Z]int
	try("zero-length array", func() { _ = zs[Z{}] })

	// The same keys in a map with entries panic with the other text.
	full := map[Top]int{ok: 1}
	try("filled read", func() { _ = full[k] })
	try("filled delete", func() { delete(full, k) })
	try("filled assign", func() { full[k] = 2 })
	fullAny := map[any]int{1: 1}
	try("filled any read", func() { _ = fullAny[Leaf{bad}] })
	try("filled any assign", func() { fullAny[NA{bad, 1}] = 2 })
	fmt.Println(len(full), len(fullAny))
}

// Output:
// nested struct nil read -> hash of unhashable type: []int
// nested struct empty delete -> hash of unhashable type: []int
// nested struct empty assign -> runtime error: hash of unhashable type []int
// nested ok nil read no panic
// nested ok empty assign no panic
// 1
// nested ok read no panic
// named array nil read -> hash of unhashable type: []int
// named array nil ok no panic
// array of structs -> hash of unhashable type: map[string]int
// array of structs ok no panic
// anonymous struct -> hash of unhashable type: func()
// anonymous struct ok no panic
// iface with methods -> hash of unhashable type: main.Bad
// iface with methods nil no panic
// iface with methods read no panic
// dynamic struct -> hash of unhashable type: []int
// dynamic struct ok no panic
// dynamic array -> hash of unhashable type: []int
// dynamic uncomparable struct -> hash of unhashable type: main.Bad
// dynamic pointer no panic
// struct with array nil read -> hash of unhashable type: []int
// struct with array empty read -> hash of unhashable type: []int
// struct with array empty delete -> hash of unhashable type: []int
// struct with array empty assign -> runtime error: hash of unhashable type []int
// struct with array ok no panic
// array of structs with array nil read -> hash of unhashable type: []int
// array of structs with array empty delete -> hash of unhashable type: []int
// array of structs with array ok no panic
// grid nil read -> hash of unhashable type: []int
// grid empty read -> hash of unhashable type: []int
// grid empty delete -> hash of unhashable type: map[int]int
// grid ok no panic
// blank field no panic
// zero-length array no panic
// filled read -> runtime error: hash of unhashable type []int
// filled delete -> runtime error: hash of unhashable type []int
// filled assign -> runtime error: hash of unhashable type []int
// filled any read -> runtime error: hash of unhashable type []int
// filled any assign -> runtime error: hash of unhashable type []int
// 1 1
