// phase: 05

// The rules for a #Derive line that the loader checks (step 10.10), before the types are known.

package main

#Derive() // ERROR "#Derive needs the names of the generators"
type NoNames struct{}

#Derive(Print) // ERROR "#Derive: Print is not the name of a generator"
type Unknown struct{}

#Derive(Str+ing) // ERROR "#Derive: Str \+ ing is not the name of a generator"
type Quoted struct{}

#Derive(String, Equal, String) // ERROR "#Derive\(String\) is written twice"
type Twice struct{}

#Derive(Equal) // ERROR "#Derive on a group of types"
type (
	G1 struct{}
	G2 struct{}
)

#Derive(Equal) // ERROR "#Derive on Alias, which is an alias"
type Alias = Twice

#Derive(Clone) // ERROR "#Derive\(Clone\): Iface is an interface type"
type Iface interface{ M() }

#Derive(Equal) // ERROR "#Derive\(Equal\): Fn is a func type"
type Fn func()

#Derive(Equal) // ERROR "#Derive\(Equal\): Ch is a channel type"
type Ch chan int

#Derive(Clone) // ERROR "#Derive\(Clone\): Ptr is a pointer type, which cannot have methods"
type Ptr *int

#Derive(String) // ERROR "#Derive\(String\): Slice is not an integer type with constants"
type Slice []int

#Derive(String) // ERROR "#Derive\(String\): Rec is a struct: fmt prints a struct by itself"
type Rec struct{}

#Derive(JSON) // ERROR "#Derive\(JSON\): Table is not a struct type"
type Table map[string]int

#Derive(Clone, Equal) // ERROR "#Derive\(Equal\): the type Dup has a method Equal already \(declared at derive_decl_errors.gp:\d+\)"
type Dup struct{ A []int }

func (d Dup) Equal(o Dup) bool { return true }

#Derive(Clone) // ERROR "#Derive\(Clone\): the type Ptrs has a method Clone already"
type Ptrs struct{}

func (p *Ptrs) Clone() Ptrs { return *p }

#Derive // ERROR "expected a macro call on top of a declaration"
type Bare struct{}

#Derive(Equal)
#Derive(Equal) // ERROR "#Derive\(Equal\) is written twice"
type TwoLines struct{}

#Derive(Clone) // ERROR "#Derive on the blank type _"
type _ struct{}

#Plain() // ERROR "only #Derive can be used there"
type Other struct{}

macro Plain() {}

func main() {}
