package inner

import "fmt"

func Mul(a, b int) int { return a * b }

func init() { fmt.Println("init inner") }
