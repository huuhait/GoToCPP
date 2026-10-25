// Package dep is the dependency of the fake example package.
package dep

// Value is a plain function.
func Value() int { return 1 }

// Pulled is a target of a pull linkname.
func Pulled() {}

// Pushed is a target of a push linkname.
func Pushed() {}

// Var is a target of a variable linkname.
var Var int
