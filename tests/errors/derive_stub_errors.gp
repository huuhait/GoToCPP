// phase: 05

// The errors of go/types about the stubs of #Derive (step 10.10), in the words of the #Derive line:
// the methods exist before the types are checked, so a type that cannot have them is found by go/types.

package main

#Derive(Equal) // ERROR "#Derive\(Equal\): Handler is an interface or a pointer type"
type Handler Fn

type Fn interface{ Call() }

#Derive(Clone) // ERROR "#Derive\(Clone\): the type Clashes has a field Clone already"
type Clashes struct {
	Clone int
}

func main() {}
