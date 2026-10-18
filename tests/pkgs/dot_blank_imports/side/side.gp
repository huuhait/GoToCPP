package side

import "fmt"

var Registered = register()

func register() bool { fmt.Println("side effect"); return true }

func init() { fmt.Println("init side") }
