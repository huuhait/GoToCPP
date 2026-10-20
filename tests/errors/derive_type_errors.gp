// phase: 09

// The rules for #Derive that need the types (step 10.10): what the generators can and cannot do
// with the type of each field, and what the type must be.

package main

import "bytes"

#Derive(String) // ERROR "#Derive\(String\): Name is not an integer type with constants"
type Name string

#Derive(String) // ERROR "#Derive\(String\): no constants of type Empty are declared in this package"
type Empty int

#Derive(Equal) // ERROR "#Derive\(Equal\): field Hook has a func type, which cannot be compared"
type WithFunc struct {
	Hook func()
}

#Derive(Equal) // ERROR "#Derive\(Equal\): field Hooks\[\] has a func type, which cannot be compared"
type WithFuncs struct {
	Hooks []func()
}

type Inner struct{ Vals []int }

#Derive(Equal) // ERROR "#Derive\(Equal\): field In has the type Inner, which has no Equal method and holds pointers, slices or maps: add #Derive\(Equal\) to Inner or write the method by hand"
type NeedsEqual struct {
	In Inner
}

#Derive(Equal) // ERROR "#Derive\(Equal\): field Buf has the type bytes.Buffer, which has no Equal method and cannot be compared with =="
type Foreign struct {
	Buf bytes.Buffer
}

#Derive(Clone) // ERROR "#Derive\(Clone\): field In has the type Inner, which refers to memory and has no Clone method: add #Derive\(Clone\) to Inner or write the method by hand"
type NeedsClone struct {
	In Inner
}

#Derive(Clone) // ERROR "#Derive\(Clone\): field List\[\] has the type Inner, which refers to memory"
type NeedsCloneInSlice struct {
	List []Inner
}

#Derive(Equal) // ERROR "#Derive\(Equal\): the type Ring refers to itself and has no Equal method: add #Derive\(Equal\) to it"
type Start []Ring

type Ring []Back
type Back []Ring

type Odd struct{ V []int }

func (o Odd) Equal(p *Odd) bool { return true }
func (o Odd) Clone() *Odd       { return &o }

#Derive(Equal) // ERROR "#Derive\(Equal\): field O has the type Odd, whose Equal method does not have the signature Equal\(Odd\) bool"
type UsesOddEqual struct {
	O Odd
}

#Derive(Clone) // ERROR "#Derive\(Clone\): field O has the type Odd, whose Clone method does not have the signature Clone\(\) Odd"
type UsesOddClone struct {
	O Odd
}

#Derive(JSON) // ERROR "#Derive\(JSON\): the methods call encoding/json, which the file does not import: add import \x22encoding/json\x22 to the file of NoImport"
type NoImport struct {
	A int `json:"a"`
}

#Derive(JSON) // ERROR "#Derive\(JSON\): Level is not a struct type"
type Level int

func main() {}
