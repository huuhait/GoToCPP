// Package macros has the syntax that only gocpp has: the error shape
// func f!() T, macros, and #Derive.
package macros

import "errors"

// Parse reads a number. It fails with ErrSyntax when s is not one.
func Parse!(s string) int {
	if s == "" {
		return 0
	}
	return len(s)
}

// ErrSyntax is the error of Parse.
var ErrSyntax = errors.New("syntax")

// Assert stops the program when cond is false.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(msg)
	}
}

// Trace logs the start of a function and its end.
macro Trace() func() {
	return func() {}
}

// Locked runs the body of a function under a lock.
macro Locked(mu *int) {
	#body()
}

// Color is a color.
//
// #Derive(String) makes the names of the values.
#Derive(String)
type Color int

// The colors.
const (
	Red Color = iota
	Green
	Blue
)

// Point is a point of the plane.
#Derive(String, Equal, Clone)
type Point struct {
	X, Y int
}

// Norm is the length of p.
func (p Point) Norm() int { return p.X + p.Y }

// Config is read from JSON.
#Derive(JSON)
type Config struct {
	Port int
}

// Run does the work and traces it.
#Trace()
func Run() {}

// Load reads a config and fails with an error.
func (c *Config) Load!(path string) {}
