// phase: 05

// Types that share a C++ type are different Go types: int and int64, uint8 and byte (the same),
// named types, element types, channel-free composite types, pointers to named types
// (step 5.5).
package main

import "fmt"

type MyInt int
type MyInt2 = int
type Pair struct{ a, b int }
type Other struct{ a, b int }

func name(x any) string {
	switch x.(type) {
	case int:
		return "int"
	case int64:
		return "int64"
	case MyInt:
		return "MyInt"
	case uint8:
		return "uint8/byte"
	case uint:
		return "uint"
	case uint64:
		return "uint64"
	case uintptr:
		return "uintptr"
	case []int:
		return "[]int"
	case []MyInt:
		return "[]MyInt"
	case []int64:
		return "[]int64"
	case map[string]int:
		return "map[string]int"
	case map[string]int64:
		return "map[string]int64"
	case func(int):
		return "func(int)"
	case func(int64):
		return "func(int64)"
	case Pair:
		return "Pair"
	case Other:
		return "Other"
	case struct{ a, b int }:
		return "anon struct"
	case *Pair:
		return "*Pair"
	case *MyInt:
		return "*MyInt"
	case [2]int:
		return "[2]int"
	case [3]int:
		return "[3]int"
	}
	return "other"
}

func main() {
	var mi MyInt2 = 4
	m := MyInt(1)
	vals := []any{1, int64(1), MyInt(1), uint8(1), byte(2), uint(1), uint64(1), uintptr(1), mi,
		[]int{}, []MyInt{}, []int64{}, map[string]int{}, map[string]int64{},
		func(int) {}, func(int64) {}, Pair{}, Other{}, struct{ a, b int }{}, &Pair{}, &m, [2]int{}, [3]int{}, &Other{}}
	for _, v := range vals {
		fmt.Print(name(v), " ")
	}
	fmt.Println()
}

// Output:
// int int64 MyInt uint8/byte uint8/byte uint uint64 uintptr int []int []MyInt []int64 map[string]int map[string]int64 func(int) func(int64) Pair Other anon struct *Pair *MyInt [2]int [3]int other
