package main

var second = fromA() + 10

func fromB() string {
	return "b"
}

func init() {
	println("init in b.gp", first)
}
