// phase: 10

// A type of another package without a Clone method that shows slices, maps or pointers is an error
// for Clone (a copy would share them), as one that cannot be compared is for Equal (step 10.10). A
// type with private fields only, a pointer to it, and a type with the methods are fine.

package main

import "example.com/df/model"

#Derive(Clone) // ERROR "#Derive\(Clone\): field P has the type model.Plain, which shows slices, maps or pointers and has no Clone method"
type ByValue struct {
	P model.Plain
}

#Derive(Clone) // ERROR "#Derive\(Clone\): field Ps\[\] has the type model.Plain, which shows slices, maps or pointers and has no Clone method"
type InSlice struct {
	Ps []model.Plain
}

#Derive(Clone) // ERROR "#Derive\(Clone\): field L has the type model.List, which shows slices, maps or pointers and has no Clone method"
type Named struct {
	L model.List
}

#Derive(Equal) // ERROR "#Derive\(Equal\): field P has the type model.Plain, which has no Equal method and cannot be compared with =="
type Eq struct {
	P model.Plain
}

#Derive(Clone)
type FineClone struct {
	PP *model.Plain // shared
	O  model.Opaque // a value
	C  model.Cloner // cloned with its Clone
}

#Derive(Equal)
type FineEqual struct {
	PP *model.Plain // compared by identity
	C  model.Cloner // compared with its Equal
	L  model.List
}

func main() {}
