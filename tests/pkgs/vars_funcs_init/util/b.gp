package util

import "fmt"

var B = f()

var CC = A * 2

func f() int { fmt.Println("util.f"); return 5 }

func init() { fmt.Println("util.init b") }
