// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Base struct{ Alpha, Beta int }
type T struct {
	Base
	Gamma string
	delta int
}

func main() {
	t := reflect.TypeOf(T{})
	has := func(prefix string) func(string) bool {
		return func(s string) bool { return len(s) >= len(prefix) && s[:len(prefix)] == prefix }
	}
	for _, p := range []string{"Al", "Be", "Ga", "de", "Ba", "zz", ""} {
		f, ok := t.FieldByNameFunc(has(p))
		fmt.Printf("%q %v %q %v\n", p, ok, f.Name, f.Index)
	}
	v := reflect.ValueOf(T{Base{1, 2}, "g", 4})
	fmt.Println(v.FieldByNameFunc(has("Be")), v.FieldByNameFunc(has("Ga")), v.FieldByNameFunc(has("nothing")).IsValid())
}

// Output:
// "Al" true "Alpha" [0 0]
// "Be" true "Beta" [0 1]
// "Ga" true "Gamma" [1]
// "de" true "delta" [2]
// "Ba" true "Base" [0]
// "zz" false "" []
// "" false "" []
// 2 g false
