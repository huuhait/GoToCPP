package first

import "fmt"

var Name = start()

func start() string {
	fmt.Println("first: var")
	return "first"
}

func init() {
	fmt.Println("first: init")
}
