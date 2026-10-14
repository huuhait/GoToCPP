// A declaration that uses the name it declares gets a new C++ name, because in
// C++ the new name is already in scope in its own initializer.
package main

import "fmt"

var w = 3

func f() int {
	return 40
}

func main() {
	v := 1
	v_1 := 100 // the new name of the inner v must not be v_1
	{
		v := v + 1
		fmt.Println(v, v_1)
		{
			v := v * 2
			fmt.Println(v)
		}
	}
	x := 5
	{
		x := x
		var w = w + x
		f := f()
		fmt.Println(x, w, f)
	}
	{
		x, y := 2, x // y gets the outer x
		fmt.Println(x, y)
	}
	fmt.Println(v, x, w, f())
}
