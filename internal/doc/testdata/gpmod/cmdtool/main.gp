// Command cmdtool does a thing.
package main

import "fmt"

// Helper is exported but in a command.
func Helper() {}

func main() { fmt.Println("hi") }
