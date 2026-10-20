package main

import (
	"example.com/outline/first"
	"example.com/outline/lib"
)

func again(w lib.Wrapped[first.Feet]) first.Feet { return w.Last() }
