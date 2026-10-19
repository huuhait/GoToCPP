// phase: 07

// Declaration errors of generic code: type parameters as embedded fields or right-hand sides,
// methods with type parameters, bad constraint interfaces, recursive instantiation and
// overlapping terms.
package main

type Alias[T any] T // ERROR "cannot use a type parameter as RHS in type declaration"

type Emb[T any] struct {
	T // ERROR "embedded field type cannot be a \(pointer to a\) type parameter"
}

type EmbPtr[T any] struct {
	*T // ERROR "embedded field type cannot be a \(pointer to a\) type parameter"
}

type EmbIface[T any] interface {
	T // ERROR "term cannot be a type parameter"
}

type Dup interface {
	~int | int // ERROR "overlapping terms int and ~int"
}

type Bad interface {
	~string | Stringer // ERROR "undefined: Stringer"
}

type Tilde interface {
	~MyInt // ERROR "invalid use of ~ \(underlying type of MyInt is int\)"
}

type MyInt int

type Meth interface {
	~int
	String() string
}

type Rec[T any] struct {
	next Rec[T] // ERROR "invalid recursive type: Rec refers to itself"
}

func F[T any, T any]() {} // ERROR "T redeclared in this block"

func G[T Meth](x T) string { return x.String() }

func H() {
	_ = G(MyInt(1)) // ERROR "in call to G, T \(type MyInt\) does not satisfy Meth \(missing method String\)"
	_ = G(1)        // ERROR "int does not satisfy Meth \(missing method String\)"
}

func I[T int | string, U ~[]T](u U) {}

func J[P *Q, Q any](p P) {}

func K() {
	I[int, []string](nil) // ERROR "\[\]string does not satisfy ~\[\]int"
	J[*int, string](nil)  // ERROR "\*int does not satisfy \*string \(\*int missing in \*string\)"
}

var v interface{ ~int } // ERROR "cannot use type interface{~int} outside a type constraint: interface contains type constraints"

func L[T any](x T) {
	var y T = nil // ERROR "cannot use nil as T value in variable declaration"
	_ = y
	switch x.(type) { // ERROR "cannot use type switch on type parameter value x"
	}
	_ = x.(int) // ERROR "invalid operation: cannot use type assertion on type parameter value x"
}

func N[T any]() {
	var x T
	_ = T{}      // ERROR "invalid composite literal type T"
	_ = x + x    // ERROR "operator \+ not defined on x"
	_ = make(T)  // ERROR "invalid argument: cannot make T: no specific type"
	_ = new(T)
	_ = len(x)   // ERROR "invalid argument: x \(variable of type T constrained by any\) for built-in len"
	for range x { // ERROR "cannot range over x \(variable of type T constrained by any\): no specific type"
	}
}
