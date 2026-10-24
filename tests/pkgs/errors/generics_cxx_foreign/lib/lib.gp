// Package lib has generic functions whose bodies use C++ names, and one whose body does not.
package lib

import "<string>"

func Ret[T any](v T) func(std.string) T { return func(std.string) T { return v } }

func Size[T any](v T) int { return int(std.string("abc").size()) }

func Plain[T any](v T) T { return v }
