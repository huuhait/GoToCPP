// Package extest has its examples in an external test package.
package extest

// Add adds two numbers.
func Add(a, b int) int { return a + b }

// T is a type.
type T struct{}

// M is a method.
func (T) M() {}
