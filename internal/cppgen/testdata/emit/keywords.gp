// Go names that are C++ keywords or compiler words get a _ suffix; a name
// that ends in _ already gets one more, so the two never meet.
package main

import "fmt"

func new(class int) int {
	return class + 1
}

func delete(this, template string) string {
	return this + template
}

func operator() (and, or bool) {
	return true, false
}

var namespace = "ns"
var _Pragma = 3
var __int128 = 4
var __FILE__ = "file"

func main() {
	int := 5
	bool := int > 4
	char, auto := "c", 1.5
	typename, new_ := new(int), new(1)
	new__ := new_ + 1
	xor, not := operator()
	__is_same, _Complex, char8_t := 1, 2, 3
	fmt.Println(int, bool, char, auto, typename, new_, new__)
	fmt.Println(delete("a", "b"), xor, not, namespace, _Pragma, __int128, __FILE__)
	fmt.Println(__is_same, _Complex, char8_t)
}
