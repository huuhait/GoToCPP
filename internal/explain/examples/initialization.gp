package main

import "fmt"

var limit = 100
var table = [3]int{1, 2, 3}
var total = compute()

func compute() int { return limit * table[2] }

func init() { fmt.Println("init", total) }

func main() { fmt.Println(limit, total) }
