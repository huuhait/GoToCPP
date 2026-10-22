// phase: 12
// opaque: reject

// The address of a pointer variable goes to a void** parameter: the callee's pointer is stored in
// the variable when the call returns. A pointer goes to a void* or const void* parameter as it is.
package main

import (
	"fmt"
	"unsafe"

	"./adapt.hpp"
)

type holder struct {
	obj *adapt.Obj
}

func main() {
	var obj *adapt.Obj
	adapt.make_object(&obj)
	fmt.Println(obj != nil, obj.v)

	// A field is a variable too.
	var h holder
	adapt.make_object(&h.obj)
	fmt.Println(h.obj.v)

	// The variable gets the callee's value, nil included.
	adapt.make_nothing(&obj)
	fmt.Println(obj == nil)

	// A pointer to a C++ type goes to void* and const void* by itself; so does unsafe.Pointer.
	adapt.make_object(&h.obj)
	fmt.Println(adapt.mutable_value(h.obj), adapt.object_value(h.obj), adapt.object_value(unsafe.Pointer(h.obj)))
	fmt.Println(adapt.object_value(nil))
}

// Output:
// true 7
// 7
// true
// 7 7 7
// -1
